package osbuild

// The ZiplStageOptions describe how to create zipl stage
//
// The only configuration option available is a boot timeout and it is optional
type ZiplStageOptions struct {
	Timeout int `json:"timeout,omitempty"`
}

func (ZiplStageOptions) isStageOptions() {}

// NewZiplStageOptions creates a new ZiplStageOptions object with no timeout
func NewZiplStageOptions() *ZiplStageOptions {
	return &ZiplStageOptions{
		Timeout: 0,
	}
}

// NewZiplStage creates a new zipl Stage object.
func NewZiplStage(options *ZiplStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.zipl",
		Options: options,
	}
}
