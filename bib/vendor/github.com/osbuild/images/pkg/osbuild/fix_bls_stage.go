package osbuild

// A FixBLSStageOptions struct
//
// The paths in the Bootloader Specification are relative to the partition
// they are located on, i.e. `/boot/loader/...` if `/boot` is on the root
// file-system partition. If `/boot` is on a separate partition, the correct
// path would be `/loader/.../` The `prefix` can be used to adjust for that.
// By default it is `/boot`, i.e. assumes `/boot` is on the root file-system.
type FixBLSStageOptions struct {
	// Prefix defaults to "/boot" if not provided
	Prefix *string `json:"prefix,omitempty"`
}

func (FixBLSStageOptions) isStageOptions() {}

// NewFixBLSStage creates a new FixBLSStage.
func NewFixBLSStage(options *FixBLSStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.fix-bls",
		Options: options,
	}
}
