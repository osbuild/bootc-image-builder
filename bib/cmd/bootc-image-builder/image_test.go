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

func TestLocalMountpointPolicy(t *testing.T) {
	type testCase struct {
		path    string
		allowed bool
	}

	testCases := []testCase{
		// existing mountpoints / and /boot are fine for sizing
		{"/", true},
		{"/boot", true},

		// root mountpoints are not allowed
		{"/data", false},
		{"/opt", false},
		{"/stuff", false},
		{"/usr", false},

		// /var explicitly is not allowed
		{"/var", false},

		// subdirs of /boot are not allowed
		{"/boot/stuff", false},
		{"/boot/loader", false},

		// /var subdirectories are allowed
		{"/var/data", true},
		{"/var/scratch", true},
		{"/var/log", true},
		{"/var/opt", true},
		{"/var/opt/application", true},

		// but not these
		{"/var/home", false},
		{"/var/lock", false}, // symlink to ../run/lock which is on tmpfs
		{"/var/mail", false}, // symlink to spool/mail
		{"/var/mnt", false},
		{"/var/roothome", false},
		{"/var/run", false}, // symlink to ../run which is on tmpfs
		{"/var/srv", false},
		{"/var/usrlocal", false},

		// nor their subdirs
		{"/var/run/subrun", false},
		{"/var/srv/test", false},
		{"/var/home/user", false},
		{"/var/usrlocal/bin", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			err := bib.CheckMountpoints([]blueprint.FilesystemCustomization{{Mountpoint: tc.path}})
			if err != nil && tc.allowed {
				t.Errorf("expected %s to be allowed, but got error: %v", tc.path, err)
			} else if err == nil && !tc.allowed {
				t.Errorf("expected %s to be denied, but got no error", tc.path)
			}
		})
	}
}

func TestBasePartitionTablesHaveRoot(t *testing.T) {
	// make sure that all base partition tables have at least a root partition defined
	for arch, pt := range bib.PartitionTables {
		rootMountable := pt.FindMountable("/")
		if rootMountable == nil {
			t.Errorf("partition table %q does not define a root filesystem", arch)
		}
		_, isFS := rootMountable.(*disk.Filesystem)
		if !isFS {
			t.Errorf("root mountable for %q is not an ordinary filesystem", arch)
		}
	}

}
