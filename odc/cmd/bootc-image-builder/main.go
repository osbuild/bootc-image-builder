package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/spf13/cobra"
)

//go:embed fedora-eln.json
var reposStr string

const (
	distroName       = "fedora-39"
	modulePlatformID = "platform:f39"
	releaseVersion   = "39"
)

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}

type BuildConfig struct {
	Name      string               `json:"name"`
	OSTree    *ostree.ImageOptions `json:"ostree,omitempty"`
	Blueprint *blueprint.Blueprint `json:"blueprint,omitempty"`
	Depends   interface{}          `json:"depends,omitempty"` // ignored
}

// Parse embedded repositories and return repo configs for the given
// architecture.
func loadRepos(archName string) []rpmmd.RepoConfig {
	var repoData map[string][]rpmmd.RepoConfig
	err := json.Unmarshal([]byte(reposStr), &repoData)
	if err != nil {
		fail(fmt.Sprintf("error loading repositories: %s", err))
	}
	archRepos, ok := repoData[archName]
	if !ok {
		fail(fmt.Sprintf("no repositories defined for %s", archName))
	}
	return archRepos
}

func loadConfig(path string) BuildConfig {
	fp, err := os.Open(path)
	check(err)
	defer fp.Close()

	dec := json.NewDecoder(fp)
	dec.DisallowUnknownFields()
	var conf BuildConfig

	check(dec.Decode(&conf))
	if dec.More() {
		fail(fmt.Sprintf("multiple configuration objects or extra data found in %q", path))
	}
	return conf
}

func makeManifest(imgref string, config *BuildConfig, repos []rpmmd.RepoConfig, architecture arch.Arch, seedArg int64, cacheRoot string) (manifest.OSBuildManifest, error) {
	manifest, err := Manifest(imgref, config, repos, architecture, seedArg)
	check(err)

	// depsolve packages
	solver := dnfjson.NewSolver(modulePlatformID, releaseVersion, architecture.String(), distroName, cacheRoot)
	solver.SetDNFJSONPath("/usr/libexec/osbuild-depsolve-dnf")
	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	for name, pkgSet := range manifest.GetPackageSetChains() {
		res, err := solver.Depsolve(pkgSet)
		if err != nil {
			return nil, err
		}
		depsolvedSets[name] = res
	}

	// resolve container
	resolver := container.NewResolver(architecture.String())
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
		fail(fmt.Sprintf("[ERROR] manifest serialization failed: %s", err.Error()))
	}
	return mf, nil
}

func saveManifest(ms manifest.OSBuildManifest, fpath string) error {
	b, err := json.MarshalIndent(ms, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data for %q: %s\n", fpath, err.Error())
	}
	b = append(b, '\n') // add new line at end of file
	fp, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %s\n", fpath, err.Error())
	}
	defer fp.Close()
	if _, err := fp.Write(b); err != nil {
		return fmt.Errorf("failed to write output file %q: %s\n", fpath, err.Error())
	}
	return nil
}

func build(cmd *cobra.Command, args []string) {
	hostArch := arch.Current()
	repos := loadRepos(hostArch.String())

	imgref := args[0]
	outputDir, _ := cmd.Flags().GetString("output")
	osbuildStore, _ := cmd.Flags().GetString("store")
	rpmCacheRoot, _ := cmd.Flags().GetString("rpmmd")
	configFile, _ := cmd.Flags().GetString("config")

	if err := os.MkdirAll(outputDir, 0777); err != nil {
		fail(fmt.Sprintf("failed to create target directory: %s", err.Error()))
	}

	config := BuildConfig{
		Name: "empty",
	}
	if configFile != "" {
		config = loadConfig(configFile)
	}

	seedArg := int64(0)

	fmt.Printf("Generating manifest for %s: ", config.Name)
	mf, err := makeManifest(imgref, &config, repos, hostArch, seedArg, rpmCacheRoot)
	check(err)
	fmt.Print("DONE\n")

	manifestPath := filepath.Join(outputDir, "manifest.json")
	check(saveManifest(mf, manifestPath))

	fmt.Printf("Building manifest: %s\n", manifestPath)

	_, err = osbuild.RunOSBuild(mf, osbuildStore, outputDir, []string{"qcow2"}, nil, nil, false, os.Stderr)
	check(err)

	fmt.Printf("Build complete. Results saved in\n%s\n", outputDir)
}

func main() {
	rootCmd := &cobra.Command{
		Use:                   "bootc-image-builder <imgref>",
		Long:                  "create a bootable image from an ostree native container",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		Run:                   build,
	}

	rootCmd.Flags().String("output", ".", "artifact output directory")
	rootCmd.Flags().String("store", ".osbuild", "osbuild store for intermediate pipeline trees")
	rootCmd.Flags().String("rpmmd", "/var/cache/osbuild/rpmmd", "rpm metadata cache directory")
	rootCmd.Flags().String("config", "", "build config file")
	check(rootCmd.Execute())
}
