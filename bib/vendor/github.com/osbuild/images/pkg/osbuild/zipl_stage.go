package osbuild

// The ZiplStageOptions describe how to create zipl stage
//
// The only configuration option available is a boot timeout and it is optional
type ZiplStageOptions struct {
	Timeout int `json:"timeout,omitempty"`
}

func (ZiplStageOptions) isStageOptions() {}

// NewZiplStage creates a new zipl Stage object.
func NewZiplStage(options *ZiplStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.zipl",
		Options: options,
	}
}
