package main_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/runner"

	bib "github.com/osbuild/bootc-image-builder/bib/cmd/bootc-image-builder"
	"github.com/osbuild/bootc-image-builder/bib/internal/buildconfig"
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

func TestCheckFilesystemCustomizationsValidates(t *testing.T) {
	for _, tc := range []struct {
		fsCust      []blueprint.FilesystemCustomization
		ptmode      disk.PartitioningMode
		expectedErr string
	}{
		// happy
		{
			fsCust:      []blueprint.FilesystemCustomization{},
			expectedErr: "",
		},
		{
			fsCust:      []blueprint.FilesystemCustomization{},
			ptmode:      disk.BtrfsPartitioningMode,
			expectedErr: "",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"}, {Mountpoint: "/boot"},
			},
			ptmode:      disk.RawPartitioningMode,
			expectedErr: "",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"}, {Mountpoint: "/boot"},
			},
			ptmode:      disk.BtrfsPartitioningMode,
			expectedErr: "",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/boot"},
				{Mountpoint: "/var/log"},
				{Mountpoint: "/var/data"},
			},
			expectedErr: "",
		},
		// sad
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/ostree"},
			},
			ptmode:      disk.RawPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/ostree\" is not allowed",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/var"},
			},
			ptmode:      disk.RawPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/var\" is not allowed",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/var/data"},
			},
			ptmode:      disk.BtrfsPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/var/data\" is not allowed",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/boot/"},
			},
			ptmode:      disk.BtrfsPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/boot/\" must be canonical",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/boot/"},
				{Mountpoint: "/opt"},
			},
			ptmode:      disk.BtrfsPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/boot/\" must be canonical\npath \"/opt\" is not allowed",
		},
	} {
		if tc.expectedErr == "" {
			assert.NoError(t, bib.CheckFilesystemCustomizations(tc.fsCust, tc.ptmode))
		} else {
			assert.ErrorContains(t, bib.CheckFilesystemCustomizations(tc.fsCust, tc.ptmode), tc.expectedErr)
		}
	}
}

func TestLocalMountpointPolicy(t *testing.T) {
	// extended testing of the general mountpoint policy (non-minimal)
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
			err := bib.CheckFilesystemCustomizations([]blueprint.FilesystemCustomization{{Mountpoint: tc.path}}, disk.RawPartitioningMode)
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

func TestUpdateFilesystemSizes(t *testing.T) {
	type testCase struct {
		customizations []blueprint.FilesystemCustomization
		minRootSize    uint64
		expected       []blueprint.FilesystemCustomization
	}

	testCases := map[string]testCase{
		"simple": {
			customizations: nil,
			minRootSize:    999,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    999,
				},
			},
		},
		"container-is-larger": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    10,
				},
			},
			minRootSize: 999,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    999,
				},
			},
		},
		"container-is-smaller": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    1000,
				},
			},
			minRootSize: 892,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    1000,
				},
			},
		},
		"customizations-noroot": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
			},
			minRootSize: 9000,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    9000,
				},
			},
		},
		"customizations-withroot-smallcontainer": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    2_000_000,
				},
			},
			minRootSize: 9000,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    2_000_000,
				},
			},
		},
		"customizations-withroot-largecontainer": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    2_000_000,
				},
			},
			minRootSize: 9_000_000,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    9_000_000,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.ElementsMatch(t, bib.UpdateFilesystemSizes(tc.customizations, tc.minRootSize), tc.expected)
		})
	}

}

