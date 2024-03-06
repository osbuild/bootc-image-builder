package osbuild

type WAAgentConfig struct {
	RDFormat     *bool `json:"ResourceDisk.Format"`
	RDEnableSwap *bool `json:"ResourceDisk.EnableSwap"`
}

type WAAgentConfStageOptions struct {
	Config WAAgentConfig `json:"config"`
}

func (WAAgentConfStageOptions) isStageOptions() {}

func NewWAAgentConfStage(options *WAAgentConfStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.waagent.conf",
		Options: options,
	}
}
