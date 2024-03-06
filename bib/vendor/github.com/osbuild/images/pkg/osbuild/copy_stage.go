package osbuild

import (
	"fmt"

	"github.com/osbuild/images/pkg/disk"
)

// Stage to copy items from inputs to mount points or the tree. Multiple items
// can be copied. The source and destination is a URL.

type CopyStageOptions struct {
	Paths []CopyStagePath `json:"paths"`
}

type CopyStagePath struct {
	From string `json:"from"`
	To   string `json:"to"`

	// Remove the destination before copying. Works only for files, not directories.
	// Default: false
	RemoveDestination bool `json:"remove_destination,omitempty"`
}

func (CopyStageOptions) isStageOptions() {}

func NewCopyStage(options *CopyStageOptions, inputs Inputs, devices map[string]Device, mounts []Mount) *Stage {
	return &Stage{
		Type:    "org.osbuild.copy",
		Options: options,
		Inputs:  inputs,
		Devices: devices,
		Mounts:  mounts,
	}
}

func NewCopyStageSimple(options *CopyStageOptions, inputs Inputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.copy",
		Options: options,
		Inputs:  inputs,
	}
}

type CopyStageFilesInputs map[string]*FilesInput

func (*CopyStageFilesInputs) isStageInputs() {}

// GenCopyFSTreeOptions creates the options, inputs, devices, and mounts properties
// for an org.osbuild.copy stage for a given source tree using a partition
// table description to define the mounts
//
// TODO: the `inputPipeline` parameter is not used. We should instead split out
// the part that creates Devices and Mounts into a separate functions
// such as `GenFSMounts()` and `GenFSMountsDevices()` and take their output
// as parameters. Also we should be returning the final stage from this
// function, not just the options, devices, and mounts.
func GenCopyFSTreeOptions(inputName, inputPipeline, filename string, pt *disk.PartitionTable) (
	*CopyStageOptions,
	map[string]Device,
	[]Mount,
) {

	fsRootMntName, mounts, devices, err := genMountsDevicesFromPt(filename, pt)
	if err != nil {
		panic(err)
	}

	options := CopyStageOptions{
		Paths: []CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/", inputName),
				To:   fmt.Sprintf("mount://%s/", fsRootMntName),
			},
		},
	}

	return &options, devices, mounts
}
