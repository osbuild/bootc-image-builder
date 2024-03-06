package osbuild

// Inputs for ostree commits
type OSTreeInput struct {
	inputCommon
}

func (OSTreeInput) isStageInputs() {}

func NewOSTreeInput() *OSTreeInput {
	input := new(OSTreeInput)
	input.Type = "org.osbuild.ostree"
	input.Origin = "org.osbuild.source"
	return input
}

// Inputs of type org.osbuild.ostree.checkout
type OSTreeCheckoutInput struct {
	inputCommon
	References OSTreeCheckoutReferences `json:"references"`
}

type OSTreeCheckoutReferences []string

func (OSTreeCheckoutReferences) isReferences() {}

// NewOSTreeCommitsInput creates a new OSTreeCommitsInputs
// where `origin` is either "org.osbuild.source" or "org.osbuild.pipeline
// `name` is the id of the commit, i.e. its digest or the pipeline name that
// produced it)
func NewOSTreeCheckoutInput(origin, name string) *OSTreeCheckoutInput {
	input := new(OSTreeCheckoutInput)
	input.Type = "org.osbuild.ostree.checkout"
	input.Origin = origin

	inputRefs := make([]string, 1)
	inputRefs[0] = name
	input.References = inputRefs
	return input
}

type OSTreeCheckoutInputs map[string]OSTreeCheckoutInput

func (c OSTreeCheckoutInputs) isStageInputs() {}
