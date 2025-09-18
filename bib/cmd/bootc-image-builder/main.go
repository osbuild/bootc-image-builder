package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

	repos "github.com/osbuild/images/data/repositories"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/blueprintload"
	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/distro/bootc"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/experimentalflags"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/manifestgen"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"

	"github.com/osbuild/bootc-image-builder/bib/internal/imagetypes"
	podman_container "github.com/osbuild/images/pkg/bib/container"
	"github.com/osbuild/images/pkg/bib/osinfo"

	"github.com/osbuild/image-builder-cli/pkg/progress"
	"github.com/osbuild/image-builder-cli/pkg/setup"
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

	osStdout = os.Stdout
	osStderr = os.Stderr
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

func makeManifest(c *ManifestConfig, solver *depsolvednf.Solver, cacheRoot string) (manifest.OSBuildManifest, map[string][]rpmmd.RepoConfig, error) {
	rng := createRand()
	mani, err := manifestForISO(c, rng)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get manifest: %w", err)
	}

	// depsolve packages
	depsolvedSets := make(map[string]depsolvednf.DepsolveResult)
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

func saveManifest(ms manifest.OSBuildManifest, fpath string) (err error) {
	b, err := json.MarshalIndent(ms, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data for %q: %s", fpath, err.Error())
	}
	b = append(b, '\n') // add new line at end of file
	fp, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %s", fpath, err.Error())
	}
	defer func() { err = errors.Join(err, fp.Close()) }()
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
	buildImgref, _ := cmd.Flags().GetString("build-container")
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

	if targetArch != "" {
		target, err := arch.FromString(targetArch)
		if err != nil {
			return nil, nil, err
		}
		if target != arch.Current() {
			// TODO: detect if binfmt_misc for target arch is
			// available, e.g. by mounting the binfmt_misc fs into
			// the container and inspects the files or by
			// including tiny statically linked target-arch
			// binaries inside our bib container
			fmt.Fprintf(os.Stderr, "WARNING: target-arch is experimental and needs an installed 'qemu-user' package\n")
			if slices.Contains(imgTypes, "iso") {
				return nil, nil, fmt.Errorf("cannot build iso for different target arches yet")
			}
			cntArch = target
		}
	}
	// TODO: add "target-variant", see https://github.com/osbuild/bootc-image-builder/pull/139/files#r1467591868

	if err := setup.ValidateHasContainerStorageMounted(); err != nil {
		return nil, nil, fmt.Errorf("could not access container storage, did you forget -v /var/lib/containers/storage:/var/lib/containers/storage? (%w)", err)
	}

	imageTypes, err := imagetypes.New(imgTypes...)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot detect build types %v: %w", imgTypes, err)
	}
	config, err := blueprintload.LoadWithFallback(userConfigFile)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read config: %w", err)
	}

	if err := setup.ValidateHasContainerTags(imgref); err != nil {
		return nil, nil, err
	}

	pbar.SetPulseMsgf("Manifest generation step")
	pbar.Start()

	// For now shortcut here and build ding "images" for anything
	// that is not the iso
	if !imageTypes.BuildsISO() {
		distro, err := bootc.NewBootcDistro(imgref)
		if err != nil {
			return nil, nil, err
		}
		if err := distro.SetBuildContainer(buildImgref); err != nil {
			return nil, nil, err
		}
		if err := distro.SetDefaultFs(rootFs); err != nil {
			return nil, nil, err
		}
		// XXX: consider target-arch
		archi, err := distro.GetArch(cntArch.String())
		if err != nil {
			return nil, nil, err
		}
		// XXX: how to generate for all image types
		imgType, err := archi.GetImageType(imgTypes[0])
		if err != nil {
			return nil, nil, err
		}

		var buf bytes.Buffer
		repos, err := reporegistry.New(nil, []fs.FS{repos.FS})
		if err != nil {
			return nil, nil, err
		}
		mg, err := manifestgen.New(repos, &manifestgen.Options{
			Output: &buf,
			// XXX: hack to skip repo loading for the bootc image.
			// We need to add a SkipRepositories or similar to
			// manifestgen instead to make this clean
			OverrideRepos: []rpmmd.RepoConfig{
				{
					BaseURLs: []string{"https://example.com/not-used"},
				},
			},
		})
		if err != nil {
			return nil, nil, err
		}
		if err := mg.Generate(config, distro, imgType, archi, nil); err != nil {
			return nil, nil, err
		}
		return buf.Bytes(), nil, nil
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

	// Gather some data from the containers distro
	sourceinfo, err := osinfo.Load(container.Root())
	if err != nil {
		return nil, nil, err
	}

	buildContainer := container
	buildSourceinfo := sourceinfo
	startedBuildContainer := false
	defer func() {
		if startedBuildContainer {
			if err := buildContainer.Stop(); err != nil {
				logrus.Warnf("error stopping container: %v", err)
			}
		}
	}()

	if buildImgref != "" {
		buildContainer, err = podman_container.New(buildImgref)
		if err != nil {
			return nil, nil, err
		}
		startedBuildContainer = true

		// Gather some data from the containers distro
		buildSourceinfo, err = osinfo.Load(buildContainer.Root())
		if err != nil {
			return nil, nil, err
		}
	} else {
		buildImgref = imgref
	}

	// This is needed just for RHEL and RHSM in most cases, but let's run it every time in case
	// the image has some non-standard dnf plugins.
	if err := buildContainer.InitDNF(); err != nil {
		return nil, nil, err
	}
	solver, err := buildContainer.NewContainerSolver(rpmCacheRoot, cntArch, sourceinfo)
	if err != nil {
		return nil, nil, err
	}

	manifestConfig := &ManifestConfig{
		Architecture:    cntArch,
		Config:          config,
		ImageTypes:      imageTypes,
		Imgref:          imgref,
		BuildImgref:     buildImgref,
		DistroDefPaths:  distroDefPaths,
		SourceInfo:      sourceinfo,
		BuildSourceInfo: buildSourceinfo,
		RootFSType:      rootfsType,
		UseLibrepo:      useLibrepo,
	}

	manifest, repos, err := makeManifest(manifestConfig, solver, rpmCacheRoot)
	if err != nil {
		return nil, nil, err
	}

	mTLS, err := extractTLSKeys(repos)
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

func handleAWSFlags(cmd *cobra.Command) (cloud.Uploader, error) {
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	region, _ := cmd.Flags().GetString("aws-region")
	if region == "" {
		return nil, nil
	}
	bucketName, _ := cmd.Flags().GetString("aws-bucket")
	imageName, _ := cmd.Flags().GetString("aws-ami-name")
	targetArchStr, _ := cmd.Flags().GetString("target-arch")

	if !slices.Contains(imgTypes, "ami") {
		return nil, fmt.Errorf("aws flags set for non-ami image type (type is set to %s)", strings.Join(imgTypes, ","))
	}

	// check as many permission prerequisites as possible before starting
	targetArch := arch.Current()
	if targetArchStr != "" {
		var err error
		targetArch, err = arch.FromString(targetArchStr)
		if err != nil {
			return nil, err
		}
	}
	uploaderOpts := &awscloud.UploaderOptions{
		TargetArch: targetArch,
	}
	uploader, err := awscloud.NewUploader(region, bucketName, imageName, uploaderOpts)
	if err != nil {
		return nil, err
	}
	status := io.Discard
	if logrus.GetLevel() >= logrus.InfoLevel {
		status = os.Stderr
	}
	if err := uploader.Check(status); err != nil {
		return nil, err
	}
	return uploader, nil
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

	uploader, err := handleAWSFlags(cmd)
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

	pbar.SetPulseMsgf("Disk image building step")
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

	if experimentalflags.Bool("debug-qemu-user") {
		osbuildEnv = append(osbuildEnv, "OBSBUILD_EXPERIMENAL=debug-qemu-user")
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
	if uploader != nil {
		// XXX: pass our own progress.ProgressBar here
		// *for now* just stop our own progress and let the uploadAMI
		// progress take over - but we really need to fix this in a
		// followup
		pbar.Stop()
		for idx, imgType := range imgTypes {
			switch imgType {
			case "ami":
				diskpath := filepath.Join(outputDir, exports[idx], "disk.raw")
				if err := upload(uploader, diskpath, cmd.Flags()); err != nil {
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
	manifestCmd.Flags().String("build-container", "", "Use a custom container for the image build")
	manifestCmd.Flags().StringArray("type", []string{"qcow2"}, fmt.Sprintf("image types to build [%s]", imagetypes.Available()))
	manifestCmd.Flags().Bool("local", true, "DEPRECATED: --local is now the default behavior, make sure to pull the container image before running bootc-image-builder")
	if err := manifestCmd.Flags().MarkHidden("local"); err != nil {
		return nil, fmt.Errorf("cannot hide 'local' :%w", err)
	}
	manifestCmd.Flags().String("rootfs", "", "Root filesystem type. If not given, the default configured in the source container image is used.")
	manifestCmd.Flags().Bool("use-librepo", true, "switch to librepo for pkg download, needs new enough osbuild")
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
