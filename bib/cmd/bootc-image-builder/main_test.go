package main_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/rpmmd"

	main "github.com/osbuild/bootc-image-builder/bib/cmd/bootc-image-builder"
	"github.com/osbuild/bootc-image-builder/bib/internal/buildconfig"
	"github.com/osbuild/bootc-image-builder/bib/internal/imagetypes"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
)

func TestCanChownInPathHappy(t *testing.T) {
	tmpdir := t.TempDir()
	canChown, err := main.CanChownInPath(tmpdir)
	require.Nil(t, err)
	assert.Equal(t, canChown, true)

	// no tmpfile leftover
	content, err := os.ReadDir(tmpdir)
	require.Nil(t, err)
	assert.Equal(t, len(content), 0)
}

func TestCanChownInPathNotExists(t *testing.T) {
	canChown, err := main.CanChownInPath("/does/not/exists")
	assert.Equal(t, canChown, false)
	assert.ErrorContains(t, err, ": no such file or directory")
}

func TestCanChownInPathCannotChange(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot run as root (fchown never errors here)")
	}

	restore := main.MockOsGetuid(func() int {
		return -2
	})
	defer restore()

	tmpdir := t.TempDir()
	canChown, err := main.CanChownInPath(tmpdir)
	require.Nil(t, err)
	assert.Equal(t, canChown, false)
}

type manifestTestCase struct {
	config            *main.ManifestConfig
	imageTypes        imagetypes.ImageTypes
	depsolved         map[string]dnfjson.DepsolveResult
	containers        map[string][]container.Spec
	expStages         map[string][]string
	notExpectedStages map[string][]string
	err               interface{}
}

func getBaseConfig() *main.ManifestConfig {
	return &main.ManifestConfig{
		Architecture: arch.ARCH_X86_64,
		Imgref:       "testempty",
		SourceInfo: &source.Info{
			OSRelease: source.OSRelease{
				ID:         "fedora",
				VersionID:  "40",
				Name:       "Fedora Linux",
				PlatformID: "platform:f40",
			},
			UEFIVendor: "fedora",
		},

		// We need the real path here, because we are creating real manifests
		DistroDefPaths: []string{"../../data/defs"},

		// RootFSType is required to create a Manifest
		RootFSType: "ext4",
	}
}

func getUserConfig() *main.ManifestConfig {
	// add a user
	pass := "super-secret-password-42"
	key := "ssh-ed25519 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	return &main.ManifestConfig{
		Architecture: arch.ARCH_X86_64,
		Imgref:       "testuser",
		Config: &buildconfig.BuildConfig{
			Customizations: &blueprint.Customizations{
				User: []blueprint.UserCustomization{
					{
						Name:     "tester",
						Password: &pass,
						Key:      &key,
					},
				},
			},
		},
		SourceInfo: &source.Info{
			OSRelease: source.OSRelease{
				ID:         "fedora",
				VersionID:  "40",
				Name:       "Fedora Linux",
				PlatformID: "platform:f40",
			},
			UEFIVendor: "fedora",
		},

		// We need the real path here, because we are creating real manifests
		DistroDefPaths: []string{"../../data/defs"},

		// RootFSType is required to create a Manifest
		RootFSType: "ext4",
	}
}

func TestManifestGenerationEmptyConfig(t *testing.T) {
	baseConfig := getBaseConfig()
	testCases := map[string]manifestTestCase{
		"ami-base": {
			config:     baseConfig,
			imageTypes: []string{"ami"},
		},
		"raw-base": {
			config:     baseConfig,
			imageTypes: []string{"raw"},
		},
		"qcow2-base": {
			config:     baseConfig,
			imageTypes: []string{"qcow2"},
		},
		"iso-base": {
			config:     baseConfig,
			imageTypes: []string{"iso"},
		},
		"empty-config": {
			config:     &main.ManifestConfig{},
			imageTypes: []string{"qcow2"},
			err:        errors.New("pipeline: no base image defined"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := main.ManifestConfig(*tc.config)
			config.ImageTypes = tc.imageTypes
			_, err := main.Manifest(&config)
			assert.Equal(t, err, tc.err)
		})
	}
}

