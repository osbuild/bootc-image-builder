package main_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/runner"

	bib "github.com/osbuild/bootc-image-builder/bib/cmd/bootc-image-builder"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
)

func TestGetDistroAndRunner(t *testing.T) {
	cases := []struct {
		id             string
		versionID      string
		expectedDistro manifest.Distro
		expectedRunner runner.Runner
		expectedErr    string
	}{
		// Happy
		{"fedora", "40", manifest.DISTRO_FEDORA, &runner.Fedora{Version: 40}, ""},
		{"centos", "9", manifest.DISTRO_EL9, &runner.CentOS{Version: 9}, ""},
		{"centos", "10", manifest.DISTRO_EL10, &runner.CentOS{Version: 10}, ""},
		{"centos", "11", manifest.DISTRO_NULL, &runner.CentOS{Version: 11}, ""},
		{"rhel", "9.4", manifest.DISTRO_EL9, &runner.RHEL{Major: 9, Minor: 4}, ""},
		{"rhel", "10.4", manifest.DISTRO_EL10, &runner.RHEL{Major: 10, Minor: 4}, ""},
		{"rhel", "11.4", manifest.DISTRO_NULL, &runner.RHEL{Major: 11, Minor: 4}, ""},
		{"toucanos", "42", manifest.DISTRO_NULL, &runner.Linux{}, ""},

		// Sad
		{"fedora", "asdf", manifest.DISTRO_NULL, nil, "cannot parse Fedora version (asdf)"},
		{"centos", "asdf", manifest.DISTRO_NULL, nil, "cannot parse CentOS version (asdf)"},
		{"rhel", "10", manifest.DISTRO_NULL, nil, "invalid RHEL version format: 10"},
		{"rhel", "10.asdf", manifest.DISTRO_NULL, nil, "cannot parse RHEL minor version (asdf)"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%s-%s", c.id, c.versionID), func(t *testing.T) {
			osRelease := source.OSRelease{
				ID:        c.id,
				VersionID: c.versionID,
			}
			distro, runner, err := bib.GetDistroAndRunner(osRelease)
			if c.expectedErr != "" {
				assert.ErrorContains(t, err, c.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, c.expectedDistro, distro)
				assert.Equal(t, c.expectedRunner, runner)
			}
		})
	}
}

func TestApplyFilesystemCustomizationsValidates(t *testing.T) {
	for _, tc := range []struct {
		fsCust      []blueprint.FilesystemCustomization
		expectedErr string
	}{
		// happy
		{
			fsCust:      []blueprint.FilesystemCustomization{},
			expectedErr: "",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"}, {Mountpoint: "/boot"},
			},
			expectedErr: "",
		},
		// sad
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/ostree"},
			},
			expectedErr: `The following custom mountpoints are not supported ["/ostree"]`,
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/var/log"},
			},
			expectedErr: "cannot use custom mount points yet, found: /var/log",
		},
	} {
		conf := &bib.ManifestConfig{}
		cust := &blueprint.Customizations{
			Filesystem: tc.fsCust,
		}
		if tc.expectedErr == "" {
			assert.NoError(t, bib.ApplyFilesystemCustomizations(cust, conf))
		} else {
			assert.ErrorContains(t, bib.ApplyFilesystemCustomizations(cust, conf), tc.expectedErr)
		}
	}
}
