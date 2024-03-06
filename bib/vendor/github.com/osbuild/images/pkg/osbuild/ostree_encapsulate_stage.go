package osbuild

type OSTreeEncapsulateStageOptions struct {
	// Resulting image filename
	Filename string `json:"filename"`

	Cmd []string `json:"cmd,omitempty"`

	// Propagate an OSTree commit metadata key to container label
	CopyMeta []string `json:"copymeta,omitempty"`

	// The encapsulated container format version (default 1)
	FormatVersion *int `json:"format_version,omitempty"`

	// Additional labels for the container
	Labels []string `json:"labels,omitempty"`

	// Max number of container image layers
	MaxLayers *int `json:"max_layers,omitempty"`
}

func (OSTreeEncapsulateStageOptions) isStageOptions() {}

type OSTreeEncapsulateStageInput struct {
	inputCommon
	References []string `json:"references"`
}

func (OSTreeEncapsulateStageInput) isStageInput() {}

type OSTreeEncapsulateStageInputs struct {
	Commit *OSTreeEncapsulateStageInput `json:"commit"`
}

func (OSTreeEncapsulateStageInputs) isStageInputs() {}

func NewOSTreeEncapsulateStage(options *OSTreeEncapsulateStageOptions, inputPipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.encapsulate",
		Options: options,
		Inputs:  NewOSTreeEncapsulateStageInputs(InputOriginPipeline, inputPipeline),
	}
}

func NewOSTreeEncapsulateStageInputs(origin, pipeline string) *OSTreeEncapsulateStageInputs {
	encStageInput := new(OSTreeEncapsulateStageInput)
	encStageInput.Type = "org.osbuild.ostree"
	encStageInput.Origin = origin

	inputRefs := []string{"name:" + pipeline}
	encStageInput.References = inputRefs
	return &OSTreeEncapsulateStageInputs{Commit: encStageInput}
}
