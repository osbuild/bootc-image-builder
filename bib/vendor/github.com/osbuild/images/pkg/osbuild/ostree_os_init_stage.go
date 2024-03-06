package osbuild

// Options for the org.osbuild.ostree.os-init stage.
type OSTreeOsInitStageOptions struct {
	// Name of the OS
	OSName string `json:"osname"`
}

func (OSTreeOsInitStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeOsInitStage(options *OSTreeOsInitStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.os-init",
		Options: options,
	}
}
