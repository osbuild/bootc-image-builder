package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
)

//go:embed fedora-eln.json
var reposStr string

const (
	distroName       = "fedora-39"
	modulePlatformID = "platform:f39"
	releaseVersion   = "39"
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
func loadRepos(archName string) []rpmmd.RepoConfig {
	var repoData map[string][]rpmmd.RepoConfig
	err := json.Unmarshal([]byte(reposStr), &repoData)
	if err != nil {
		log.Fatalf("error loading repositories: %s", err)
	}
	archRepos, ok := repoData[archName]
	if !ok {
		log.Fatalf("no repositories defined for %s", archName)
	}
	return archRepos
}

func loadConfig(path string) BuildConfig {
	fp, err := os.Open(path)
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer fp.Close()

	dec := json.NewDecoder(fp)
	dec.DisallowUnknownFields()
	var conf BuildConfig

	if err := dec.Decode(&conf); err != nil {
		log.Fatalf("%s", err)
	}
	if dec.More() {
		log.Fatalf("multiple configuration objects or extra data found in %q", path)
	}
	return conf
}

func makeManifest(c *ManifestConfig, cacheRoot string) (manifest.OSBuildManifest, error) {
	manifest, err := Manifest(c)
	if err != nil {
		log.Fatalf("%s", err)
	}

	// depsolve packages
	solver := dnfjson.NewSolver(modulePlatformID, releaseVersion, c.Architecture.String(), distroName, cacheRoot)
	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	for name, pkgSet := range manifest.GetPackageSetChains() {
		res, err := solver.Depsolve(pkgSet)
		if err != nil {
			return nil, err
		}
		depsolvedSets[name] = res
	}

	// resolve container
	resolver := container.NewResolver(c.Architecture.String())
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

	mf, err := manifest.Serialize(depsolvedSets, containerSpecs, nil)
	if err != nil {
		log.Fatalf("[ERROR] manifest serialization failed: %s", err.Error())
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
	hostArch := arch.Current()
	repos := loadRepos(hostArch.String())

	imgref := args[0]
	rpmCacheRoot, _ := cmd.Flags().GetString("rpmmd")
	configFile, _ := cmd.Flags().GetString("config")
	tlsVerify, _ := cmd.Flags().GetBool("tls-verify")
	imgType, _ := cmd.Flags().GetString("type")

	config := BuildConfig{}
	if configFile != "" {
		config = loadConfig(configFile)
	}

	manifestConfig := &ManifestConfig{
		Imgref:       imgref,
		ImgType:      imgType,
		Config:       &config,
		Repos:        repos,
		Architecture: hostArch,
		TLSVerify:    tlsVerify,
	}
	return makeManifest(manifestConfig, rpmCacheRoot)
}

func cmdManifest(cmd *cobra.Command, args []string) {
	mf, err := manifestFromCobra(cmd, args)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(mf))
}

func cmdBuild(cmd *cobra.Command, args []string) {
	outputDir, _ := cmd.Flags().GetString("output")
	osbuildStore, _ := cmd.Flags().GetString("store")
	imgType, _ := cmd.Flags().GetString("type")

	if err := setup.Validate(); err != nil {
		log.Fatalf("%s", err)
	}
	if err := setup.EnsureEnvironment(); err != nil {
		log.Fatalf("%s", err)
	}

	if err := os.MkdirAll(outputDir, 0777); err != nil {
		log.Fatalf("failed to create target directory: %s", err.Error())
	}

	upload := false
	if region, _ := cmd.Flags().GetString("aws-region"); region != "" {
		if imgType != "ami" {
			log.Fatalf("aws flags set for non-ami image type (type is set to %s)", imgType)
		}
		// initialise the client to check if the env vars exist before building the image
		client, err := awscloud.NewDefault(region)
		if err != nil {
			log.Fatalf("%s", err)
		}

		fmt.Printf("Checking AWS permission by listing regions...\n")
		if _, err := client.Regions(); err != nil {
			log.Fatalf("%s", err)
		}
		upload = true
	}

	canChown, err := canChownInPath(outputDir)
	if err != nil {
		log.Fatalf("%s", err)
	}

	manifest_fname := fmt.Sprintf("manifest-%s.json", imgType)
	fmt.Printf("Generating %s ... ", manifest_fname)
	mf, err := manifestFromCobra(cmd, args)
	if err != nil {
		panic(err)
	}
	fmt.Print("DONE\n")

	var exports []string
	switch imgType {
	case "qcow2":
		exports = []string{"qcow2"}
	case "ami", "raw":
		exports = []string{"image"}
	case "iso":
		exports = []string{"bootiso"}
	default:
		log.Fatalf("valid types are 'qcow2', 'ami', 'raw', 'iso', not: '%s'", imgType)
	}

	manifestPath := filepath.Join(outputDir, manifest_fname)
	if err := saveManifest(mf, manifestPath); err != nil {
		log.Fatalf("%s", err)
	}

	fmt.Printf("Building %s\n", manifest_fname)

	var osbuildEnv []string
	if !canChown {
		// set export options for osbuild
		osbuildEnv = []string{"OSBUILD_EXPORT_FORCE_NO_PRESERVE_OWNER=1"}
	}
	_, err = osbuild.RunOSBuild(mf, osbuildStore, outputDir, exports, nil, osbuildEnv, false, os.Stderr)
	if err != nil {
		log.Fatalf("%s", err)
	}

	fmt.Println("Build complete!")
	if upload {
		switch imgType {
		case "ami":
			diskpath := filepath.Join(outputDir, exports[0], "disk.raw")
			if err := uploadAMI(diskpath, cmd.Flags()); err != nil {
				log.Fatalf("%s", err)
			}
		default:
			log.Panicf("upload set but image type %s doesn't support uploading", imgType)
		}
	} else {
		fmt.Printf("Results saved in\n%s\n", outputDir)
	}

}

func main() {
	rootCmd := &cobra.Command{
		Use:  "bootc-image-builder",
		Long: "create a bootable image from an ostree native container",
	}

	buildCmd := &cobra.Command{
		Use:                   "build",
		Long:                  rootCmd.Long,
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		Run:                   cmdBuild,
	}
	rootCmd.AddCommand(buildCmd)
	manifestCmd := &cobra.Command{
		Use:                   "manifest",
		Long:                  rootCmd.Long,
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		Run:                   cmdManifest,
	}
	rootCmd.AddCommand(manifestCmd)
	manifestCmd.Flags().String("rpmmd", "/var/cache/osbuild/rpmmd", "rpm metadata cache directory")
	manifestCmd.Flags().String("config", "", "build config file")
	manifestCmd.Flags().String("type", "qcow2", "image type to build [qcow2, ami]")
	manifestCmd.Flags().Bool("tls-verify", true, "require HTTPS and verify certificates when contacting registries")

	logrus.SetLevel(logrus.ErrorLevel)
	buildCmd.Flags().AddFlagSet(manifestCmd.Flags())
	buildCmd.Flags().String("output", ".", "artifact output directory")
	buildCmd.Flags().String("store", ".osbuild", "osbuild store for intermediate pipeline trees")
	buildCmd.Flags().String("aws-region", "", "target region for AWS uploads (only for type=ami)")
	buildCmd.Flags().String("aws-bucket", "", "target S3 bucket name for intermediate storage when creating AMI (only for type=ami)")
	buildCmd.Flags().String("aws-ami-name", "", "name for the AMI in AWS (only for type=ami)")

	// flag rules
	for _, dname := range []string{"output", "store", "rpmmd"} {
		if err := buildCmd.MarkFlagDirname(dname); err != nil {
			log.Fatalf("%s", err)
		}
	}
	if err := buildCmd.MarkFlagFilename("config"); err != nil {
		log.Fatalf("%s", err)
	}
	buildCmd.MarkFlagsRequiredTogether("aws-region", "aws-bucket", "aws-ami-name")

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("%s", err)
	}
}
