package osbuild

import "fmt"

type OscapAutotailorStageOptions struct {
	Filepath string                `json:"filepath"`
	Config   OscapAutotailorConfig `json:"config"`
}
type OscapAutotailorConfig struct {
	OscapConfig
	NewProfile string   `json:"new_profile"`
	Selected   []string `json:"selected,omitempty"`
	Unselected []string `json:"unselected,omitempty"`
}

func (OscapAutotailorStageOptions) isStageOptions() {}

func (c OscapAutotailorConfig) validate() error {
	if c.NewProfile == "" {
		return fmt.Errorf("'new_profile' must be specified")
	}
	// reuse the oscap validation
	return c.OscapConfig.validate()
}

func NewOscapAutotailorStage(options *OscapAutotailorStageOptions) *Stage {
	if err := options.Config.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.oscap.autotailor",
		Options: options,
	}
}

func NewOscapAutotailorStageOptions(filepath string, oscapOptions OscapConfig, autotailorOptions OscapAutotailorConfig) *OscapAutotailorStageOptions {
	return &OscapAutotailorStageOptions{
		Filepath: filepath,
		Config: OscapAutotailorConfig{
			OscapConfig: oscapOptions,
			NewProfile:  autotailorOptions.NewProfile,
			Selected:    autotailorOptions.Selected,
			Unselected:  autotailorOptions.Unselected,
		},
	}
}
