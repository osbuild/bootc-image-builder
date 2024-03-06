package osbuild

// The org.osbuild.ostree.passwd stage has no options so far.
type OSTreePasswdStageOptions struct {
}

func (s *OSTreePasswdStageOptions) isStageOptions() {}

type OSTreePasswdStageInputs struct {
	Commits *OSTreeCheckoutInput `json:"commits"`
}

func (OSTreePasswdStageInputs) isStageInputs() {}

// A new org.osbuild.ostree.passwd stage to pre-fill the user and group databases
func NewOSTreePasswdStage(origin, name string) *Stage {
	return &Stage{
		Type:   "org.osbuild.ostree.passwd",
		Inputs: &OSTreePasswdStageInputs{Commits: NewOSTreeCheckoutInput(origin, name)},
	}
}
