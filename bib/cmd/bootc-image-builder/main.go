package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/osbuild/bootc-image-builder/bib/internal/setup"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

//go:embed fedora-eln.json
var reposStr string

const (
	distroName       = "fedora-39"
	modulePlatformID = "platform:f39"
	releaseVersion   = "39"

	// As a baseline heuristic we double the size of
	// the input container to support in-place updates.
	// This is planned to be more configurable in the
	// future.
	containerSizeToDiskSizeMultiplier = 2
)

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

// Parse embedded repositories and return repo configs for the given
// architecture.
func loadRepos(archName string) ([]rpmmd.RepoConfig, error) {
	var repoData map[string][]rpmmd.RepoConfig
	err := json.Unmarshal([]byte(reposStr), &repoData)
	if err != nil {
		return nil, err
	}
	archRepos, ok := repoData[archName]
	if !ok {
		return nil, fmt.Errorf("no repositories defined for %s", archName)
	}
	return archRepos, nil
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

// getContainerSize returns the size of an already pulled container image in bytes
func getContainerSize(imgref string) (uint64, error) {
	var stdout, stderr strings.Builder

	cmd := exec.Command("podman", "image", "inspect", imgref, "--format", "{{.Size}}")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("failed inspect image: %w:\n%s", err, stderr.String())
	}
	size, err := strconv.ParseUint(strings.TrimSpace(stdout.String()), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse image size: %w", err)
	}

	return size, nil
}

func makeManifest(c *ManifestConfig, cacheRoot string) (manifest.OSBuildManifest, error) {
	if c.Local {
		if err := setup.ValidateHasContainerStorageMounted(); err != nil {
			return nil, fmt.Errorf("local storage not working, did you forget -v /var/lib/containers/storage:/var/lib/containers/storage? (%w)", err)
		}
	}

	// If --local wasn't given, always pull the container.
	// If the user mount a container storage inside bib (without --local), the code will try to pull
	// a newer version of the container even if an older one is already present. This doesn't match
	// how `podman run`` behaves by default, but it matches the bib's behaviour before the switch
	// to using containers storage in all code paths happened.
	// We might want to change this behaviour in the future to match podman.
	if !c.Local {
		pullCmd := exec.Command("podman", "pull", "--arch", c.Architecture.String(), c.Imgref)
		if err := pullCmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to pull container image: %w", err)
		}
	}
	cntSize, err := getContainerSize(c.Imgref)
	if err != nil {
		return nil, fmt.Errorf("cannot get container size: %w", err)
	}
	c.Filesystems = []blueprint.FilesystemCustomization{
		{Mountpoint: "/", MinSize: cntSize * containerSizeToDiskSizeMultiplier},
	}

	manifest, err := Manifest(c)
	if err != nil {
		return nil, err
	}

	// depsolve packages
	solver := dnfjson.NewSolver(modulePlatformID, releaseVersion, c.Architecture.String(), distroName, cacheRoot)
	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	for name, pkgSet := range manifest.GetPackageSetChains() {
		res, _, err := solver.Depsolve(pkgSet)
		if err != nil {
			return nil, err
		}
		depsolvedSets[name] = res
	}

	// Resolve container - the normal case is that host and target
	// architecture are the same. However it is possible to build
	// cross-arch images by using qemu-user. Just run everything
	// (including the build-root) with the target arch then, it
	// is fast enough (given that it's mostly I/O and all I/O is
	// run naively via syscall translation)
	hostArch := arch.Current().String()
	targetArch := c.Architecture.String()

	var resolver *container.Resolver
	if targetArch != "" {
		resolver = container.NewResolver(targetArch)
	} else {
		resolver = container.NewResolver(hostArch)
	}

	containerSpecs := make(map[string][]container.Spec)
	for plName, sourceSpecs := range manifest.GetContainerSourceSpecs() {
		for _, c := range sourceSpecs {
			resolver.Add(c)
		}
		containerSpecs[plName], err = resolver.Finish()
		if err != nil {
			return nil, err
		}
	}

	mf, err := manifest.Serialize(depsolvedSets, containerSpecs, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] manifest serialization failed: %s", err.Error())
	}
	return mf, nil
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