func TestManifestGenerationUserConfig(t *testing.T) {
	userConfig := getUserConfig()
	testCases := map[string]manifestTestCase{
		"ami-user": {
			config:     userConfig,
			imageTypes: []string{"ami"},
		},
		"raw-user": {
			config:     userConfig,
			imageTypes: []string{"raw"},
		},
		"qcow2-user": {
			config:     userConfig,
			imageTypes: []string{"qcow2"},
		},
		"iso-user": {
			config:     userConfig,
			imageTypes: []string{"iso"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := main.ManifestConfig(*tc.config)
			config.ImageTypes = tc.imageTypes
			_, err := main.Manifest(&config)
			assert.NoError(t, err)
		})
	}
}

// Disk images require a container for the build/image pipelines
var containerSpec = container.Spec{
	Source:  "test-container",
	Digest:  "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	ImageID: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
}

// diskContainers can be passed to Serialize() to get a minimal disk image
var diskContainers = map[string][]container.Spec{
	"build": {
		containerSpec,
	},
	"image": {
		containerSpec,
	},
	"target": {
		containerSpec,
	},
}

// TODO: this tests at this layer is not ideal, it has too much knowledge
// over the implementation details of the "images" library and how an
// image.NewBootcDiskImage() works (i.e. what the pipeline names are and
// what key piplines to expect). These details should be tested in "images"
// and here we would just check (somehow) that image.NewBootcDiskImage()
// (or image.NewAnacondaContainerInstaller()) is called and the right
// customizations are passed. The existing layout makes this hard so this
// is fine for now but would be nice to revisit this.
func TestManifestSerialization(t *testing.T) {
	// Tests that the manifest is generated without error and is serialized
	// with expected key stages.

	// ISOs require a container for the bootiso-tree, build packages, and packages for the anaconda-tree (with a kernel).
	var isoContainers = map[string][]container.Spec{
		"bootiso-tree": {
			containerSpec,
		},
	}
	isoPackages := map[string]dnfjson.DepsolveResult{
		"build": {
			Packages: []rpmmd.PackageSpec{
				{
					Name:     "package",
					Version:  "113",
					Checksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
			},
		},
		"anaconda-tree": {
			Packages: []rpmmd.PackageSpec{
				{
					Name:     "kernel",
					Version:  "10.11",
					Checksum: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				},
				{
					Name:     "package",
					Version:  "113",
					Checksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
			},
		},
	}

	pkgsNoBuild := map[string]dnfjson.DepsolveResult{
		"anaconda-tree": {
			Packages: []rpmmd.PackageSpec{

				{
					Name:     "kernel",
					Version:  "10.11",
					Checksum: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				},
				{
					Name:     "package",
					Version:  "113",
					Checksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
			},
		},
	}

	baseConfig := getBaseConfig()
	userConfig := getUserConfig()
	testCases := map[string]manifestTestCase{
		"ami-base": {
			config:     baseConfig,
			imageTypes: []string{"ami"},
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
				"image": {
					"org.osbuild.users",
				},
			},
		},
		"raw-base": {
			config:     baseConfig,
			imageTypes: []string{"raw"},
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
				"image": {
					"org.osbuild.users",
				},
			},
		},
		"qcow2-base": {
			config:     baseConfig,
			imageTypes: []string{"qcow2"},
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
				"image": {
					"org.osbuild.users",
				},
			},
		},
		"ami-user": {
			config:     userConfig,
			imageTypes: []string{"ami"},
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.users",
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
			},
		},
		"raw-user": {
			config:     userConfig,
			imageTypes: []string{"raw"},
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.users", // user creation stage when we add users
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
			},
		},
		"qcow2-user": {
			config:     userConfig,
			imageTypes: []string{"qcow2"},
			containers: diskContainers,
			expStages: map[string][]string{
				"build": {"org.osbuild.container-deploy"},
				"image": {
					"org.osbuild.users", // user creation stage when we add users
					"org.osbuild.bootc.install-to-filesystem",
				},
			},
			notExpectedStages: map[string][]string{
				"build": {"org.osbuild.rpm"},
			},
		},
		"iso-user": {
			config:     userConfig,
			imageTypes: []string{"iso"},
			containers: isoContainers,
			depsolved:  isoPackages,
			expStages: map[string][]string{
				"build":        {"org.osbuild.rpm"},
				"bootiso-tree": {"org.osbuild.skopeo"}, // adds the container to the ISO tree
			},
		},
		"iso-nobuildpkg": {
			config:     userConfig,
			imageTypes: []string{"iso"},
			containers: isoContainers,
			depsolved:  pkgsNoBuild,
			err:        "serialization not started",
		},
		"iso-nocontainer": {
			config:     userConfig,
			imageTypes: []string{"iso"},
			depsolved:  isoPackages,
			err:        "missing ostree, container, or ospipeline parameters in ISO tree pipeline",
		},
		"ami-nocontainer": {
			config:     userConfig,
			imageTypes: []string{"ami"},
			// errors come from BuildrootFromContainer()
			// TODO: think about better error and testing here (not the ideal layer or err msg)
			err: "serialization not started",
		},
		"raw-nocontainer": {
			config:     userConfig,
			imageTypes: []string{"raw"},
			// errors come from BuildrootFromContainer()
			// TODO: think about better error and testing here (not the ideal layer or err msg)
			err: "serialization not started",
		},
		"qcow2-nocontainer": {
			config:     userConfig,
			imageTypes: []string{"qcow2"},
			// errors come from BuildrootFromContainer()
			// TODO: think about better error and testing here (not the ideal layer or err msg)
			err: "serialization not started",
		},
	}

	// Use an empty config: only the imgref is required
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			config := main.ManifestConfig(*tc.config)
			config.ImageTypes = tc.imageTypes
			mf, err := main.Manifest(&config)
			assert.NoError(err) // this isn't the error we're testing for

			if tc.err != nil {
				assert.PanicsWithValue(tc.err, func() {
					_, err := mf.Serialize(tc.depsolved, tc.containers, nil, nil)
					assert.NoError(err)
				})
			} else {
				manifestJson, err := mf.Serialize(tc.depsolved, tc.containers, nil, nil)
				assert.NoError(err)
				assert.NoError(checkStages(manifestJson, tc.expStages, tc.notExpectedStages))
			}
		})
	}

	{
		// this one panics with a typed error and needs to be tested separately from the above (PanicsWithError())
		t.Run("iso-nopkgs", func(t *testing.T) {
			assert := assert.New(t)
			config := main.ManifestConfig(*userConfig)
			config.ImageTypes, _ = imagetypes.New("iso")
			manifest, err := main.Manifest(&config)
			assert.NoError(err) // this isn't the error we're testing for

			expError := "package \"kernel\" not found in the PackageSpec list"
			assert.PanicsWithError(expError, func() {
				_, err := manifest.Serialize(nil, isoContainers, nil, nil)
				assert.NoError(err)
			})
		})
	}
}

