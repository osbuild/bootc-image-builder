package osbuild

import (
	"encoding/json"
	"fmt"
)

// TunedStageOptions represents manually set TuneD profiles.
type TunedStageOptions struct {
	// List of TuneD profiles to apply.
	Profiles []string `json:"profiles"`
}

func (TunedStageOptions) isStageOptions() {}

// NewTunedStageOptions creates a new TuneD Stage options object.
func NewTunedStageOptions(profiles ...string) *TunedStageOptions {
	return &TunedStageOptions{
		Profiles: profiles,
	}
}

// Unexported alias for use in TunedStageOptions's MarshalJSON() to prevent recursion
type tunedStageOptions TunedStageOptions

func (o TunedStageOptions) MarshalJSON() ([]byte, error) {
	if len(o.Profiles) == 0 {
		return nil, fmt.Errorf("at least one Profile must be provided")
	}
	options := tunedStageOptions(o)
	return json.Marshal(options)
}

// NewTunedStage creates a new TuneD Stage object.
func NewTunedStage(options *TunedStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.tuned",
		Options: options,
	}
}
