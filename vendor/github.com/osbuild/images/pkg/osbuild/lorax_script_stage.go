package osbuild

type LoraxScriptStageOptions struct {
	// Where to put the script
	Path string `json:"path"`

	// The basic architecture parameter to supply to the template
	BaseArch string `json:"basearch,omitempty"`

	Product Product `json:"product,omitempty"`

	LibDir string `json:"libdir,omitempty"`
}

type Product struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (LoraxScriptStageOptions) isStageOptions() {}

// Run a Lorax template script on the tree
func NewLoraxScriptStage(options *LoraxScriptStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.lorax-script",
		Options: options,
	}
}
