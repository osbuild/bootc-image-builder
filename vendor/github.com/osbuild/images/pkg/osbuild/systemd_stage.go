package osbuild

type SystemdStageOptions struct {
	EnabledServices  []string `json:"enabled_services,omitempty"`
	DisabledServices []string `json:"disabled_services,omitempty"`
	MaskedServices   []string `json:"masked_services,omitempty"`
	DefaultTarget    string   `json:"default_target,omitempty"`
}

func (SystemdStageOptions) isStageOptions() {}

func NewSystemdStage(options *SystemdStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.systemd",
		Options: options,
	}
}
