package osbuild

// Options for the org.osbuild.ostree.fillvar stage.
type OSTreeFillvarStageOptions struct {
	Deployment OSTreeDeployment `json:"deployment"`
}

type OSTreeDeployment struct {
	OSName string `json:"osname"`

	Ref string `json:"ref"`

	Serial *int `json:"serial,omitempty"`
}

func (OSTreeFillvarStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeFillvarStage(options *OSTreeFillvarStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.fillvar",
		Options: options,
	}
}
