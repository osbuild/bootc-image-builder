package osbuild

type SquashfsStageOptions struct {
	Filename string `json:"filename"`

	Compression FSCompression `json:"compression"`
}

func (SquashfsStageOptions) isStageOptions() {}

func NewSquashfsStage(options *SquashfsStageOptions, inputPipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.squashfs",
		Options: options,
		Inputs:  NewPipelineTreeInputs("tree", inputPipeline),
	}
}
