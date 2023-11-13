package osbuild

type MkfsFATStageOptions struct {
	VolID   string `json:"volid"`
	Label   string `json:"label,omitempty"`
	FATSize *int   `json:"fat-size,omitempty"`
}

func (MkfsFATStageOptions) isStageOptions() {}

func NewMkfsFATStage(options *MkfsFATStageOptions, devices map[string]Device) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.fat",
		Options: options,
		Devices: devices,
	}
}
