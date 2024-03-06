package osbuild

import "fmt"

type OscapAutotailorStageOptions struct {
	Filepath string                `json:"filepath"`
	Config   OscapAutotailorConfig `json:"config"`
}

type OscapAutotailorConfig struct {
	NewProfile string   `json:"new_profile"`
	Datastream string   `json:"datastream" toml:"datastream"`
	ProfileID  string   `json:"profile_id" toml:"profile_id"`
	Selected   []string `json:"selected,omitempty"`
	Unselected []string `json:"unselected,omitempty"`
}

func (OscapAutotailorStageOptions) isStageOptions() {}

func (c OscapAutotailorConfig) validate() error {
	if c.Datastream == "" {
		return fmt.Errorf("'datastream' must be specified")
	}
	if c.ProfileID == "" {
		return fmt.Errorf("'profile_id' must be specified")
	}
	if c.NewProfile == "" {
		return fmt.Errorf("'new_profile' must be specified")
	}
	return nil
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

func NewOscapAutotailorStageOptions(filepath string, autotailorOptions OscapAutotailorConfig) *OscapAutotailorStageOptions {
	return &OscapAutotailorStageOptions{
		Filepath: filepath,
		Config: OscapAutotailorConfig{
			NewProfile: autotailorOptions.NewProfile,
			Datastream: autotailorOptions.Datastream,
			ProfileID:  autotailorOptions.ProfileID,
			Selected:   autotailorOptions.Selected,
			Unselected: autotailorOptions.Unselected,
		},
	}
}
