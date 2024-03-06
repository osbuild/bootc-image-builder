package osbuild

import "fmt"

type SystemdPresetStageOptions struct {
	Presets []Preset `json:"presets,omitempty"`
}

type PresetState string

const (
	StateEnable  PresetState = "enable"
	StateDisable PresetState = "disable"
)

type Preset struct {
	Name  string      `json:"name,omitempty"`
	State PresetState `json:"state,omitempty"`
}

func (SystemdPresetStageOptions) isStageOptions() {}

func NewSystemdPresetStage(options *SystemdPresetStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    "org.osbuild.systemd.preset",
		Options: options,
	}
}

func (o SystemdPresetStageOptions) validate() error {
	if len(o.Presets) == 0 {
		return fmt.Errorf("at least one preset is required")
	}
	return nil
}

// GenServicesPresetStage creates a new systemd preset stage for the given
// list of services to enable and disable.
func GenServicesPresetStage(enabled, disabled []string) *Stage {
	if len(enabled) == 0 && len(disabled) == 0 {
		return nil
	}

	presets := make([]Preset, 0, len(enabled)+len(disabled))
	for _, name := range enabled {
		presets = append(presets, Preset{Name: name, State: StateEnable})
	}
	for _, name := range disabled {
		presets = append(presets, Preset{Name: name, State: StateDisable})
	}

	return NewSystemdPresetStage(&SystemdPresetStageOptions{Presets: presets})
}
