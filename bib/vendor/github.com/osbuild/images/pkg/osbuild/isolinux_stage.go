package osbuild

type ISOLinuxStageOptions struct {
	Product ISOLinuxProduct `json:"product"`
	Kernel  ISOLinuxKernel  `json:"kernel"`
}

func (ISOLinuxStageOptions) isStageOptions() {}

type ISOLinuxProduct struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ISOLinuxKernel struct {
	Dir string `json:"dir"`

	Opts []string `json:"opts"`
}

func NewISOLinuxStage(options *ISOLinuxStageOptions, inputPipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.isolinux",
		Options: options,
		Inputs:  NewPipelineTreeInputs("data", inputPipeline),
	}
}
