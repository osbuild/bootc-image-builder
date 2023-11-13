package osbuild

type AuthconfigStageOptions struct {
}

func (AuthconfigStageOptions) isStageOptions() {}

func NewAuthconfigStage(options *AuthconfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.authconfig",
		Options: options,
	}
}