func manifestFromCobra(cmd *cobra.Command, args []string) ([]byte, error) {
	buildArch := arch.Current()
	repos, err := loadRepos(buildArch.String())
	if err != nil {
		return nil, err
	}

	imgref := args[0]
	configFile, _ := cmd.Flags().GetString("config")
	imgTypes, _ := cmd.Flags().GetStringArray("type")
	rpmCacheRoot, _ := cmd.Flags().GetString("rpmmd")
	targetArch, _ := cmd.Flags().GetString("target-arch")
	tlsVerify, _ := cmd.Flags().GetBool("tls-verify")
	localStorage, _ := cmd.Flags().GetBool("local")

	if targetArch != "" {
		// TODO: detect if binfmt_misc for target arch is
		// available, e.g. by mounting the binfmt_misc fs into
		// the container and inspects the files or by
		// including tiny statically linked target-arch
		// binaries inside our bib container
		fmt.Fprintf(os.Stderr, "WARNING: target-arch is experimental and needs an installed 'qemu-user' package\n")
		if slices.Contains(imgTypes, "iso") {
			return nil, fmt.Errorf("cannot build iso for different target arches yet")
		}
		buildArch = arch.FromString(targetArch)
	}
	// TODO: add "target-variant", see https://github.com/osbuild/bootc-image-builder/pull/139/files#r1467591868

	buildType, err := NewBuildType(imgTypes)
	if err != nil {
		return nil, err
	}

	var config *BuildConfig
	if configFile != "" {
		config, err = loadConfig(configFile)
		if err != nil {
			return nil, err
		}
	} else {
		config = &BuildConfig{}
	}

	manifestConfig := &ManifestConfig{
		Architecture: buildArch,
		Config:       config,
		BuildType:    buildType,
		Imgref:       imgref,
		Repos:        repos,
		TLSVerify:    tlsVerify,
		Local:        localStorage,
	}
	return makeManifest(manifestConfig, rpmCacheRoot)
}

func cmdManifest(cmd *cobra.Command, args []string) error {
	mf, err := manifestFromCobra(cmd, args)
	if err != nil {
		return err
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
		return err
	}
	if err := setup.EnsureEnvironment(osbuildStore); err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0777); err != nil {
		return err
	}

	upload, err := handleAWSFlags(cmd)
	if err != nil {
		return err
	}

	canChown, err := canChownInPath(outputDir)
	if err != nil {
		return err
	}
	if !canChown && chown != "" {
		return fmt.Errorf("chowning is not allowed in output directory")
	}

	manifest_fname := fmt.Sprintf("manifest-%s.json", strings.Join(imgTypes, "-"))
	fmt.Printf("Generating manifest %s\n", manifest_fname)
	mf, err := manifestFromCobra(cmd, args)
	if err != nil {
		panic(err)
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
		return err
	}

	fmt.Printf("Building %s\n", manifest_fname)

	var osbuildEnv []string
	if !canChown {
		// set export options for osbuild
		osbuildEnv = []string{"OSBUILD_EXPORT_FORCE_NO_PRESERVE_OWNER=1"}
	}
	_, err = osbuild.RunOSBuild(mf, osbuildStore, outputDir, exports, nil, osbuildEnv, false, os.Stderr)
	if err != nil {
		return err
	}

	fmt.Println("Build complete!")
	if upload {
		for idx, imgType := range imgTypes {
			switch imgType {
			case "ami":
				diskpath := filepath.Join(outputDir, exports[idx], "disk.raw")
				if err := uploadAMI(diskpath, targetArch, cmd.Flags()); err != nil {
					return err
				}
			default:
				continue
			}
		}
	} else {
		fmt.Printf("Results saved in\n%s\n", outputDir)
	}

	if err := chownR(outputDir, chown); err != nil {
		return err
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
	manifestCmd.Flags().String("config", "", "build config file")
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
