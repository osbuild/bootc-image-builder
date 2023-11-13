package osbuild

type LUKS2RemoveKeyStageOptions struct {
	Passphrase string `json:"passphrase"`
}

func (LUKS2RemoveKeyStageOptions) isStageOptions() {}

func NewLUKS2RemoveKeyStage(options *LUKS2RemoveKeyStageOptions, devices Devices) *Stage {
	return &Stage{
		Type:    "org.osbuild.luks2.remove-key",
		Options: options,
		Devices: devices,
	}
}
