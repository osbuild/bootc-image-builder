package osbuild

type PwqualityConfConfig struct {
	Minlen   *int `json:"minlen,omitempty"`
	Dcredit  *int `json:"dcredit,omitempty"`
	Ucredit  *int `json:"ucredit,omitempty"`
	Lcredit  *int `json:"lcredit,omitempty"`
	Ocredit  *int `json:"ocredit,omitempty"`
	Minclass *int `json:"minclass,omitempty"`
}

type PwqualityConfStageOptions struct {
	Config PwqualityConfConfig `json:"config"`
}

func (PwqualityConfStageOptions) isStageOptions() {}

func NewPwqualityConfStage(options *PwqualityConfStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.pwquality.conf",
		Options: options,
	}
}
