package osbuild

// Partition a target using sgdisk(8)

import (
	"github.com/google/uuid"
)

type SgdiskStageOptions struct {
	// UUID for the disk image's partition table
	UUID uuid.UUID `json:"uuid"`

	// Partition layout
	Partitions []SgdiskPartition `json:"partitions,omitempty"`
}

func (SgdiskStageOptions) isStageOptions() {}

// Description of a partition
type SgdiskPartition struct {
	// Mark the partition as bootable (dos)
	Bootable bool `json:"bootable,omitempty"`

	// The partition name
	Name string `json:"name,omitempty"`

	// The size of the partition (sectors)
	Size uint64 `json:"size,omitempty"`

	// The start offset of the partition (sectors)
	Start uint64 `json:"start,omitempty"`

	// The partition type
	Type string `json:"type,omitempty"`

	// UUID of the partition
	UUID *uuid.UUID `json:"uuid,omitempty"`
}

func NewSgdiskStage(options *SgdiskStageOptions, device *Device) *Stage {
	return &Stage{
		Type:    "org.osbuild.sgdisk",
		Options: options,
		Devices: Devices{"device": *device},
	}
}
