package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"

	"github.com/osbuild/bootc-image-builder/bib/internal/buildconfig"
	podman_container "github.com/osbuild/bootc-image-builder/bib/internal/container"
	"github.com/osbuild/bootc-image-builder/bib/internal/imagetypes"
	"github.com/osbuild/bootc-image-builder/bib/internal/setup"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
	"github.com/osbuild/bootc-image-builder/bib/internal/util"
	"github.com/osbuild/bootc-image-builder/bib/pkg/progress"
)

const (
	// As a baseline heuristic we double the size of
	// the input container to support in-place updates.
	// This is planned to be more configurable in the
	// future.
	containerSizeToDiskSizeMultiplier = 2
)

// all possible locations for the bib's distro definitions
// ./data/defs and ./bib/data/defs are for development
// /usr/share/bootc-image-builder/defs is for the production, containerized version
var distroDefPaths = []string{
	"./data/defs",
	"./bib/data/defs",
	"/usr/share/bootc-image-builder/defs",
}

var (
	osGetuid = os.Getuid
	osGetgid = os.Getgid
)

// canChownInPath checks if the ownership of files can be set in a given path.
func canChownInPath(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a directory", path)
	}

	checkFile, err := os.CreateTemp(path, ".writecheck")
	if err != nil {
		return false, err
	}
	defer func() {
		if err := os.Remove(checkFile.Name()); err != nil {
			// print the error message for info but don't error out
			fmt.Fprintf(os.Stderr, "error deleting %s: %s\n", checkFile.Name(), err.Error())
		}
	}()
	return checkFile.Chown(osGetuid(), osGetgid()) == nil, nil
}

func inContainerOrUnknown() bool {
	// no systemd-detect-virt, err on the side of container
	if _, err := exec.LookPath("systemd-detect-virt"); err != nil {
		return true
	}
	// exit code "0" means the container is detected
	err := exec.Command("systemd-detect-virt", "-c", "-q").Run()
	return err == nil
}

// getContainerSize returns the size of an already pulled container image in bytes
func getContainerSize(imgref string) (uint64, error) {
	output, err := exec.Command("podman", "image", "inspect", imgref, "--format", "{{.Size}}").Output()
	if err != nil {
		return 0, fmt.Errorf("failed inspect image: %w", util.OutputErr(err))
	}
	size, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse image size: %w", err)
	}

	logrus.Debugf("container size: %v", size)
	return size, nil
}

func makeManifest(c *ManifestConfig, solver *dnfjson.Solver, cacheRoot string) (manifest.OSBuildManifest, map[string][]rpmmd.RepoConfig, error) {
	mani, err := Manifest(c)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get manifest: %w", err)
	}

	// depsolve packages
	depsolvedSets := make(map[string]dnfjson.DepsolveResult)
	depsolvedRepos := make(map[string][]rpmmd.RepoConfig)
	for name, pkgSet := range mani.GetPackageSetChains() {
		res, err := solver.Depsolve(pkgSet, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot depsolve: %w", err)
		}
		depsolvedSets[name] = *res
		depsolvedRepos[name] = res.Repos
	}

	// Resolve container - the normal case is that host and target
	// architecture are the same. However it is possible to build
	// cross-arch images by using qemu-user. This will run everything
	// (including the build-root) with the target arch then, it
	// is fast enough (given that it's mostly I/O and all I/O is
	// run naively via syscall translation)

	// XXX: should NewResolver() take "arch.Arch"?
	resolver := container.NewResolver(c.Architecture.String())

	containerSpecs := make(map[string][]container.Spec)
	for plName, sourceSpecs := range mani.GetContainerSourceSpecs() {
		for _, c := range sourceSpecs {
			resolver.Add(c)
		}
		specs, err := resolver.Finish()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot resolve containers: %w", err)
		}
		for _, spec := range specs {
			if spec.Arch != c.Architecture {
				return nil, nil, fmt.Errorf("image found is for unexpected architecture %q (expected %q), if that is intentional, please make sure --target-arch matches", spec.Arch, c.Architecture)
			}
		}
		containerSpecs[plName] = specs
	}

	var opts manifest.SerializeOptions
	if c.UseLibrepo {
		opts.RpmDownloader = osbuild.RpmDownloaderLibrepo
	}
	mf, err := mani.Serialize(depsolvedSets, containerSpecs, nil, &opts)
	if err != nil {
		return nil, nil, fmt.Errorf("[ERROR] manifest serialization failed: %s", err.Error())
	}
	return mf, depsolvedRepos, nil
}

