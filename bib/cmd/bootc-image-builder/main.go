package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"

	podman_container "github.com/osbuild/bootc-image-builder/bib/internal/container"
	"github.com/osbuild/bootc-image-builder/bib/internal/setup"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
	"github.com/osbuild/bootc-image-builder/bib/internal/util"
)

const (
	// If present, this config will be picked up
	configFileDefault = "/config.json"

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

type BuildConfig struct {
	Blueprint *blueprint.Blueprint `json:"blueprint,omitempty"`
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

func loadConfig(path string) (*BuildConfig, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	dec := json.NewDecoder(fp)
	dec.DisallowUnknownFields()

	var conf BuildConfig
	if err := dec.Decode(&conf); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("multiple configuration objects or extra data found in %q", path)
	}
	return &conf, nil
}

// getContainerArch returns the architecture of an already pulled container image
func getContainerArch(imgref string) (cntArch arch.Arch, err error) {
	outputB, err := exec.Command("podman", "image", "inspect", imgref, "--format", "{{.Architecture}}").Output()
	if err != nil {
		return 0, fmt.Errorf("failed inspect image for architecture: %w", util.OutputErr(err))
	}
	output := strings.TrimSpace(string(outputB))

	// TODO: make images:arch.FromString() return an error
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("cannot convert %q to an architecture", output)
		}
	}()
	cntArch = arch.FromString(output)

	return cntArch, nil
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
	solver.SetDNFJSONPath(c.DepsolverCmd[0], c.DepsolverCmd[1:]...)
	solver.SetRootDir("/")
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
		containerSpecs[plName], err = resolver.Finish()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot resolve containers: %w", err)
		}
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
	configFile, _ := cmd.Flags().GetString("config")
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	rpmCacheRoot, _ := cmd.Flags().GetString("rpmmd")
	targetArch, _ := cmd.Flags().GetString("target-arch")
	tlsVerify, _ := cmd.Flags().GetBool("tls-verify")
	localStorage, _ := cmd.Flags().GetBool("local")

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

	var config *BuildConfig
	// If we're not passed a config path explicitly, use the default.
	if configFile == "" {
		if _, err := os.Stat(configFileDefault); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, nil, fmt.Errorf("cannot find config file: %w", err)
			}
		} else {
			configFile = configFileDefault
		}
	}
	if configFile != "" {
		config, err = loadConfig(configFile)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot load config: %w", err)
		}
	} else {
		config = &BuildConfig{}
	}

	// If --local wasn't given, always pull the container.
	// If the user mount a container storage inside bib (without --local), the code will try to pull
	// a newer version of the container even if an older one is already present. This doesn't match
	// how `podman run`` behaves by default, but it matches the bib's behaviour before the switch
	// to using containers storage in all code paths happened.
	// We might want to change this behaviour in the future to match podman.
	if !localStorage {
		if output, err := exec.Command("podman", "pull", "--arch", cntArch.String(), fmt.Sprintf("--tls-verify=%v", tlsVerify), imgref).CombinedOutput(); err != nil {
			return nil, nil, fmt.Errorf("failed to pull container image: %w\n%s", err, output)
		}
	}

	// TODO: check arch compat before pulling
	pulledCntArch, err := getContainerArch(imgref)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get container architecture: %w", err)
	}
	if cntArch != pulledCntArch {
		return nil, nil, fmt.Errorf("image found is for unexpected architecture %q (expected %q), if that is intentional, please make sure --target-arch matches", pulledCntArch, cntArch)
	}
	cntSize, err := getContainerSize(imgref)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get container size: %w", err)
	}
	filesystems := []blueprint.FilesystemCustomization{
		{Mountpoint: "/", MinSize: cntSize * containerSizeToDiskSizeMultiplier},
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

	if err := container.CopyInto("/usr/libexec/osbuild-depsolve-dnf", "/osbuild-depsolve-dnf"); err != nil {
		return nil, nil, fmt.Errorf("cannot prepare depsolve in the container: %w", err)
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
		Architecture:   cntArch,
		Config:         config,
		BuildType:      buildType,
		Imgref:         imgref,
		TLSVerify:      tlsVerify,
		Filesystems:    filesystems,
		DistroDefPaths: distroDefPaths,
		SourceInfo:     sourceinfo,
		DepsolverCmd:   append(container.ExecArgv(), "/osbuild-depsolve-dnf"),
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

	if err := setup.Validate(); err != nil {
		return fmt.Errorf("cannot validate the setup: %w", err)
	}
	if err := setup.EnsureEnvironment(osbuildStore); err != nil {
		return fmt.Errorf("cannot ensure the environment: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0777); err != nil {
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

func run() error {
	rootCmd := &cobra.Command{
		Use:  "bootc-image-builder",
		Long: "create a bootable image from an ostree native container",
	}

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
	rootCmd.AddCommand(manifestCmd)
	manifestCmd.Flags().Bool("tls-verify", true, "require HTTPS and verify certificates when contacting registries")
	manifestCmd.Flags().String("config", "", "build config file; /config.json will be used if present")
	manifestCmd.Flags().String("rpmmd", "/rpmmd", "rpm metadata cache directory")
	manifestCmd.Flags().String("target-arch", "", "build for the given target architecture (experimental)")
	manifestCmd.Flags().StringArray("type", []string{"qcow2"}, fmt.Sprintf("image types to build [%s]", allImageTypesString()))
	manifestCmd.Flags().Bool("local", false, "use a local container rather than a container from a registry")

	logrus.SetLevel(logrus.ErrorLevel)
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
