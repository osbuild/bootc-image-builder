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
	"github.com/osbuild/bootc-image-builder/bib/internal/setup"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
	"github.com/osbuild/bootc-image-builder/bib/internal/util"
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

func makeManifest(c *ManifestConfig, cacheRoot string) (manifest.OSBuildManifest, map[string][]rpmmd.RepoConfig, error) {
	manifest, err := Manifest(c)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get manifest: %w", err)
	}

	// depsolve packages
	solver := dnfjson.NewSolver(
		c.SourceInfo.OSRelease.PlatformID,
		c.SourceInfo.OSRelease.VersionID,
		c.Architecture.String(),
		fmt.Sprintf("%s-%s", c.SourceInfo.OSRelease.ID, c.SourceInfo.OSRelease.VersionID),
		cacheRoot)
	solver.SetRootDir(c.DepsolverRootDir)
	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	depsolvedRepos := make(map[string][]rpmmd.RepoConfig)
	for name, pkgSet := range manifest.GetPackageSetChains() {
		res, repos, err := solver.Depsolve(pkgSet)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot depsolve: %w", err)
		}
		depsolvedSets[name] = res
		depsolvedRepos[name] = repos
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
	for plName, sourceSpecs := range manifest.GetContainerSourceSpecs() {
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

	mf, err := manifest.Serialize(depsolvedSets, containerSpecs, nil, depsolvedRepos)
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

func manifestFromCobra(cmd *cobra.Command, args []string) ([]byte, *mTLSConfig, error) {
	cntArch := arch.Current()

	imgref := args[0]
	userConfigFile, _ := cmd.Flags().GetString("config")
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	rpmCacheRoot, _ := cmd.Flags().GetString("rpmmd")
	targetArch, _ := cmd.Flags().GetString("target-arch")
	tlsVerify, _ := cmd.Flags().GetBool("tls-verify")
	localStorage, _ := cmd.Flags().GetBool("local")
	rootFs, _ := cmd.Flags().GetString("rootfs")

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

	if localStorage {
		if err := setup.ValidateHasContainerStorageMounted(); err != nil {
			return nil, nil, fmt.Errorf("local storage not working, did you forget -v /var/lib/containers/storage:/var/lib/containers/storage? (%w)", err)
		}
	}

	buildType, err := NewBuildType(imgTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot detect build types %v: %w", imgTypes, err)
	}

	config, err := buildconfig.ReadWithFallback(userConfigFile)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read config: %w", err)
	}

	// If --local wasn't given, always pull the container.
	// If the user mount a container storage inside bib (without --local), the code will try to pull
	// a newer version of the container even if an older one is already present. This doesn't match
	// how `podman run`` behaves by default, but it matches the bib's behaviour before the switch
	// to using containers storage in all code paths happened.
	// We might want to change this behaviour in the future to match podman.
	if !localStorage {
		logrus.Infof("Pulling image %s (arch=%s)\n", imgref, cntArch)
		cmd := exec.Command("podman", "pull", "--arch", cntArch.String(), fmt.Sprintf("--tls-verify=%v", tlsVerify), imgref)
		// podman prints progress on stderr so connect that to give
		// better UX. But do not connect stdout as "bib manifest"
		// needs to be purely json so any stray output there is a
		// problem
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, nil, fmt.Errorf("failed to pull container image: %w", util.OutputErr(err))
		}
	} else {
		logrus.Debug("Using local container")
	}

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
	if buildType != BuildTypeISO {
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

		// TODO: on a cross arch build we need to be conservative,
		// i.e.  we can only use the default ext4 because if xfs is
		// select we run into the issue that mkfs.xfs calls
		// "ioctl(BLKBSZSET)" which is missing in qemu-user, once
		// https://www.mail-archive.com/qemu-devel@nongnu.org/msg1037409.html
		// is merged we can remove the following code
		if cntArch != arch.Current() && rootfsType != "ext4" {
			logrus.Warningf("container preferred root filesystem %q cannot be used during cross arch build", rootfsType)
			rootfsType = ""
		}
	}

	// This is needed just for RHEL and RHSM in most cases, but let's run it every time in case
	// the image has some non-standard dnf plugins.
	if err := container.InitDNF(); err != nil {
		return nil, nil, err
	}

	sourceinfo, err := source.LoadInfo(container.Root())
	if err != nil {
		return nil, nil, err
	}

	manifestConfig := &ManifestConfig{
		Architecture:     cntArch,
		Config:           config,
		BuildType:        buildType,
		Imgref:           imgref,
		TLSVerify:        tlsVerify,
		RootfsMinsize:    cntSize * containerSizeToDiskSizeMultiplier,
		DistroDefPaths:   distroDefPaths,
		SourceInfo:       sourceinfo,
		RootFSType:       rootfsType,
		DepsolverRootDir: container.Root(),
	}

	manifest, repos, err := makeManifest(manifestConfig, rpmCacheRoot)
	if err != nil {
		return nil, nil, err
	}

	mTLS, err := extractTLSKeys(container, repos)
	if err != nil {
		return nil, nil, err
	}

	return manifest, mTLS, nil
}

