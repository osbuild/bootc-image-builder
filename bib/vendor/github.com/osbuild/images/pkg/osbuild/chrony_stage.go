package osbuild

type ChronyStageOptions struct {
	Servers   []ChronyConfigServer `json:"servers,omitempty"`
	LeapsecTz *string              `json:"leapsectz,omitempty"`
}

func (ChronyStageOptions) isStageOptions() {}

// Use '*ToPtr()' functions from the internal/common package to set the pointer values in literals
type ChronyConfigServer struct {
	Hostname string `json:"hostname"`
	Minpoll  *int   `json:"minpoll,omitempty"`
	Maxpoll  *int   `json:"maxpoll,omitempty"`
	Iburst   *bool  `json:"iburst,omitempty"`
	Prefer   *bool  `json:"prefer,omitempty"`
}

func NewChronyStage(options *ChronyStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.chrony",
		Options: options,
	}
}
