package osbuild

import (
	"fmt"
	"sort"

	"github.com/osbuild/images/pkg/disk"
)

type BootupdStageOptionsBios struct {
	Device    string `json:"device"`
	Partition int    `json:"partition,omitempty"`
}

type BootupdStageOptions struct {
	Deployment    *OSTreeDeployment        `json:"deployment,omitempty"`
	StaticConfigs bool                     `json:"static-configs"`
	Bios          *BootupdStageOptionsBios `json:"bios,omitempty"`
}

func (BootupdStageOptions) isStageOptions() {}

func (opts *BootupdStageOptions) validate(devices map[string]Device) error {
	if opts.Bios != nil && opts.Bios.Device != "" {
		if _, ok := devices[opts.Bios.Device]; !ok {
			var devnames []string
			for devname := range devices {
				devnames = append(devnames, devname)
			}
			sort.Strings(devnames)
			return fmt.Errorf("cannot find expected device %q for bootupd bios option in %v", opts.Bios.Device, devnames)
		}
	}
	return nil
}

// validateBootupdMounts ensures that all required mounts for the bootup
// stage are generated. Right now the stage requires root, boot and boot/efi
// to find all the bootloader configs
func validateBootupdMounts(mounts []Mount) error {
	requiredMounts := map[string]bool{
		"/":         true,
		"/boot":     true,
		"/boot/efi": true,
	}
	for _, mnt := range mounts {
		delete(requiredMounts, mnt.Target)
	}
	if len(requiredMounts) != 0 {
		var missingMounts []string
		for mnt := range requiredMounts {
			missingMounts = append(missingMounts, mnt)
		}
		sort.Strings(missingMounts)
		return fmt.Errorf("required mounts for bootupd stage %v missing", missingMounts)
	}
	return nil
}

// NewBootupdStage creates a new stage for the org.osbuild.bootupd stage. It
// requires a mount setup of "/", "/boot" and "/boot/efi" right now so that
// bootupd can find and install all required bootloader bits.
func NewBootupdStage(opts *BootupdStageOptions, devices map[string]Device, mounts []Mount) (*Stage, error) {
	if err := validateBootupdMounts(mounts); err != nil {
		return nil, err
	}
	if err := opts.validate(devices); err != nil {
		return nil, err
	}

	return &Stage{
		Type:    "org.osbuild.bootupd",
		Options: opts,
		Devices: devices,
		Mounts:  mounts,
	}, nil
}

func GenBootupdDevicesMounts(filename string, pt *disk.PartitionTable) (map[string]Device, []Mount) {
	_, mounts, devices, err := genMountsDevicesFromPt(filename, pt)
	if err != nil {
		panic(err)
	}
	devices["disk"] = Device{
		Type: "org.osbuild.loopback",
		Options: &LoopbackDeviceOptions{
			Filename: filename,
		},
	}

	return devices, mounts
}
