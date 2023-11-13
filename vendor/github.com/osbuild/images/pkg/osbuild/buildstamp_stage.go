package osbuild

type BuildstampStageOptions struct {
	// Build architecture
	Arch string `json:"arch"`

	// The product name
	Product string `json:"product"`

	// The version
	Version string `json:"version"`

	Final bool `json:"final"`

	// The variant of the product
	Variant string `json:"variant"`

	// The bugurl of the product
	BugURL string `json:"bugurl"`
}

func (BuildstampStageOptions) isStageOptions() {}

// Creates a buildstamp file describing the system (required by anaconda)
func NewBuildstampStage(options *BuildstampStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.buildstamp",
		Options: options,
	}
}