func saveManifest(ms manifest.OSBuildManifest, fpath string) error {
	b, err := json.MarshalIndent(ms, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data for %q: %s", fpath, err.Error())
	}
	b = append(b, '\n') // add new line at end of file
	fp, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %s", fpath, err.Error())
	}
	defer fp.Close()
	if _, err := fp.Write(b); err != nil {
		return fmt.Errorf("failed to write output file %q: %s", fpath, err.Error())
	}
	return nil
}

// manifestFromCobra generate an osbuild manifest from a cobra commandline.
//
// It takes an unstarted progres bar and will start it at the right
// point (it cannot be started yet to avoid the "podman pull" progress
// and our progress fighting). The caller is responsible for stopping
// the progress bar (this function cannot know what else needs to happen
// after manifest generation).
//
// TODO: provide a podman progress reader to integrate the podman progress
// into our progress.
func manifestFromCobra(cmd *cobra.Command, args []string, pbar progress.ProgressBar) ([]byte, *mTLSConfig, error) {
	cntArch := arch.Current()

	imgref := args[0]
	userConfigFile, _ := cmd.Flags().GetString("config")
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	rpmCacheRoot, _ := cmd.Flags().GetString("rpmmd")
	targetArch, _ := cmd.Flags().GetString("target-arch")
	rootFs, _ := cmd.Flags().GetString("rootfs")
	useLibrepo, _ := cmd.Flags().GetBool("use-librepo")

	// If --local was given, warn in the case of --local or --local=true (true is the default), error in the case of --local=false
	if cmd.Flags().Changed("local") {
		localStorage, _ := cmd.Flags().GetBool("local")
		if localStorage {
			fmt.Fprintf(os.Stderr, "WARNING: --local is now the default behavior, you can remove it from the command line\n")
		} else {
			return nil, nil, fmt.Errorf(`--local=false is no longer supported, remove it and make sure to pull the container before running bib:
	sudo podman pull %s`, imgref)
		}
	}

	if targetArch != "" && arch.FromString(targetArch) != arch.Current() {
		// TODO: detect if binfmt_misc for target arch is
		// available, e.g. by mounting the binfmt_misc fs into
		// the container and inspects the files or by
		// including tiny statically linked target-arch
		// binaries inside our bib container
		fmt.Fprintf(os.Stderr, "WARNING: target-arch is experimental and needs an installed 'qemu-user' package\n")
		if slices.Contains(imgTypes, "iso") {
			return nil, nil, fmt.Errorf("cannot build iso for different target arches yet")
		}
		cntArch = arch.FromString(targetArch)
	}
	// TODO: add "target-variant", see https://github.com/osbuild/bootc-image-builder/pull/139/files#r1467591868

	if err := setup.ValidateHasContainerStorageMounted(); err != nil {
		return nil, nil, fmt.Errorf("could not access container storage, did you forget -v /var/lib/containers/storage:/var/lib/containers/storage? (%w)", err)
	}

	imageTypes, err := imagetypes.New(imgTypes...)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot detect build types %v: %w", imgTypes, err)
	}

	config, err := buildconfig.ReadWithFallback(userConfigFile)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read config: %w", err)
	}

	pbar.SetPulseMsgf("Manifest generation step")
	pbar.Start()

	if err := setup.ValidateHasContainerTags(imgref); err != nil {
		return nil, nil, err
	}

	cntSize, err := getContainerSize(imgref)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get container size: %w", err)
	}
	container, err := podman_container.New(imgref)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := container.Stop(); err != nil {
			logrus.Warnf("error stopping container: %v", err)
		}
	}()

	var rootfsType string
	if !imageTypes.BuildsISO() {
		if rootFs != "" {
			rootfsType = rootFs
		} else {
			rootfsType, err = container.DefaultRootfsType()
			if err != nil {
				return nil, nil, fmt.Errorf("cannot get rootfs type for container: %w", err)
			}
			if rootfsType == "" {
				return nil, nil, fmt.Errorf(`no default root filesystem type specified in container, please use "--rootfs" to set manually`)
			}
		}

		// TODO: on a cross arch build we need to be conservative, i.e. we can
		// only use the default ext4 because if xfs is select we run into the
		// issue that mkfs.xfs calls "ioctl(BLKBSZSET)" which is missing in
		// qemu-user.
		// The fix has been merged upstream https://www.mail-archive.com/qemu-devel@nongnu.org/msg1037409.html
		// and is expected to be included in v9.1.0 https://github.com/qemu/qemu/commit/e6e903db6a5e960e595f9f1fd034adb942dd9508
		// Remove the following condition once we update to qemu-user v9.1.0.
		if cntArch != arch.Current() && rootfsType != "ext4" {
			logrus.Warningf("container preferred root filesystem %q cannot be used during cross arch build", rootfsType)
			rootfsType = "ext4"
		}
	}
	// Gather some data from the containers distro
	sourceinfo, err := source.LoadInfo(container.Root())
	if err != nil {
		return nil, nil, err
	}

	// This is needed just for RHEL and RHSM in most cases, but let's run it every time in case
	// the image has some non-standard dnf plugins.
	if err := container.InitDNF(); err != nil {
		return nil, nil, err
	}
	solver, err := container.NewContainerSolver(rpmCacheRoot, cntArch, sourceinfo)
	if err != nil {
		return nil, nil, err
	}

	manifestConfig := &ManifestConfig{
		Architecture:   cntArch,
		Config:         config,
		ImageTypes:     imageTypes,
		Imgref:         imgref,
		RootfsMinsize:  cntSize * containerSizeToDiskSizeMultiplier,
		DistroDefPaths: distroDefPaths,
		SourceInfo:     sourceinfo,
		RootFSType:     rootfsType,
		UseLibrepo:     useLibrepo,
	}

	manifest, repos, err := makeManifest(manifestConfig, solver, rpmCacheRoot)
	if err != nil {
		return nil, nil, err
	}

	mTLS, err := extractTLSKeys(SimpleFileReader{}, repos)
	if err != nil {
		return nil, nil, err
	}

	return manifest, mTLS, nil
}

