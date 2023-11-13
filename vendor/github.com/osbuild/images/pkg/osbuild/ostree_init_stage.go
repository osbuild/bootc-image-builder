package osbuild

type InitMode string

const (
	ModeBare         InitMode = "bare"
	ModeBareUser     InitMode = "bare-user"
	ModeBareUserOnly InitMode = "bare-user-only"
	ModeArchvie      InitMode = "archive"
)

// Options for the org.osbuild.ostree.init stage.
type OSTreeInitStageOptions struct {
	// The Mode in which to initialise the repo
	Mode InitMode `json:"mode,omitempty"`

	// Location in which to create the repo
	Path string `json:"path,omitempty"`
}

func (OSTreeInitStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeInitStage(options *OSTreeInitStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.init",
		Options: options,
	}
}
