package osbuild

type AuthselectStageOptions struct {
	Profile  string   `json:"profile"`
	Features []string `json:"features,omitempty"`
}

func (AuthselectStageOptions) isStageOptions() {}

func NewAuthselectStage(options *AuthselectStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.authselect",
		Options: options,
	}
}
