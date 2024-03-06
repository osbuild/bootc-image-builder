package osbuild

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SysctldStageOptions represents a single sysctl.d configuration file.
type SysctldStageOptions struct {
	// Filename of the configuration file to be created. Must end with '.conf'.
	Filename string `json:"filename"`
	// List of configuration directives. The list must contain at least one item.
	Config []SysctldConfigLine `json:"config"`
}

func (SysctldStageOptions) isStageOptions() {}

// NewSysctldStageOptions creates a new PamLimitsConf Stage options object.
func NewSysctldStageOptions(filename string, config []SysctldConfigLine) *SysctldStageOptions {
	return &SysctldStageOptions{
		Filename: filename,
		Config:   config,
	}
}

// Unexported alias for use in SysctldStageOptions's MarshalJSON() to prevent recursion
type sysctldStageOptions SysctldStageOptions

func (o SysctldStageOptions) MarshalJSON() ([]byte, error) {
	if len(o.Config) == 0 {
		return nil, fmt.Errorf("the 'Config' list must contain at least one item")
	}
	options := sysctldStageOptions(o)
	return json.Marshal(options)
}

// NewSysctldStage creates a new Sysctld Stage object.
func NewSysctldStage(options *SysctldStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.sysctld",
		Options: options,
	}
}

// SysctldConfigLine represents a single line in a sysctl.d configuration.
type SysctldConfigLine struct {
	// Kernel parameter name.
	// If the string starts with "-" and the Value is not set,
	// then the key is excluded from being set by a matching glob.
	Key string `json:"key"`
	// Kernel parameter value.
	// Must be set, unless the Key value starts with "-".
	Value string `json:"value,omitempty"`
}

// Unexported alias for use in SysctldConfigLine's MarshalJSON() to prevent recursion.
type sysctldConfigLine SysctldConfigLine

func (l SysctldConfigLine) MarshalJSON() ([]byte, error) {
	if l.Value == "" && !strings.HasPrefix(l.Key, "-") {
		return nil, fmt.Errorf("only Keys starting with '-' can have an empty Value")
	}
	line := sysctldConfigLine(l)
	return json.Marshal(line)
}
