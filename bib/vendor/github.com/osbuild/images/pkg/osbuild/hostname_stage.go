package osbuild

type HostnameStageOptions struct {
	Hostname string `json:"hostname"`
}

func (HostnameStageOptions) isStageOptions() {}

func NewHostnameStage(options *HostnameStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.hostname",
		Options: options,
	}
}