func cmdManifest(cmd *cobra.Command, args []string) error {
	pbar, err := progress.New("")
	if err != nil {
		// this should never happen
		return fmt.Errorf("cannot create progress bar: %w", err)
	}
	defer pbar.Stop()

	mf, _, err := manifestFromCobra(cmd, args, pbar)
	if err != nil {
		return fmt.Errorf("cannot generate manifest: %w", err)
	}
	fmt.Println(string(mf))
	return nil
}

func handleAWSFlags(cmd *cobra.Command) (upload bool, err error) {
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	region, _ := cmd.Flags().GetString("aws-region")
	if region == "" {
		return false, nil
	}
	bucketName, _ := cmd.Flags().GetString("aws-bucket")

	if !slices.Contains(imgTypes, "ami") {
		return false, fmt.Errorf("aws flags set for non-ami image type (type is set to %s)", strings.Join(imgTypes, ","))
	}

	// check as many permission prerequisites as possible before starting
	client, err := awscloud.NewDefault(region)
	if err != nil {
		return false, err
	}

	logrus.Info("Checking AWS region access...")
	regions, err := client.Regions()
	if err != nil {
		return false, fmt.Errorf("retrieving AWS regions for '%s' failed: %w", region, err)
	}

	if !slices.Contains(regions, region) {
		return false, fmt.Errorf("given AWS region '%s' not found", region)
	}

	logrus.Info("Checking AWS bucket...")
	buckets, err := client.Buckets()
	if err != nil {
		return false, fmt.Errorf("retrieving AWS list of buckets failed: %w", err)
	}
	if !slices.Contains(buckets, bucketName) {
		return false, fmt.Errorf("bucket '%s' not found in the given AWS account", bucketName)
	}

	logrus.Info("Checking AWS bucket permissions...")
	writePermission, err := client.CheckBucketPermission(bucketName, awscloud.S3PermissionWrite)
	if err != nil {
		return false, err
	}
	if !writePermission {
		return false, fmt.Errorf("you don't have write permissions to bucket '%s' with the given AWS account", bucketName)
	}
	logrus.Info("Upload conditions met.")
	return true, nil
}

