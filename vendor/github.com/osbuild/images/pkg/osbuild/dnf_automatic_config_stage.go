package osbuild

import "fmt"

type DNFAutomaticUpgradeTypeValue string

// Valid values of the 'upgrade_type' option
const (
	DNFAutomaticUpgradeTypeDefault  DNFAutomaticUpgradeTypeValue = "default"
	DNFAutomaticUpgradeTypeSecurity DNFAutomaticUpgradeTypeValue = "security"
)

// DNFAutomaticConfigCommands represents the 'commands' configuration section.
type DNFAutomaticConfigCommands struct {
	// Whether packages comprising the available updates should be installed
	ApplyUpdates *bool `json:"apply_updates,omitempty"`
	// What kind of upgrades to look at
	UpgradeType DNFAutomaticUpgradeTypeValue `json:"upgrade_type,omitempty"`
}

// DNFAutomaticConfig represents DNF Automatic configuration.
type DNFAutomaticConfig struct {
	Commands *DNFAutomaticConfigCommands `json:"commands,omitempty"`
}

type DNFAutomaticConfigStageOptions struct {
	Config *DNFAutomaticConfig `json:"config,omitempty"`
}

func (DNFAutomaticConfigStageOptions) isStageOptions() {}

// NewDNFAutomaticConfigStageOptions creates a new DNFAutomaticConfig Stage options object.
func NewDNFAutomaticConfigStageOptions(config *DNFAutomaticConfig) *DNFAutomaticConfigStageOptions {
	return &DNFAutomaticConfigStageOptions{
		Config: config,
	}
}

func (o DNFAutomaticConfigStageOptions) validate() error {
	if o.Config != nil && o.Config.Commands != nil {
		allowedUpgradeTypeValues := []DNFAutomaticUpgradeTypeValue{
			DNFAutomaticUpgradeTypeDefault,
			DNFAutomaticUpgradeTypeSecurity,
			"", // default empty value when the option is not set
		}
		valid := false
		for _, value := range allowedUpgradeTypeValues {
			if o.Config.Commands.UpgradeType == value {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("'upgrade_type' option does not allow %q as a value", o.Config.Commands.UpgradeType)
		}
	}

	return nil
}

func NewDNFAutomaticConfigStage(options *DNFAutomaticConfigStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.dnf-automatic.config",
		Options: options,
	}
}
