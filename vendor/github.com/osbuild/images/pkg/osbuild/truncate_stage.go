package osbuild

// Create, shrink, or extend a file

type TruncateStageOptions struct {
	// Image filename
	Filename string `json:"filename"`

	// Desired size
	Size string `json:"size"`
}

func (TruncateStageOptions) isStageOptions() {}

func NewTruncateStage(options *TruncateStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.truncate",
		Options: options,
	}
}
