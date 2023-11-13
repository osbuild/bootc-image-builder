package osbuild

import (
	"encoding/json"
	"fmt"
)

type SystemdLogindStageOptions struct {
	Filename string                    `json:"filename"`
	Config   SystemdLogindConfigDropin `json:"config"`
}

func (SystemdLogindStageOptions) isStageOptions() {}

func NewSystemdLogindStage(options *SystemdLogindStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.systemd-logind",
		Options: options,
	}
}

// Drop-in configuration for systemd-logind
type SystemdLogindConfigDropin struct {
	Login SystemdLogindConfigLoginSection `json:"Login"`
}

// 'Login' configuration section - at least one option must be specified
type SystemdLogindConfigLoginSection struct {
	// Configures how many virtual terminals (VTs) to allocate by default
	// The option is optional, but zero is a valid value
	NAutoVTs *int `json:"NAutoVTs,omitempty"`
}

// Unexported alias for use in SystemdLogindConfigLoginSection's MarshalJSON() to prevent recursion
type systemdLogindConfigLoginSection SystemdLogindConfigLoginSection

func (s SystemdLogindConfigLoginSection) MarshalJSON() ([]byte, error) {
	if s.NAutoVTs == nil {
		return nil, fmt.Errorf("at least one 'Login' section option must be specified")
	}
	loginSection := systemdLogindConfigLoginSection(s)
	return json.Marshal(loginSection)
}