// simplified representation of a manifest
type testManifest struct {
	Pipelines []pipeline `json:"pipelines"`
}
type pipeline struct {
	Name   string  `json:"name"`
	Stages []stage `json:"stages"`
}
type stage struct {
	Type string `json:"type"`
}

func checkStages(serialized manifest.OSBuildManifest, pipelineStages map[string][]string, missingStages map[string][]string) error {
	mf := &testManifest{}
	if err := json.Unmarshal(serialized, mf); err != nil {
		return err
	}
	pipelineMap := map[string]pipeline{}
	for _, pl := range mf.Pipelines {
		pipelineMap[pl.Name] = pl
	}

	for plname, stages := range pipelineStages {
		pl, found := pipelineMap[plname]
		if !found {
			return fmt.Errorf("pipeline %q not found", plname)
		}

		stageMap := map[string]bool{}
		for _, stage := range pl.Stages {
			stageMap[stage.Type] = true
		}
		for _, stage := range stages {
			if _, found := stageMap[stage]; !found {
				return fmt.Errorf("pipeline %q - stage %q - not found", plname, stage)
			}
		}
	}

	for plname, stages := range missingStages {
		pl, found := pipelineMap[plname]
		if !found {
			return fmt.Errorf("pipeline %q not found", plname)
		}

		stageMap := map[string]bool{}
		for _, stage := range pl.Stages {
			stageMap[stage.Type] = true
		}
		for _, stage := range stages {
			if _, found := stageMap[stage]; found {
				return fmt.Errorf("pipeline %q - stage %q - found (but should not be)", plname, stage)
			}
		}
	}

	return nil
}

