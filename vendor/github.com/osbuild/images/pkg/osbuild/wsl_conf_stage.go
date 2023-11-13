package osbuild

type WSLConfStageOptions struct {
	Boot WSLConfBootOptions `json:"boot"`
}

type WSLConfBootOptions struct {
	Systemd bool `json:"systemd"`
}

func (WSLConfStageOptions) isStageOptions() {}

func NewWSLConfStage(options *WSLConfStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.wsl.conf",
		Options: options,
	}
}
