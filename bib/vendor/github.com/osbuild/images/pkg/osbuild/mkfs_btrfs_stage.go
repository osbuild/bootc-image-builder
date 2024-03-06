package osbuild

type MkfsBtrfsStageOptions struct {
	UUID  string `json:"uuid"`
	Label string `json:"label,omitempty"`
}

func (MkfsBtrfsStageOptions) isStageOptions() {}

func NewMkfsBtrfsStage(options *MkfsBtrfsStageOptions, devices map[string]Device) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.btrfs",
		Options: options,
		Devices: devices,
	}
}
