package osbuild

import (
	"strings"

	"github.com/osbuild/images/pkg/disk"
)

// GenMkfsStages generates a list of org.mkfs.* stages based on a
// partition table description for a single device node
func GenMkfsStages(pt *disk.PartitionTable, device *Device) []*Stage {
	stages := make([]*Stage, 0, len(pt.Partitions))

	// assume loopback device for simplicity since it's the only one currently supported
	// panic if the conversion fails
	devOptions, ok := device.Options.(*LoopbackDeviceOptions)
	if !ok {
		panic("GenMkfsStages: failed to convert device options to loopback options")
	}

	genStage := func(mnt disk.Mountable, path []disk.Entity) error {
		t := mnt.GetFSType()
		var stage *Stage

		stageDevices, lastName := getDevices(path, devOptions.Filename, true)

		// the last device on the PartitionTable must be named "device"
		lastDevice := stageDevices[lastName]
		delete(stageDevices, lastName)
		stageDevices["device"] = lastDevice

		fsSpec := mnt.GetFSSpec()
		switch t {
		case "xfs":
			options := &MkfsXfsStageOptions{
				UUID:  fsSpec.UUID,
				Label: fsSpec.Label,
			}
			stage = NewMkfsXfsStage(options, stageDevices)
		case "vfat":
			options := &MkfsFATStageOptions{
				VolID: strings.Replace(fsSpec.UUID, "-", "", -1),
			}
			stage = NewMkfsFATStage(options, stageDevices)
		case "btrfs":
			options := &MkfsBtrfsStageOptions{
				UUID:  fsSpec.UUID,
				Label: fsSpec.Label,
			}
			stage = NewMkfsBtrfsStage(options, stageDevices)
		case "ext4":
			options := &MkfsExt4StageOptions{
				UUID:  fsSpec.UUID,
				Label: fsSpec.Label,
			}
			stage = NewMkfsExt4Stage(options, stageDevices)
		default:
			panic("unknown fs type " + t)
		}
		stages = append(stages, stage)

		return nil
	}

	_ = pt.ForEachMountable(genStage) // genStage always returns nil
	return stages
}
