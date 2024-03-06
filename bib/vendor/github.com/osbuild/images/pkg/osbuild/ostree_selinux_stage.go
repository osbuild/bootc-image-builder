package osbuild

// Options for the org.osbuild.ostree.selinux stage.
type OSTreeSelinuxStageOptions struct {
	// shared with ostree.fillvar
	Deployment OSTreeDeployment `json:"deployment"`
}

func (OSTreeSelinuxStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeSelinuxStage(options *OSTreeSelinuxStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.selinux",
		Options: options,
	}
}