func cmdBuild(cmd *cobra.Command, args []string) error {
	chown, _ := cmd.Flags().GetString("chown")
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	osbuildStore, _ := cmd.Flags().GetString("store")
	outputDir, _ := cmd.Flags().GetString("output")
	targetArch, _ := cmd.Flags().GetString("target-arch")
	progressType, _ := cmd.Flags().GetString("progress")

	logrus.Debug("Validating environment")
	if err := setup.Validate(targetArch); err != nil {
		return fmt.Errorf("cannot validate the setup: %w", err)
	}
	logrus.Debug("Ensuring environment setup")
	switch inContainerOrUnknown() {
	case false:
		fmt.Fprintf(os.Stderr, "WARNING: running outside a container, this is an unsupported configuration\n")
	case true:
		if err := setup.EnsureEnvironment(osbuildStore); err != nil {
			return fmt.Errorf("cannot ensure the environment: %w", err)
		}
	}

	if err := os.MkdirAll(outputDir, 0o777); err != nil {
		return fmt.Errorf("cannot setup build dir: %w", err)
	}

	upload, err := handleAWSFlags(cmd)
	if err != nil {
		return fmt.Errorf("cannot handle AWS setup: %w", err)
	}

	canChown, err := canChownInPath(outputDir)
	if err != nil {
		return fmt.Errorf("cannot ensure ownership: %w", err)
	}
	if !canChown && chown != "" {
		return fmt.Errorf("chowning is not allowed in output directory")
	}

	pbar, err := progress.New(progressType)
	if err != nil {
		return fmt.Errorf("cannto create progress bar: %w", err)
	}
	defer pbar.Stop()

	manifest_fname := fmt.Sprintf("manifest-%s.json", strings.Join(imgTypes, "-"))
	pbar.SetMessagef("Generating manifest %s", manifest_fname)
	mf, mTLS, err := manifestFromCobra(cmd, args, pbar)
	if err != nil {
		return fmt.Errorf("cannot build manifest: %w", err)
	}
	pbar.SetMessagef("Done generating manifest")

	// collect pipeline exports for each image type
	imageTypes, err := imagetypes.New(imgTypes...)
	if err != nil {
		return err
	}
	exports := imageTypes.Exports()
	manifestPath := filepath.Join(outputDir, manifest_fname)
	if err := saveManifest(mf, manifestPath); err != nil {
		return fmt.Errorf("cannot save manifest: %w", err)
	}

	pbar.SetPulseMsgf("Image building step")
	pbar.SetMessagef("Building %s", manifest_fname)

	var osbuildEnv []string
	if !canChown {
		// set export options for osbuild
		osbuildEnv = []string{"OSBUILD_EXPORT_FORCE_NO_PRESERVE_OWNER=1"}
	}

	if mTLS != nil {
		envVars, cleanup, err := prepareOsbuildMTLSConfig(mTLS)
		if err != nil {
			return fmt.Errorf("failed to prepare osbuild TLS keys: %w", err)
		}

		defer cleanup()

		osbuildEnv = append(osbuildEnv, envVars...)
	}

	osbuildOpts := progress.OSBuildOptions{
		StoreDir:  osbuildStore,
		OutputDir: outputDir,
		ExtraEnv:  osbuildEnv,
	}
	if err = progress.RunOSBuild(pbar, mf, exports, &osbuildOpts); err != nil {
		return fmt.Errorf("cannot run osbuild: %w", err)
	}

	pbar.SetMessagef("Build complete!")
	if upload {
		// XXX: pass our own progress.ProgressBar here
		// *for now* just stop our own progress and let the uploadAMI
		// progress take over - but we really need to fix this in a
		// followup
		pbar.Stop()
		for idx, imgType := range imgTypes {
			switch imgType {
			case "ami":
				diskpath := filepath.Join(outputDir, exports[idx], "disk.raw")
				if err := uploadAMI(diskpath, targetArch, cmd.Flags()); err != nil {
					return fmt.Errorf("cannot upload AMI: %w", err)
				}
			default:
				continue
			}
		}
	} else {
		pbar.SetMessagef("Results saved in %s", outputDir)
	}

	if err := chownR(outputDir, chown); err != nil {
		return fmt.Errorf("cannot setup owner for %q: %w", outputDir, err)
	}

	return nil
}

