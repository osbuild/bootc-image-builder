package osbuild

type DracutStageOptions struct {
	// List of target kernel versions
	Kernel []string `json:"kernel"`

	// Compression method for the initramfs
	Compress string `json:"compress,omitempty"`

	// Exact list of dracut modules to use
	Modules []string `json:"modules,omitempty"`

	// Additional dracut modules to include
	AddModules []string `json:"add_modules,omitempty"`

	// Dracut modules to not include
	OmitModules []string `json:"omit_modules,omitempty"`

	// Kernel modules to exclusively include
	Drivers []string `json:"drivers,omitempty"`

	// Add a specific kernel module
	AddDrivers []string `json:"add_drivers,omitempty"`

	// Add driver and ensure that they are tried to be loaded
	ForceDrivers []string `json:"force_drivers,omitempty"`

	// Kernel filesystem modules to exclusively include
	Filesystems []string `json:"filesystems,omitempty"`

	// Add custom files to the initramfs
	// What (keys) to include where (values)
	Include map[string]string `json:"include,omitempty"`

	// Install the specified files
	Install []string `json:"install,omitempty"`

	// Combine early microcode with the initramfs
	EarlyMicrocode bool `json:"early_microcode,omitempty"`

	// Create reproducible images
	Reproducible bool `json:"reproducible,omitempty"`

	// Extra arguments to directly pass to dracut
	Extra []string `json:"extra,omitempty"`
}

func (DracutStageOptions) isStageOptions() {}

// Dracut stage (re-)creates the initial RAM file-system
func NewDracutStage(options *DracutStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.dracut",
		Options: options,
	}
}
