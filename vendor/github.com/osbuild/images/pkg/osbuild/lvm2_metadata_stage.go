package osbuild

import (
	"fmt"
	"regexp"
	"strconv"
)

// Set LVM2 Volume Group metadata

type LVM2MetadataStageOptions struct {
	CreationHost string `json:"creation_host,omitempty"`

	// Creation time (uint64 represented as string)
	CreationTime string `json:"creation_time,omitempty"`

	Description string `json:"description,omitempty"`

	VGName string `json:"vg_name"`
}

func (LVM2MetadataStageOptions) isStageOptions() {}

func (o LVM2MetadataStageOptions) validate() error {
	nameRegex := regexp.MustCompile(lvmVolNameRegex)
	if !nameRegex.MatchString(o.VGName) {
		return fmt.Errorf("volume group name %q doesn't conform to schema (%s)", o.VGName, nameRegex.String())
	}

	if o.CreationTime != "" {
		if _, err := strconv.ParseUint(o.CreationTime, 10, 64); err != nil {
			return fmt.Errorf("invalid volume creation time: %s", o.CreationTime)
		}
	}
	return nil
}

func NewLVM2MetadataStage(options *LVM2MetadataStageOptions, devices Devices) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.lvm2.metadata",
		Options: options,
		Devices: devices,
	}
}
