package osbuild

type DiscinfoStageOptions struct {
	// Build architecture
	BaseArch string `json:"basearch"`

	// The product name
	Release string `json:"release"`
}

func (DiscinfoStageOptions) isStageOptions() {}

// Creates a .discinfo file describing a disk
func NewDiscinfoStage(options *DiscinfoStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.discinfo",
		Options: options,
	}
}
