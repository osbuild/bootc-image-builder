package main_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/osbuild/bootc-image-builder/bib/cmd/bootc-image-builder"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/rpmmd"
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
	config     *main.ManifestConfig
	imageType  string
	packages   map[string][]rpmmd.PackageSpec
	containers map[string][]container.Spec
	err        error
}

func TestManifestGenerationEmptyConfig(t *testing.T) {
	baseConfig := &main.ManifestConfig{Imgref: "testempty"}
	testCases := map[string]manifestTestCase{
		"ami-base": {
			config:    baseConfig,
			imageType: "ami",
		},
		"raw-base": {
			config:    baseConfig,
			imageType: "raw",
		},
		"qcow2-base": {
			config:    baseConfig,
			imageType: "qcow2",
		},
		"iso-base": {
			config:    baseConfig,
			imageType: "iso",
		},
		"empty-config": {
			config:    &main.ManifestConfig{},
			imageType: "qcow2",
			err:       errors.New("pipeline: no base image defined"),
		},
		"bad-image-type": {
			config:    baseConfig,
			imageType: "bad",
			err:       errors.New("Manifest(): unsupported image type \"bad\""),
		},
	}

	assert := assert.New(t)
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := main.ManifestConfig(*tc.config)
			config.ImgType = tc.imageType
			_, err := main.Manifest(&config)
			assert.Equal(err, tc.err)
		})
	}
}

func TestManifestGenerationUserConfig(t *testing.T) {
	// add a user
	pass := "super-secret-password-42"
	key := "ssh-ed25519 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	userConfig := &main.ManifestConfig{
		Imgref:  "testuser",
		ImgType: "",
		Config: &main.BuildConfig{
			Blueprint: &blueprint.Blueprint{
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
		},
	}

	testCases := map[string]manifestTestCase{
		"ami-user": {
			config:    userConfig,
			imageType: "ami",
		},
		"raw-user": {
			config:    userConfig,
			imageType: "raw",
		},
		"qcow2-user": {
			config:    userConfig,
			imageType: "qcow2",
		},
		"iso-user": {
			config:    userConfig,
			imageType: "iso",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := main.ManifestConfig(*tc.config)
			config.ImgType = tc.imageType
			_, err := main.Manifest(&config)

			assert := assert.New(t)
			assert.NoError(err)
		})
	}
}
