package osbuild

import "github.com/osbuild/images/pkg/disk"

// Install the Z Initial Program Loader

type ZiplInstStageOptions struct {
	Kernel string `json:"kernel"`

	// The offset of the partition containing /boot
	Location uint64 `json:"location"`

	SectorSize *uint64 `json:"sector-size,omitempty"`
}

func (ZiplInstStageOptions) isStageOptions() {}

// Return a new zipl.inst stage. The 'disk' parameter must represent the
// (entire) device that contains the /boot partition.
func NewZiplInstStage(options *ZiplInstStageOptions, disk *Device, devices map[string]Device, mounts []Mount) *Stage {
	// create a new devices map and add the disk to it
	devmap := map[string]Device(devices)
	devmap["disk"] = *disk
	ziplDevices := map[string]Device(devmap)
	return &Stage{
		Type:    "org.osbuild.zipl.inst",
		Options: options,
		Devices: ziplDevices,
		Mounts:  mounts,
	}
}

func NewZiplInstStageOptions(kernel string, pt *disk.PartitionTable) *ZiplInstStageOptions {
	bootIdx := -1
	rootIdx := -1
	for idx := range pt.Partitions {
		// NOTE: we only support having /boot at the top level of the partition
		// table (e.g., not in LUKS or LVM), so we don't need to descend into
		// VolumeContainer types. If /boot is on the root partition, then the
		// root partition needs to be at the top level.
		partition := &pt.Partitions[idx]
		if partition.Payload == nil {
			continue
		}
		mnt, isMountable := partition.Payload.(disk.Mountable)
		if !isMountable {
			continue
		}
		if mnt.GetMountpoint() == "/boot" {
			bootIdx = idx
		} else if mnt.GetMountpoint() == "/" {
			rootIdx = idx
		}
	}
	if bootIdx == -1 {
		// if there's no boot partition, fall back to root
		if rootIdx == -1 {
			// no root either!?
			panic("failed to find boot or root partition for zipl.inst stage")
		}
		bootIdx = rootIdx
	}

	bootPart := pt.Partitions[bootIdx]
	return &ZiplInstStageOptions{
		Kernel:   kernel,
		Location: pt.BytesToSectors(bootPart.Start),
	}
}
