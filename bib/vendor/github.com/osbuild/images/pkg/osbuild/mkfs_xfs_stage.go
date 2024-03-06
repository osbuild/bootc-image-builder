package osbuild

type MkfsXfsStageOptions struct {
	UUID  string `json:"uuid"`
	Label string `json:"label,omitempty"`
}

func (MkfsXfsStageOptions) isStageOptions() {}

func NewMkfsXfsStage(options *MkfsXfsStageOptions, devices map[string]Device) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.xfs",
		Options: options,
		Devices: devices,
	}
}