func findMountableSizeableFor(pt *disk.PartitionTable, needle string) (disk.Mountable, disk.Sizeable) {
	var foundMnt disk.Mountable
	var foundParent disk.Sizeable
	err := pt.ForEachMountable(func(mnt disk.Mountable, path []disk.Entity) error {
		if mnt.GetMountpoint() == needle {
			foundMnt = mnt
			for idx := len(path) - 1; idx >= 0; idx-- {
				if sz, ok := path[idx].(disk.Sizeable); ok {
					foundParent = sz
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return foundMnt, foundParent
}

func TestGenPartitionTableSetsRootfsForAllFilesystemsXFS(t *testing.T) {
	rng := bib.CreateRand()

	cnf := &bib.ManifestConfig{
		Architecture: arch.ARCH_X86_64,
		RootFSType:   "xfs",
	}
	cus := &blueprint.Customizations{
		Filesystem: []blueprint.FilesystemCustomization{
			{Mountpoint: "/var/data", MinSize: 2_000_000},
			{Mountpoint: "/var/stuff", MinSize: 10_000_000},
		},
	}
	pt, err := bib.GenPartitionTable(cnf, cus, rng)
	assert.NoError(t, err)

	for _, mntPoint := range []string{"/", "/boot", "/var/data"} {
		mnt, _ := findMountableSizeableFor(pt, mntPoint)
		assert.Equal(t, "xfs", mnt.GetFSType())
	}
	_, parent := findMountableSizeableFor(pt, "/var/data")
	assert.True(t, parent.GetSize() >= 2_000_000)

	_, parent = findMountableSizeableFor(pt, "/var/stuff")
	assert.True(t, parent.GetSize() >= 10_000_000)

	// ESP is always vfat
	mnt, _ := findMountableSizeableFor(pt, "/boot/efi")
	assert.Equal(t, "vfat", mnt.GetFSType())
}

func TestGenPartitionTableSetsRootfsForAllFilesystemsBtrfs(t *testing.T) {
	rng := bib.CreateRand()

	cnf := &bib.ManifestConfig{
		Architecture: arch.ARCH_X86_64,
		RootFSType:   "btrfs",
	}
	cus := &blueprint.Customizations{}
	pt, err := bib.GenPartitionTable(cnf, cus, rng)
	assert.NoError(t, err)

	mnt, _ := findMountableSizeableFor(pt, "/")
	assert.Equal(t, "btrfs", mnt.GetFSType())

	// btrfs has a default (ext4) /boot
	mnt, _ = findMountableSizeableFor(pt, "/boot")
	assert.Equal(t, "ext4", mnt.GetFSType())

	// ESP is always vfat
	mnt, _ = findMountableSizeableFor(pt, "/boot/efi")
	assert.Equal(t, "vfat", mnt.GetFSType())
}

func TestGenPartitionTableDiskCustomizationRunsValidateLayoutConstraints(t *testing.T) {
	rng := bib.CreateRand()

	cnf := &bib.ManifestConfig{
		Architecture: arch.ARCH_X86_64,
		RootFSType:   "xfs",
	}
	cus := &blueprint.Customizations{
		Disk: &blueprint.DiskCustomization{
			Partitions: []blueprint.PartitionCustomization{
				{
					Type:            "lvm",
					VGCustomization: blueprint.VGCustomization{},
				},
				{
					Type:            "lvm",
					VGCustomization: blueprint.VGCustomization{},
				},
			},
		},
	}
	_, err := bib.GenPartitionTable(cnf, cus, rng)
	assert.EqualError(t, err, "cannot use disk customization: multiple LVM volume groups are not yet supported")
}

func TestGenPartitionTableDiskCustomizationUnknownTypesError(t *testing.T) {
	cus := &blueprint.Customizations{
		Disk: &blueprint.DiskCustomization{
			Partitions: []blueprint.PartitionCustomization{
				{
					Type: "rando",
				},
			},
		},
	}
	_, err := bib.CalcRequiredDirectorySizes(cus.Disk, 5*datasizes.GiB)
	assert.EqualError(t, err, `unknown disk customization type "rando"`)
}

func TestGenPartitionTableDiskCustomizationSizes(t *testing.T) {
	rng := bib.CreateRand()

	for _, tc := range []struct {
		name                string
		rootfsMinSize       uint64
		partitions          []blueprint.PartitionCustomization
		expectedMinRootSize uint64
	}{
		{
			"empty disk customizaton, root expands to rootfsMinsize",
			2 * datasizes.GiB,
			nil,
			2 * datasizes.GiB,
		},
		// plain
		{
			"plain, no root minsize, expands to rootfsMinSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					MinSize: 10 * datasizes.GiB,
					FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
						Mountpoint: "/var",
						FSType:     "xfs",
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"plain, small root minsize, expands to rootfsMnSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					MinSize: 1 * datasizes.GiB,
					FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
						Mountpoint: "/",
						FSType:     "xfs",
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"plain, big root minsize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					MinSize: 10 * datasizes.GiB,
					FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
						Mountpoint: "/",
						FSType:     "xfs",
					},
				},
			},
			10 * datasizes.GiB,
		},
		// btrfs
		{
			"btrfs, no root minsize, expands to rootfsMinSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "btrfs",
					MinSize: 10 * datasizes.GiB,
					BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
						Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
							{
								Mountpoint: "/var",
								Name:       "varvol",
							},
						},
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"btrfs, small root minsize, expands to rootfsMnSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "btrfs",
					MinSize: 1 * datasizes.GiB,
					BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
						Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
							{
								Mountpoint: "/",
								Name:       "rootvol",
							},
						},
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"btrfs, big root minsize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "btrfs",
					MinSize: 10 * datasizes.GiB,
					BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
						Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
							{
								Mountpoint: "/",
								Name:       "rootvol",
							},
						},
					},
				},
			},
			10 * datasizes.GiB,
		},
		// lvm
		{
			"lvm, no root minsize, expands to rootfsMinSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "lvm",
					MinSize: 10 * datasizes.GiB,
					VGCustomization: blueprint.VGCustomization{
						LogicalVolumes: []blueprint.LVCustomization{
							{
								MinSize: 10 * datasizes.GiB,
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/var",
									FSType:     "xfs",
								},
							},
						},
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"lvm, small root minsize, expands to rootfsMnSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "lvm",
					MinSize: 1 * datasizes.GiB,
					VGCustomization: blueprint.VGCustomization{
						LogicalVolumes: []blueprint.LVCustomization{
							{
								MinSize: 1 * datasizes.GiB,
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/",
									FSType:     "xfs",
								},
							},
						},
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"lvm, big root minsize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "lvm",
					MinSize: 10 * datasizes.GiB,
					VGCustomization: blueprint.VGCustomization{
						LogicalVolumes: []blueprint.LVCustomization{
							{
								MinSize: 10 * datasizes.GiB,
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/",
									FSType:     "xfs",
								},
							},
						},
					},
				},
			},
			10 * datasizes.GiB,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cnf := &bib.ManifestConfig{
				Architecture:  arch.ARCH_X86_64,
				RootFSType:    "xfs",
				RootfsMinsize: tc.rootfsMinSize,
			}
			cus := &blueprint.Customizations{
				Disk: &blueprint.DiskCustomization{
					Partitions: tc.partitions,
				},
			}
			pt, err := bib.GenPartitionTable(cnf, cus, rng)
			assert.NoError(t, err)

			var rootSize uint64
			err = pt.ForEachMountable(func(mnt disk.Mountable, path []disk.Entity) error {
				if mnt.GetMountpoint() == "/" {
					for idx := len(path) - 1; idx >= 0; idx-- {
						if parent, ok := path[idx].(disk.Sizeable); ok {
							rootSize = parent.GetSize()
							break
						}
					}
				}
				return nil
			})
			assert.NoError(t, err)
			// expected size is within a reasonable limit
			assert.True(t, rootSize >= tc.expectedMinRootSize && rootSize < tc.expectedMinRootSize+5*datasizes.MiB)
		})
	}
}

func TestManifestFilecustomizationsSad(t *testing.T) {
	config := getBaseConfig()
	config.ImageTypes = []string{"qcow2"}
	config.Config = &buildconfig.BuildConfig{
		Customizations: &blueprint.Customizations{
			Files: []blueprint.FileCustomization{
				{
					Path: "/not/allowed",
					Data: "some-data",
				},
			},
		},
	}

	_, err := bib.Manifest(config)
	assert.EqualError(t, err, `the following custom files are not allowed: ["/not/allowed"]`)
}

func TestManifestDirCustomizationsSad(t *testing.T) {
	config := getBaseConfig()
	config.ImageTypes = []string{"qcow2"}
	config.Config = &buildconfig.BuildConfig{
		Customizations: &blueprint.Customizations{
			Directories: []blueprint.DirectoryCustomization{
				{
					Path: "/dir/not/allowed",
				},
			},
		},
	}

	_, err := bib.Manifest(config)
	assert.EqualError(t, err, `the following custom directories are not allowed: ["/dir/not/allowed"]`)
}
