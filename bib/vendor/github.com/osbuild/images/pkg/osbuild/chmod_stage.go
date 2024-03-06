package osbuild

type ChmodStageOptions struct {
	Items map[string]ChmodStagePathOptions `json:"items"`
}

type ChmodStagePathOptions struct {
	Mode      string `json:"mode"`
	Recursive bool   `json:"recursive,omitempty"`
}

func (ChmodStageOptions) isStageOptions() {}

// NewChmodStage creates a new org.osbuild.chmod stage
func NewChmodStage(options *ChmodStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.chmod",
		Options: options,
	}
}
