package osbuild

import (
	"encoding/json"
	"fmt"
)

type SshdConfigConfig struct {
	PasswordAuthentication          *bool                `json:"PasswordAuthentication,omitempty"`
	ChallengeResponseAuthentication *bool                `json:"ChallengeResponseAuthentication,omitempty"`
	ClientAliveInterval             *int                 `json:"ClientAliveInterval,omitempty"`
	PermitRootLogin                 PermitRootLoginValue `json:"PermitRootLogin,omitempty"`
}

// PermitRootLoginValue is defined to represent all valid types of the
// 'PermitRootLogin' item in the SshdConfigConfig structure.
type PermitRootLoginValue interface {
	isPermitRootLoginValue()
}

// PermitRootLoginValueStr represents a string type of the 'PermitRootLogin'
// item in the SshdConfigConfig structure.
type PermitRootLoginValueStr string

func (v PermitRootLoginValueStr) isPermitRootLoginValue() {}

// PermitRootLoginValueBool represents a bool type of the 'PermitRootLogin'
// item in the SshdConfigConfig structure.
type PermitRootLoginValueBool bool

func (v PermitRootLoginValueBool) isPermitRootLoginValue() {}

// Valid values which can be used for the 'PermitRootLogin' item in
// the SshdConfigConfig structure.
const (
	PermitRootLoginValueYes PermitRootLoginValueBool = true
	PermitRootLoginValueNo  PermitRootLoginValueBool = false

	PermitRootLoginValueProhibitPassword   PermitRootLoginValueStr = "prohibit-password"
	PermitRootLoginValueForcedCommandsOnly PermitRootLoginValueStr = "forced-commands-only"
)

// Unexported struct used for Unmarshalling of SshdConfigConfig due to
// 'PermitRootLogin' being a boolean or a string.
type rawSshdConfigConfig struct {
	PasswordAuthentication          *bool       `json:"PasswordAuthentication,omitempty"`
	ChallengeResponseAuthentication *bool       `json:"ChallengeResponseAuthentication,omitempty"`
	ClientAliveInterval             *int        `json:"ClientAliveInterval,omitempty"`
	PermitRootLogin                 interface{} `json:"PermitRootLogin,omitempty"`
}

func (c *SshdConfigConfig) UnmarshalJSON(data []byte) error {
	var rawConfig rawSshdConfigConfig
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return err
	}

	var permitRootLogin PermitRootLoginValue
	if rawConfig.PermitRootLogin != nil {
		switch valueType := rawConfig.PermitRootLogin.(type) {
		case bool:
			permitRootLogin = PermitRootLoginValueBool(rawConfig.PermitRootLogin.(bool))
		case string:
			permitRootLogin = PermitRootLoginValueStr(rawConfig.PermitRootLogin.(string))
		default:
			return fmt.Errorf("the 'PermitRootLogin' item has unsupported type %q", valueType)
		}
	}

	c.PasswordAuthentication = rawConfig.PasswordAuthentication
	c.ChallengeResponseAuthentication = rawConfig.ChallengeResponseAuthentication
	c.ClientAliveInterval = rawConfig.ClientAliveInterval
	c.PermitRootLogin = permitRootLogin

	return nil
}

type SshdConfigStageOptions struct {
	Config SshdConfigConfig `json:"config"`
}

func (SshdConfigStageOptions) isStageOptions() {}

func (o SshdConfigStageOptions) validate() error {
	if o.Config.PermitRootLogin != nil {
		value, ok := o.Config.PermitRootLogin.(PermitRootLoginValueStr)
		if ok {
			allowedPermitRootLoginStrValues := []PermitRootLoginValueStr{
				PermitRootLoginValueForcedCommandsOnly,
				PermitRootLoginValueProhibitPassword,
			}
			valid := false
			for _, validValue := range allowedPermitRootLoginStrValues {
				if value == validValue {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("%q is not a valid value for 'PermitRootLogin' option", value)
			}
		}
	}

	return nil
}

func NewSshdConfigStage(options *SshdConfigStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.sshd.config",
		Options: options,
	}
}