func chownR(path string, chown string) error {
	if chown == "" {
		return nil
	}
	errFmt := "cannot parse chown: %v"

	var gid int
	uidS, gidS, _ := strings.Cut(chown, ":")
	uid, err := strconv.Atoi(uidS)
	if err != nil {
		return fmt.Errorf(errFmt, err)
	}
	if gidS != "" {
		gid, err = strconv.Atoi(gidS)
		if err != nil {
			return fmt.Errorf(errFmt, err)
		}
	} else {
		gid = osGetgid()
	}

	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = os.Chown(name, uid, gid)
		}
		return err
	})
}

var rootLogLevel string

func rootPreRunE(cmd *cobra.Command, _ []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	progress, _ := cmd.Flags().GetString("progress")
	switch {
	case rootLogLevel != "":
		level, err := logrus.ParseLevel(rootLogLevel)
		if err != nil {
			return err
		}
		logrus.SetLevel(level)
	case verbose:
		logrus.SetLevel(logrus.InfoLevel)
	default:
		logrus.SetLevel(logrus.ErrorLevel)
	}
	if verbose && progress == "auto" {
		if err := cmd.Flags().Set("progress", "verbose"); err != nil {
			return err
		}
	}

	return nil
}

func versionFromBuildInfo() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("cannot read build info")
	}
	var buildTainted bool
	gitRev := "unknown"
	buildTime := "unknown"
	for _, bs := range info.Settings {
		switch bs.Key {
		case "vcs.revision":
			gitRev = bs.Value[:7]
		case "vcs.time":
			buildTime = bs.Value
		case "vcs.modified":
			bT, err := strconv.ParseBool(bs.Value)
			if err != nil {
				logrus.Errorf("Error parsing 'vcs.modified': %v", err)
				bT = true
			}
			buildTainted = bT
		}
	}

	return fmt.Sprintf(`build_revision: %s
build_time: %s
build_tainted: %v
`, gitRev, buildTime, buildTainted), nil
}

