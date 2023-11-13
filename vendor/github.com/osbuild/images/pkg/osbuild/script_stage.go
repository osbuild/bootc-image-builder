package osbuild

// The ScriptStageOptions specifies a custom script to run in the image
type ScriptStageOptions struct {
	Script string `json:"script"`
}

func (ScriptStageOptions) isStageOptions() {}

// NewScriptStageOptions creates a new script stage options object, with
// the mandatory fields set.
func NewScriptStageOptions(script string) *ScriptStageOptions {
	return &ScriptStageOptions{
		Script: script,
	}
}

// NewScriptStage creates a new Script Stage object.
func NewScriptStage(options *ScriptStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.script",
		Options: options,
	}
}
