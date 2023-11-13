package osbuild

type FirstBootStageOptions struct {
	Commands       []string `json:"commands"`
	WaitForNetwork bool     `json:"wait_for_network,omitempty"`
}

func (FirstBootStageOptions) isStageOptions() {}

func NewFirstBootStage(options *FirstBootStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.first-boot",
		Options: options,
	}
}
