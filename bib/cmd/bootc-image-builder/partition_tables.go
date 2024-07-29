package main

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
)

const (
	MebiByte = 1024 * 1024        // MiB
	GibiByte = 1024 * 1024 * 1024 // GiB
	// BootOptions defines the mountpoint options for /boot
	// See https://github.com/containers/bootc/pull/341 for the rationale for
	// using `ro` by default.  Briefly it protects against corruption
	// by non-ostree aware tools.
	BootOptions = "ro"
	// And we default to `ro` for the rootfs too, because we assume the input
	// container image is using composefs.  For more info, see
	// https://github.com/containers/bootc/pull/417 and
	// https://github.com/ostreedev/ostree/issues/3193
	RootOptions = "ro"
)

var partitionTables = distro.BasePartitionTableMap{
	arch.ARCH_X86_64.String(): disk.PartitionTable{
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     1 * MebiByte,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
			},
			{
				Size: 501 * MebiByte,
				Type: disk.EFISystemPartitionGUID,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "umask=0077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 1 * GibiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: BootOptions,
					FSTabFreq:    1,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 2 * GibiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: RootOptions,
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
		},
	},
	arch.ARCH_AARCH64.String(): disk.PartitionTable{
		Type: "dos",
		Partitions: []disk.Partition{
			{
				Size:     501 * MebiByte,
				Type:     "06",
				Bootable: true,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "umask=0077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 1 * GibiByte,
				Type: "83",
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: BootOptions,
					FSTabFreq:    1,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 2569 * MebiByte,
				Type: "83",
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: RootOptions,
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
		},
	},
	arch.ARCH_S390X.String(): disk.PartitionTable{
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size: 1 * GibiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: BootOptions,
					FSTabFreq:    1,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 2 * GibiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: RootOptions,
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
		},
	},
	arch.ARCH_PPC64LE.String(): disk.PartitionTable{
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     4 * MebiByte,
				Type:     disk.PRePartitionGUID,
				Bootable: true,
			},
			{
				Size: 500 * MebiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: BootOptions,
					FSTabFreq:    1,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 2 * GibiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: RootOptions,
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
		},
	},
}