func cmdManifest(cmd *cobra.Command, args []string) error {
	mf, _, err := manifestFromCobra(cmd, args)
	if err != nil {
		return fmt.Errorf("cannot generate manifest: %w", err)
	}
	fmt.Print(string(mf))
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

	manifest_fname := fmt.Sprintf("manifest-%s.json", strings.Join(imgTypes, "-"))
	fmt.Printf("Generating manifest %s\n", manifest_fname)
	mf, mTLS, err := manifestFromCobra(cmd, args)
	if err != nil {
		return fmt.Errorf("cannot build manifest: %w", err)
	}
	fmt.Print("DONE\n")

	// collect pipeline exports for each image type
	var exports []string
	for _, imgType := range imgTypes {
		switch imgType {
		case "qcow2":
			exports = append(exports, "qcow2")
		case "ami", "raw":
			// this might be appended more than once, but that's okay
			exports = append(exports, "image")
		case "vmdk":
			exports = append(exports, "vmdk")

		case "anaconda-iso", "iso":
			exports = append(exports, "bootiso")
		default:
			return fmt.Errorf("valid types are %s, not: '%s'", allImageTypesString(), imgType)
		}
	}
	manifestPath := filepath.Join(outputDir, manifest_fname)
	if err := saveManifest(mf, manifestPath); err != nil {
		return fmt.Errorf("cannot save manifest: %w", err)
	}

	fmt.Printf("Building %s\n", manifest_fname)

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

	_, err = osbuild.RunOSBuild(mf, osbuildStore, outputDir, exports, nil, osbuildEnv, false, os.Stderr)
	if err != nil {
		return fmt.Errorf("cannot run osbuild: %w", err)
	}

	fmt.Println("Build complete!")
	if upload {
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
		fmt.Printf("Results saved in\n%s\n", outputDir)
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
	if rootLogLevel == "" {
		logrus.SetLevel(logrus.ErrorLevel)
		return nil
	}

	level, err := logrus.ParseLevel(rootLogLevel)
	if err != nil {
		return err
	}

	logrus.SetLevel(level)

	return nil
}

// TODO: provide more version info (like actual version number) once we
// release a real version
func cmdVersion(_ *cobra.Command, _ []string) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("cannot read build info")
	}
	var gitRev string
	for _, bs := range info.Settings {
		if bs.Key == "vcs.revision" {
			gitRev = bs.Value
			break
		}
	}
	if gitRev != "" {
		fmt.Printf("revision: %s\n", gitRev[:7])
	} else {
		fmt.Printf("revision: unknown\n")
	}
	return nil
}

func run() error {
	rootCmd := &cobra.Command{
		Use:               "bootc-image-builder",
		Long:              "create a bootable image from an ostree native container",
		PersistentPreRunE: rootPreRunE,
		SilenceErrors:     true,
	}

	rootCmd.PersistentFlags().StringVar(&rootLogLevel, "log-level", "", "logging level (debug, info, error); default error")

	buildCmd := &cobra.Command{
		Use:                   "build",
		Long:                  rootCmd.Long,
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE:                  cmdBuild,
		SilenceUsage:          true,
	}
	rootCmd.AddCommand(buildCmd)
	manifestCmd := &cobra.Command{
		Use:                   "manifest",
		Long:                  rootCmd.Long,
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE:                  cmdManifest,
		SilenceUsage:          true,
	}
	versionCmd := &cobra.Command{
		Use:          "version",
		SilenceUsage: true,
		Hidden:       true,
		RunE:         cmdVersion,
	}
	rootCmd.AddCommand(versionCmd)

	rootCmd.AddCommand(manifestCmd)
	manifestCmd.Flags().Bool("tls-verify", true, "require HTTPS and verify certificates when contacting registries")
	manifestCmd.Flags().String("rpmmd", "/rpmmd", "rpm metadata cache directory")
	manifestCmd.Flags().String("target-arch", "", "build for the given target architecture (experimental)")
	manifestCmd.Flags().StringArray("type", []string{"qcow2"}, fmt.Sprintf("image types to build [%s]", allImageTypesString()))
	manifestCmd.Flags().Bool("local", false, "use a local container rather than a container from a registry")
	manifestCmd.Flags().String("rootfs", "", "Root filesystem type. If not given, the default configured in the source container image is used.")
	// --config is only useful for developers who run bib outside
	// of a container to generate a manifest. so hide it by
	// default from users.
	manifestCmd.Flags().String("config", "", "build config file; /config.json will be used if present")
	if err := manifestCmd.Flags().MarkHidden("config"); err != nil {
		return fmt.Errorf("cannot hide 'config' :%w", err)
	}

	buildCmd.Flags().AddFlagSet(manifestCmd.Flags())
	buildCmd.Flags().String("aws-ami-name", "", "name for the AMI in AWS (only for type=ami)")
	buildCmd.Flags().String("aws-bucket", "", "target S3 bucket name for intermediate storage when creating AMI (only for type=ami)")
	buildCmd.Flags().String("aws-region", "", "target region for AWS uploads (only for type=ami)")
	buildCmd.Flags().String("chown", "", "chown the ouput directory to match the specified UID:GID")
	buildCmd.Flags().String("output", ".", "artifact output directory")
	buildCmd.Flags().String("progress", "text", "type of progress bar to use")
	buildCmd.Flags().String("store", "/store", "osbuild store for intermediate pipeline trees")

	// flag rules
	for _, dname := range []string{"output", "store", "rpmmd"} {
		if err := buildCmd.MarkFlagDirname(dname); err != nil {
			return err
		}
	}
	if err := buildCmd.MarkFlagFilename("config"); err != nil {
		return err
	}
	buildCmd.MarkFlagsRequiredTogether("aws-region", "aws-bucket", "aws-ami-name")

	return rootCmd.Execute()
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %s", err)
	}
}
