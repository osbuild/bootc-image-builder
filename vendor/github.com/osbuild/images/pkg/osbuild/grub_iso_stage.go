package osbuild

type GrubISOStageOptions struct {
	Product Product `json:"product"`

	Kernel ISOKernel `json:"kernel"`

	ISOLabel string `json:"isolabel"`

	Architectures []string `json:"architectures,omitempty"`

	Vendor string `json:"vendor,omitempty"`
}

func (GrubISOStageOptions) isStageOptions() {}

type ISOKernel struct {
	Dir string `json:"dir"`

	// Additional kernel boot options
	Opts []string `json:"opts,omitempty"`
}

// Assemble a file system tree for a bootable ISO
func NewGrubISOStage(options *GrubISOStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.grub2.iso",
		Options: options,
	}
}
