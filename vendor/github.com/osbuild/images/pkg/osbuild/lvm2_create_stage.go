package osbuild

import (
	"fmt"
	"regexp"
)

const lvmVolNameRegex = "^[a-zA-Z0-9+_.][a-zA-Z0-9+_.-]*$"

// Create LVM2 physical volumes, volume groups, and logical volumes

type LVM2CreateStageOptions struct {
	Volumes []LogicalVolume `json:"volumes"`
}

func (LVM2CreateStageOptions) isStageOptions() {}

func (o LVM2CreateStageOptions) validate() error {
	if len(o.Volumes) == 0 {
		return fmt.Errorf("at least one volume is required")
	}

	nameRegex := regexp.MustCompile(lvmVolNameRegex)
	for _, volume := range o.Volumes {
		if !nameRegex.MatchString(volume.Name) {
			return fmt.Errorf("volume name %q doesn't conform to schema (%s)", volume.Name, nameRegex.String())
		}
	}
	return nil
}

type LogicalVolume struct {
	Name string `json:"name"`

	Size string `json:"size"`
}

func NewLVM2CreateStage(options *LVM2CreateStageOptions, devices Devices) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.lvm2.create",
		Options: options,
		Devices: devices,
	}
}
