package osbuild

// Options for the org.osbuild.ostree.pull stage.
type OSTreePullStageOptions struct {
	// Location of the ostree repo
	Repo string `json:"repo"`
	// Remote to configure for all commits
	Remote string `json:"remote,omitempty"`
}

func (OSTreePullStageOptions) isStageOptions() {}

type OSTreePullStageInput struct {
	inputCommon
	References OSTreePullStageReferences `json:"references"`
}

func (OSTreePullStageInput) isStageInput() {}

type OSTreePullStageInputs struct {
	Commits *OSTreePullStageInput `json:"commits"`
}

func (OSTreePullStageInputs) isStageInputs() {}

type OSTreePullStageReferences map[string]OSTreePullStageReference

func (OSTreePullStageReferences) isReferences() {}

type OSTreePullStageReference struct {
	Ref string `json:"ref"`
}

// A new org.osbuild.ostree.pull stage to pull OSTree commits into an existing repo
func NewOSTreePullStage(options *OSTreePullStageOptions, inputs *OSTreePullStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.pull",
		Inputs:  inputs,
		Options: options,
	}
}

func NewOstreePullStageInputs(origin, source, commitRef string) *OSTreePullStageInputs {
	pullStageInput := new(OSTreePullStageInput)
	pullStageInput.Type = "org.osbuild.ostree"
	pullStageInput.Origin = origin

	inputRefs := make(map[string]OSTreePullStageReference)
	inputRefs[source] = OSTreePullStageReference{Ref: commitRef}
	pullStageInput.References = inputRefs
	return &OSTreePullStageInputs{Commits: pullStageInput}
}
