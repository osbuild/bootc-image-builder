package osbuild

type UpdateCryptoPoliciesStageOptions struct {
	Policy string `json:"policy"`
}

func (UpdateCryptoPoliciesStageOptions) isStageOptions() {}

func NewUpdateCryptoPoliciesStage(options *UpdateCryptoPoliciesStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.update-crypto-policies",
		Options: options,
	}
}
