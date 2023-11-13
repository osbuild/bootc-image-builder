package osbuild

type MkfsExt4StageOptions struct {
	UUID  string `json:"uuid"`
	Label string `json:"label,omitempty"`
}

func (MkfsExt4StageOptions) isStageOptions() {}

func NewMkfsExt4Stage(options *MkfsExt4StageOptions, devices map[string]Device) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.ext4",
		Options: options,
		Devices: devices,
	}
}
