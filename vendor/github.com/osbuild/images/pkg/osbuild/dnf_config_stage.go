package osbuild

import (
	"fmt"
)

// DNFConfigStageOptions represents persistent DNF configuration.
type DNFConfigStageOptions struct {
	// List of DNF variables.
	Variables []DNFVariable `json:"variables,omitempty"`
	Config    *DNFConfig    `json:"config,omitempty"`
}

func (DNFConfigStageOptions) isStageOptions() {}

// NewDNFConfigStageOptions creates a new DNFConfig Stage options object.
func NewDNFConfigStageOptions(variables []DNFVariable, config *DNFConfig) *DNFConfigStageOptions {
	return &DNFConfigStageOptions{
		Variables: variables,
		Config:    config,
	}
}

func (o DNFConfigStageOptions) validate() error {
	if o.Config != nil && o.Config.Main != nil {
		valid := false
		allowedIPR := []string{"4", "IPv4", "6", "IPv6", ""}
		for _, v := range allowedIPR {
			if o.Config.Main.IPResolve == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("DNF config parameter ip_resolve does not allow '%s' as a value", o.Config.Main.IPResolve)
		}
	}

	return nil
}

// NewDNFConfigStage creates a new DNFConfig Stage object.
func NewDNFConfigStage(options *DNFConfigStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.dnf.config",
		Options: options,
	}
}

// DNFVariable represents a single DNF variable.
type DNFVariable struct {
	// Name of the variable.
	Name string `json:"name"`
	// Value of the variable.
	Value string `json:"value"`
}

type DNFConfig struct {
	Main *DNFConfigMain `json:"main,omitempty"`
}

type DNFConfigMain struct {
	IPResolve string `json:"ip_resolve,omitempty"`
}
