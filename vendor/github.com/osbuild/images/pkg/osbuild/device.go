package osbuild

import (
	"fmt"
	"strings"

	"github.com/osbuild/images/pkg/disk"
)

type Devices map[string]Device

type Device struct {
	Type    string        `json:"type"`
	Parent  string        `json:"parent,omitempty"`
	Options DeviceOptions `json:"options,omitempty"`
}

type DeviceOptions interface {
	isDeviceOptions()
}

func GenDeviceCreationStages(pt *disk.PartitionTable, filename string) []*Stage {
	stages := make([]*Stage, 0)

	genStages := func(e disk.Entity, path []disk.Entity) error {

		switch ent := e.(type) {
		case *disk.LUKSContainer:
			// do not include us when getting the devices
			stageDevices, lastName := getDevices(path[:len(path)-1], filename, true)

			// "org.osbuild.luks2.format" expects a "device" to create the VG on,
			// thus rename the last device to "device"
			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			stage := NewLUKS2CreateStage(
				&LUKS2CreateStageOptions{
					UUID:       ent.UUID,
					Passphrase: ent.Passphrase,
					Cipher:     ent.Cipher,
					Label:      ent.Label,
					Subsystem:  ent.Subsystem,
					SectorSize: ent.SectorSize,
					PBKDF: Argon2id{
						Method:      "argon2id",
						Iterations:  ent.PBKDF.Iterations,
						Memory:      ent.PBKDF.Memory,
						Parallelism: ent.PBKDF.Parallelism,
					},
				},
				stageDevices)

			stages = append(stages, stage)

			if ent.Clevis != nil {
				stages = append(stages, NewClevisLuksBindStage(&ClevisLuksBindStageOptions{
					Passphrase: ent.Passphrase,
					Pin:        ent.Clevis.Pin,
					Policy:     ent.Clevis.Policy,
				}, stageDevices))
			}

		case *disk.LVMVolumeGroup:
			// do not include us when getting the devices
			stageDevices, lastName := getDevices(path[:len(path)-1], filename, true)

			// "org.osbuild.lvm2.create" expects a "device" to create the VG on,
			// thus rename the last device to "device"
			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			volumes := make([]LogicalVolume, len(ent.LogicalVolumes))
			for idx, lv := range ent.LogicalVolumes {
				volumes[idx].Name = lv.Name
				// NB: we need to specify the size in bytes, since lvcreate
				// defaults to megabytes
				volumes[idx].Size = fmt.Sprintf("%dB", lv.Size)
			}

			stage := NewLVM2CreateStage(
				&LVM2CreateStageOptions{
					Volumes: volumes,
				}, stageDevices)

			stages = append(stages, stage)
		}

		return nil
	}

	_ = pt.ForEachEntity(genStages)
	return stages
}

func GenDeviceFinishStages(pt *disk.PartitionTable, filename string) []*Stage {
	stages := make([]*Stage, 0)
	removeKeyStages := make([]*Stage, 0)

	genStages := func(e disk.Entity, path []disk.Entity) error {

		switch ent := e.(type) {
		case *disk.LUKSContainer:
			// do not include us when getting the devices
			stageDevices, lastName := getDevices(path[:len(path)-1], filename, true)

			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			if ent.Clevis != nil {
				if ent.Clevis.RemovePassphrase {
					removeKeyStages = append(removeKeyStages, NewLUKS2RemoveKeyStage(&LUKS2RemoveKeyStageOptions{
						Passphrase: ent.Passphrase,
					}, stageDevices))
				}
			}
		case *disk.LVMVolumeGroup:
			// do not include us when getting the devices
			stageDevices, lastName := getDevices(path[:len(path)-1], filename, true)

			// "org.osbuild.lvm2.metadata" expects a "device" to rename the VG,
			// thus rename the last device to "device"
			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			stage := NewLVM2MetadataStage(
				&LVM2MetadataStageOptions{
					VGName: ent.Name,
				}, stageDevices)

			stages = append(stages, stage)
		}

		return nil
	}

	_ = pt.ForEachEntity(genStages)
	// Ensure that "org.osbuild.luks2.remove-key" stages are done after
	// "org.osbuild.lvm2.metadata" stages, we cannot open a device if its
	// password has changed
	stages = append(stages, removeKeyStages...)
	return stages
}

func deviceName(p disk.Entity) string {
	if p == nil {
		panic("device is nil; this is a programming error")
	}

	switch payload := p.(type) {
	case disk.Mountable:
		return pathEscape(payload.GetMountpoint())
	case *disk.LUKSContainer:
		return "luks-" + payload.UUID[:4]
	case *disk.LVMVolumeGroup:
		return payload.Name
	case *disk.LVMLogicalVolume:
		return payload.Name
	}
	panic(fmt.Sprintf("unsupported device type in deviceName: '%T'", p))
}

func getDevices(path []disk.Entity, filename string, lockLoopback bool) (map[string]Device, string) {
	var pt *disk.PartitionTable

	do := make(map[string]Device)
	parent := ""
	for _, elem := range path {
		switch e := elem.(type) {
		case *disk.PartitionTable:
			pt = e
		case *disk.Partition:
			if pt == nil {
				panic("path does not contain partition table; this is a programming error")
			}
			lbopt := LoopbackDeviceOptions{
				Filename:   filename,
				Start:      pt.BytesToSectors(e.Start),
				Size:       pt.BytesToSectors(e.Size),
				SectorSize: nil,
				Lock:       lockLoopback,
			}
			name := deviceName(e.Payload)
			do[name] = *NewLoopbackDevice(&lbopt)
			parent = name
		case *disk.LUKSContainer:
			lo := LUKS2DeviceOptions{
				Passphrase: e.Passphrase,
			}
			name := deviceName(e.Payload)
			do[name] = *NewLUKS2Device(parent, &lo)
			parent = name
		case *disk.LVMLogicalVolume:
			lo := LVM2LVDeviceOptions{
				Volume: e.Name,
			}
			name := deviceName(e.Payload)
			do[name] = *NewLVM2LVDevice(parent, &lo)
			parent = name
		}
	}
	return do, parent
}

// pathEscape implements similar path escaping as used by systemd-escape
// https://github.com/systemd/systemd/blob/c57ff6230e4e199d40f35a356e834ba99f3f8420/src/basic/unit-name.c#L389
func pathEscape(path string) string {
	if len(path) == 0 || path == "/" {
		return "-"
	}

	path = strings.Trim(path, "/")

	escapeChars := func(s, char string) string {
		return strings.ReplaceAll(s, char, fmt.Sprintf("\\x%x", char[0]))
	}

	path = escapeChars(path, "\\")
	path = escapeChars(path, "-")

	return strings.ReplaceAll(path, "/", "-")
}
