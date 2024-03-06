package osbuild

import "fmt"

type FirewallStageOptions struct {
	Ports            []string       `json:"ports,omitempty"`
	EnabledServices  []string       `json:"enabled_services,omitempty"`
	DisabledServices []string       `json:"disabled_services,omitempty"`
	DefaultZone      string         `json:"default_zone,omitempty"`
	Zones            []FirewallZone `json:"zones,omitempty"`
}

type FirewallZone struct {
	Name    string   `json:"name,omitempty"`
	Sources []string `json:"sources,omitempty"`
}

func (FirewallStageOptions) isStageOptions() {}

func (o FirewallStageOptions) validate() error {
	if len(o.Zones) != 0 {
		for _, fz := range o.Zones {
			if fz.Name == "" || len(fz.Sources) == 0 {
				return fmt.Errorf("items in firewall Zones cannot be empty, provide a non-empty 'Name' and a list of 'Sources'")
			}
		}
	}
	return nil
}

func NewFirewallStage(options *FirewallStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.firewall",
		Options: options,
	}
}
