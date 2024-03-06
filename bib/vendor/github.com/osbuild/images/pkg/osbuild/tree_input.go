package osbuild

// Tree inputs
type TreeInput struct {
	inputCommon
	References []string `json:"references"`
}

func (TreeInput) isInput() {}

// NewTreeInput creates an org.osbuild.tree input for an osbuild stage.
// The input is the final tree from a pipeline that should be referenced as
// 'name:<pipelinename>' in the reference argument.
func NewTreeInput(reference string) *TreeInput {
	input := new(TreeInput)
	input.Type = "org.osbuild.tree"
	input.Origin = "org.osbuild.pipeline"
	input.References = []string{reference}
	return input
}

type PipelineTreeInputs map[string]TreeInput

func NewPipelineTreeInputs(name, pipeline string) *PipelineTreeInputs {
	return &PipelineTreeInputs{
		name: *NewTreeInput("name:" + pipeline),
	}
}

func (PipelineTreeInputs) isStageInputs() {}
