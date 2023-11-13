package osbuild

type ClevisLuksBindStageOptions struct {
	Passphrase string `json:"passphrase"`
	Pin        string `json:"pin"`
	Policy     string `json:"policy"`
}

func (ClevisLuksBindStageOptions) isStageOptions() {}

func NewClevisLuksBindStage(options *ClevisLuksBindStageOptions, devices Devices) *Stage {
	return &Stage{
		Type:    "org.osbuild.clevis.luks-bind",
		Options: options,
		Devices: devices,
	}
}