func mockOsArgs(new []string) (restore func()) {
	saved := os.Args
	os.Args = append([]string{"argv0"}, new...)
	return func() {
		os.Args = saved
	}
}

func addRunLog(rootCmd *cobra.Command, runeCall *string) {
	for _, cmd := range rootCmd.Commands() {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			callStr := fmt.Sprintf("<%v>: %v", cmd.Name(), strings.Join(args, ","))
			if *runeCall != "" {
				panic(fmt.Sprintf("runE called with %v but already called before: %v", callStr, *runeCall))
			}
			*runeCall = callStr
			return nil
		}
	}
}

func TestCobraCmdline(t *testing.T) {
	for _, tc := range []struct {
		cmdline      []string
		expectedCall string
	}{
		// trivial: cmd is given explicitly
		{
			[]string{"manifest", "quay.io..."},
			"<manifest>: quay.io...",
		},
		{
			[]string{"build", "quay.io..."},
			"<build>: quay.io...",
		},
		{
			[]string{"version", "quay.io..."},
			"<version>: quay.io...",
		},
		// implicit: no cmd like build/manifest defaults to build
		{
			[]string{"--local", "quay.io..."},
			"<build>: quay.io...",
		},
		{
			[]string{"quay.io..."},
			"<build>: quay.io...",
		},
	} {
		var runeCall string

		restore := mockOsArgs(tc.cmdline)
		defer restore()

		rootCmd, err := main.BuildCobraCmdline()
		assert.NoError(t, err)
		addRunLog(rootCmd, &runeCall)

		t.Run(tc.expectedCall, func(t *testing.T) {
			err = rootCmd.Execute()
			assert.NoError(t, err)
			assert.Equal(t, runeCall, tc.expectedCall)
		})
	}
}

func TestCobraCmdlineVerbose(t *testing.T) {
	for _, tc := range []struct {
		cmdline             []string
		expectedProgress    string
		expectedLogrusLevel logrus.Level
	}{
		{
			[]string{"quay.io..."},
			"auto",
			logrus.ErrorLevel,
		},
		{
			[]string{"-v", "quay.io..."},
			"verbose",
			logrus.InfoLevel,
		},
	} {
		restore := mockOsArgs(tc.cmdline)
		defer restore()

		rootCmd, err := main.BuildCobraCmdline()
		assert.NoError(t, err)

		// collect progressFlag value
		var progressFlag string
		for _, cmd := range rootCmd.Commands() {
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				if progressFlag != "" {
					t.Error("progressFlag set twice")
				}
				progressFlag, err = cmd.Flags().GetString("progress")
				assert.NoError(t, err)
				return nil
			}
		}

		t.Run(tc.expectedProgress, func(t *testing.T) {
			err = rootCmd.Execute()
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedProgress, progressFlag)
			assert.Equal(t, tc.expectedLogrusLevel, logrus.GetLevel())
		})
	}
}
