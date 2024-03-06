package osbuild

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/disk"
)

// sfdiskStageOptions creates the options and devices properties for an
// org.osbuild.sfdisk stage based on a partition table description
func sfdiskStageOptions(pt *disk.PartitionTable) *SfdiskStageOptions {
	partitions := make([]SfdiskPartition, len(pt.Partitions))
	for idx, p := range pt.Partitions {
		partitions[idx] = SfdiskPartition{
			Bootable: p.Bootable,
			Start:    pt.BytesToSectors(p.Start),
			Size:     pt.BytesToSectors(p.Size),
			Type:     p.Type,
			UUID:     p.UUID,
		}
	}
	stageOptions := &SfdiskStageOptions{
		Label:      pt.Type,
		UUID:       pt.UUID,
		Partitions: partitions,
	}

	return stageOptions
}

// sgdiskStageOptions creates the options and devices properties for an
// org.osbuild.sgdisk stage based on a partition table description
func sgdiskStageOptions(pt *disk.PartitionTable) *SgdiskStageOptions {
	partitions := make([]SgdiskPartition, len(pt.Partitions))
	for idx, p := range pt.Partitions {
		partitions[idx] = SgdiskPartition{
			Bootable: p.Bootable,
			Start:    pt.BytesToSectors(p.Start),
			Size:     pt.BytesToSectors(p.Size),
			Type:     p.Type,
		}

		if p.UUID != "" {
			u := uuid.MustParse(p.UUID)
			partitions[idx].UUID = &u
		}
	}

	stageOptions := &SgdiskStageOptions{
		UUID:       uuid.MustParse(pt.UUID),
		Partitions: partitions,
	}

	return stageOptions
}

type PartTool string

const (
	PTSfdisk PartTool = "sfdisk"
	PTSgdisk PartTool = "sgdisk"
)

func GenImagePrepareStages(pt *disk.PartitionTable, filename string, partTool PartTool) []*Stage {
	stages := make([]*Stage, 0)

	// create an empty file of the given size via `org.osbuild.truncate`
	stage := NewTruncateStage(
		&TruncateStageOptions{
			Filename: filename,
			Size:     fmt.Sprintf("%d", pt.Size),
		})

	stages = append(stages, stage)

	// create the partition layout in the empty file
	loopback := NewLoopbackDevice(
		&LoopbackDeviceOptions{
			Filename: filename,
			Lock:     true,
		},
	)

	if partTool == PTSfdisk {
		sfOptions := sfdiskStageOptions(pt)
		sfdisk := NewSfdiskStage(sfOptions, loopback)
		stages = append(stages, sfdisk)
	} else if partTool == PTSgdisk {
		sgOptions := sgdiskStageOptions(pt)
		sgdisk := NewSgdiskStage(sgOptions, loopback)
		stages = append(stages, sgdisk)
	} else {
		panic("programming error: unknown PartTool: " + partTool)
	}

	// Generate all the needed "devices", like LUKS2 and LVM2
	s := GenDeviceCreationStages(pt, filename)
	stages = append(stages, s...)

	// Generate all the filesystems on partitons and devices
	s = GenMkfsStages(pt, loopback)
	stages = append(stages, s...)

	return stages
}

func GenImageFinishStages(pt *disk.PartitionTable, filename string) []*Stage {
	return GenDeviceFinishStages(pt, filename)
}

func GenImageKernelOptions(pt *disk.PartitionTable) []string {
	cmdline := make([]string, 0)

	genOptions := func(e disk.Entity, path []disk.Entity) error {
		switch ent := e.(type) {
		case *disk.LUKSContainer:
			karg := "luks.uuid=" + ent.UUID
			cmdline = append(cmdline, karg)
		}
		return nil
	}

	_ = pt.ForEachEntity(genOptions)
	return cmdline
}
