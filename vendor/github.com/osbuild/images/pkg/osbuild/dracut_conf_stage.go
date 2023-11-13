package osbuild

import (
	"encoding/json"
	"fmt"
)

type DracutConfStageOptions struct {
	Filename string           `json:"filename"`
	Config   DracutConfigFile `json:"config"`
}

func (DracutConfStageOptions) isStageOptions() {}

// Dracut.conf stage creates dracut configuration files under /usr/lib/dracut/dracut.conf.d/
func NewDracutConfStage(options *DracutConfStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.dracut.conf",
		Options: options,
	}
}

type DracutConfigFile struct {
	// Compression method for the initramfs
	Compress string `json:"compress,omitempty"`

	// Exact list of dracut modules to use
	Modules []string `json:"dracutmodules,omitempty"`

	// Additional dracut modules to include
	AddModules []string `json:"add_dracutmodules,omitempty"`

	// Dracut modules to not include
	OmitModules []string `json:"omit_dracutmodules,omitempty"`

	// Kernel modules to exclusively include
	Drivers []string `json:"drivers,omitempty"`

	// Add a specific kernel module
	AddDrivers []string `json:"add_drivers,omitempty"`

	// Add driver and ensure that they are tried to be loaded
	ForceDrivers []string `json:"force_drivers,omitempty"`

	// Kernel filesystem modules to exclusively include
	Filesystems []string `json:"filesystems,omitempty"`

	// Install the specified files
	Install []string `json:"install_items,omitempty"`

	// Combine early microcode with the initramfs
	EarlyMicrocode *bool `json:"early_microcode,omitempty"`

	// Create reproducible images
	Reproducible *bool `json:"reproducible,omitempty"`
}

// Unexported alias for use in DracutConfigFile MarshalJSON() to prevent recursion
type dracutConfigFile DracutConfigFile

func (c DracutConfigFile) MarshalJSON() ([]byte, error) {
	if c.Compress == "" &&
		len(c.Modules) == 0 &&
		len(c.AddModules) == 0 &&
		len(c.OmitModules) == 0 &&
		len(c.Drivers) == 0 &&
		len(c.AddDrivers) == 0 &&
		len(c.ForceDrivers) == 0 &&
		len(c.Filesystems) == 0 &&
		len(c.Install) == 0 &&
		c.EarlyMicrocode == nil &&
		c.Reproducible == nil {
		return nil, fmt.Errorf("at least one dracut configuration option must be specified")
	}
	configFile := dracutConfigFile(c)
	return json.Marshal(configFile)
}