func buildCobraCmdline() (*cobra.Command, error) {
	version, err := versionFromBuildInfo()
	if err != nil {
		return nil, err
	}

	rootCmd := &cobra.Command{
		Use:               "bootc-image-builder",
		Long:              "Create a bootable image from an ostree native container",
		PersistentPreRunE: rootPreRunE,
		SilenceErrors:     true,
		Version:           version,
	}
	rootCmd.SetVersionTemplate(version)

	rootCmd.PersistentFlags().StringVar(&rootLogLevel, "log-level", "", "logging level (debug, info, error); default error")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, `Switch to verbose mode`)

	buildCmd := &cobra.Command{
		Use:   "build IMAGE_NAME",
		Short: rootCmd.Long + " (default command)",
		Long: rootCmd.Long + "\n" +
			"(default action if no command is given)\n" +
			"IMAGE_NAME: container image to build into a bootable image",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE:                  cmdBuild,
		SilenceUsage:          true,
		Example: rootCmd.Use + " build quay.io/centos-bootc/centos-bootc:stream9\n" +
			rootCmd.Use + " quay.io/centos-bootc/centos-bootc:stream9\n",
		Version: rootCmd.Version,
	}
	buildCmd.SetVersionTemplate(version)

	rootCmd.AddCommand(buildCmd)
	manifestCmd := &cobra.Command{
		Use:                   "manifest",
		Short:                 "Only create the manifest but don't build the image.",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE:                  cmdManifest,
		SilenceUsage:          true,
		Version:               rootCmd.Version,
	}
	manifestCmd.SetVersionTemplate(version)

	versionCmd := &cobra.Command{
		Use:          "version",
		Short:        "Show the version and quit",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			root.SetArgs([]string{"--version"})
			return root.Execute()
		},
	}

	rootCmd.AddCommand(versionCmd)

	rootCmd.AddCommand(manifestCmd)
	manifestCmd.Flags().Bool("tls-verify", false, "DEPRECATED: require HTTPS and verify certificates when contacting registries")
	if err := manifestCmd.Flags().MarkHidden("tls-verify"); err != nil {
		return nil, fmt.Errorf("cannot hide 'tls-verify' :%w", err)
	}
	manifestCmd.Flags().String("rpmmd", "/rpmmd", "rpm metadata cache directory")
	manifestCmd.Flags().String("target-arch", "", "build for the given target architecture (experimental)")
	manifestCmd.Flags().StringArray("type", []string{"qcow2"}, fmt.Sprintf("image types to build [%s]", imagetypes.Available()))
	manifestCmd.Flags().Bool("local", true, "DEPRECATED: --local is now the default behavior, make sure to pull the container image before running bootc-image-builder")
	if err := manifestCmd.Flags().MarkHidden("local"); err != nil {
		return nil, fmt.Errorf("cannot hide 'local' :%w", err)
	}
	manifestCmd.Flags().String("rootfs", "", "Root filesystem type. If not given, the default configured in the source container image is used.")
	manifestCmd.Flags().Bool("use-librepo", false, "(experimenal) switch to librepo for pkg download, needs new enough osbuild")
	// --config is only useful for developers who run bib outside
	// of a container to generate a manifest. so hide it by
	// default from users.
	manifestCmd.Flags().String("config", "", "build config file; /config.json will be used if present")
	if err := manifestCmd.Flags().MarkHidden("config"); err != nil {
		return nil, fmt.Errorf("cannot hide 'config' :%w", err)
	}

	buildCmd.Flags().AddFlagSet(manifestCmd.Flags())
	buildCmd.Flags().String("aws-ami-name", "", "name for the AMI in AWS (only for type=ami)")
	buildCmd.Flags().String("aws-bucket", "", "target S3 bucket name for intermediate storage when creating AMI (only for type=ami)")
	buildCmd.Flags().String("aws-region", "", "target region for AWS uploads (only for type=ami)")
	buildCmd.Flags().String("chown", "", "chown the ouput directory to match the specified UID:GID")
	buildCmd.Flags().String("output", ".", "artifact output directory")
	buildCmd.Flags().String("store", "/store", "osbuild store for intermediate pipeline trees")
	//TODO: add json progress for higher level tools like "podman bootc"
	buildCmd.Flags().String("progress", "auto", "type of progress bar to use (e.g. verbose,term)")
	// flag rules
	for _, dname := range []string{"output", "store", "rpmmd"} {
		if err := buildCmd.MarkFlagDirname(dname); err != nil {
			return nil, err
		}
	}
	if err := buildCmd.MarkFlagFilename("config"); err != nil {
		return nil, err
	}
	buildCmd.MarkFlagsRequiredTogether("aws-region", "aws-bucket", "aws-ami-name")

	// If no subcommand is given, assume the user wants to use the build subcommand
	// See https://github.com/spf13/cobra/issues/823#issuecomment-870027246
	// which cannot be used verbatim because the arguments for "build" like
	// "quay.io" will create an "err != nil". Ideally we could check err
	// for something like cobra.UnknownCommandError but cobra just gives
	// us an error string
	cmd, _, err := rootCmd.Find(os.Args[1:])
	injectBuildArg := func() {
		args := append([]string{buildCmd.Name()}, os.Args[1:]...)
		rootCmd.SetArgs(args)
	}
	// command not known, i.e. happens for "bib quay.io/centos/..."
	if err != nil && !slices.Contains([]string{"help", "completion"}, os.Args[1]) {
		injectBuildArg()
	}
	// command appears valid, e.g. "bib --local quay.io/centos" but this
	// is the parser just assuming "quay.io" is an argument for "--local" :(
	if err == nil && cmd.Use == rootCmd.Use && cmd.Flags().Parse(os.Args[1:]) != pflag.ErrHelp {
		injectBuildArg()
	}

	return rootCmd, nil
}

func run() error {
	rootCmd, err := buildCobraCmdline()
	if err != nil {
		return err
	}

	return rootCmd.Execute()
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %s", err)
	}
}
