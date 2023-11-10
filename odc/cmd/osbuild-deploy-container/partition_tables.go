package main

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
)

const (
	MebiByte = 1024 * 1024        // MiB
	GibiByte = 1024 * 1024 * 1024 // GiB
)

var partitionTables = distro.BasePartitionTableMap{
	arch.ARCH_X86_64.String(): disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     1 * MebiByte,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
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
			},
			{
				Size: 1 * GibiByte,
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 2 * GibiByte,
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
		},
	},
	arch.ARCH_AARCH64.String(): disk.PartitionTable{
		UUID: "0xc1748067",
		Type: "dos",
		Partitions: []disk.Partition{
			{
				Size:     501 * MebiByte,
				Type:     "06",
				Bootable: true,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
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
					FSTabOptions: "defaults",
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
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
		},
	},
}
