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

// diskUuidOfUnknownOrigin is used by default for disk images,
// picked by someone in the past for unknown reasons. More in
// e.g. https://github.com/osbuild/bootc-image-builder/pull/568 and
// https://github.com/osbuild/images/pull/823
const diskUuidOfUnknownOrigin = "D209C89E-EA5E-4FBD-B161-B461CCE297E0"

// efiPartition defines the default ESP. See also
// https://en.wikipedia.org/wiki/EFI_system_partition
var efiPartition = disk.Partition{
	Size: 501 * MebiByte,
	Type: disk.EFISystemPartitionGUID,
	UUID: disk.EFISystemPartitionUUID,
	Payload: &disk.Filesystem{
		Type:         "vfat",
		UUID:         disk.EFIFilesystemUUID,
		Mountpoint:   "/boot/efi",
		Label:        "EFI-SYSTEM",
		FSTabOptions: "umask=0077,shortname=winnt",
		FSTabFreq:    0,
		FSTabPassNo:  2,
	},
}

// bootPartition defines a distinct filesystem for /boot
// which is needed for e.g. LVM or LUKS when using GRUB
// (which this project doesn't support today...)
// See also https://github.com/containers/bootc/pull/529/commits/e5548d8765079171e6ed39a3ab0479bc8681a1c9
var bootPartition = disk.Partition{
	Size: 1 * GibiByte,
	Type: disk.FilesystemDataGUID,
	UUID: disk.DataPartitionUUID,
	Payload: &disk.Filesystem{
		Type:         "ext4",
		Mountpoint:   "/boot",
		Label:        "boot",
		FSTabOptions: BootOptions,
		FSTabFreq:    1,
		FSTabPassNo:  2,
	},
}

// rootPartition holds the root filesystem; however note
// that while the type here defines "ext4" because the data
// type requires something there, in practice we pull
// the rootfs type from the container image by default.
// See https://containers.github.io/bootc/bootc-install.html
var rootPartition = disk.Partition{
	Size: 2 * GibiByte,
	Type: disk.FilesystemDataGUID,
	UUID: disk.RootPartitionUUID,
	Payload: &disk.Filesystem{
		Type:         "ext4",
		Label:        "root",
		Mountpoint:   "/",
		FSTabOptions: RootOptions,
		FSTabFreq:    1,
		FSTabPassNo:  1,
	},
}

var partitionTables = distro.BasePartitionTableMap{
	arch.ARCH_X86_64.String(): disk.PartitionTable{
		UUID: diskUuidOfUnknownOrigin,
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Size:     1 * MebiByte,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			efiPartition,
			bootPartition,
			rootPartition,
		},
	},
	arch.ARCH_AARCH64.String(): disk.PartitionTable{
		UUID: diskUuidOfUnknownOrigin,
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			efiPartition,
			bootPartition,
			rootPartition,
		},
	},
	arch.ARCH_S390X.String(): disk.PartitionTable{
		UUID: diskUuidOfUnknownOrigin,
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			bootPartition,
			rootPartition,
		},
	},
	arch.ARCH_PPC64LE.String(): disk.PartitionTable{
		UUID: diskUuidOfUnknownOrigin,
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Size:     4 * MebiByte,
				Type:     disk.PRePartitionGUID,
				Bootable: true,
			},
			bootPartition,
			rootPartition,
		},
	},
}
