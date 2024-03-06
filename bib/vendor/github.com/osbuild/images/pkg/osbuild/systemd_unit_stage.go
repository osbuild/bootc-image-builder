package osbuild

type unitType string

const (
	System unitType = "system"
	Global unitType = "global"
)

type SystemdUnitStageOptions struct {
	Unit     string                   `json:"unit"`
	Dropin   string                   `json:"dropin"`
	Config   SystemdServiceUnitDropin `json:"config"`
	UnitType unitType                 `json:"unit-type,omitempty"`
}

func (SystemdUnitStageOptions) isStageOptions() {}

func NewSystemdUnitStage(options *SystemdUnitStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.systemd.unit",
		Options: options,
	}
}

// Drop-in configuration for a '.service' unit
type SystemdServiceUnitDropin struct {
	Service *SystemdUnitServiceSection `json:"Service,omitempty"`
	Unit    *SystemdUnitSection        `json:"Unit,omitempty"`
}

// 'Service' configuration section of a unit file
type SystemdUnitServiceSection struct {
	// Sets environment variables for executed process
	Environment string `json:"Environment,omitempty"`
}

// 'Unit' configuration section of a unit file
type SystemdUnitSection struct {
	// Sets condition to to check if file exits
	FileExists string `json:"ConditionPathExists,omitempty"`
}
