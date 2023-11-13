package osbuild

import (
	"fmt"
)

type GcpGuestAgentConfigScopeValue string

const (
	GcpGuestAgentConfigScopeDistro   GcpGuestAgentConfigScopeValue = "distro"
	GcpGuestAgentConfigScopeInstance GcpGuestAgentConfigScopeValue = "instance"
)

type GcpGuestAgentConfigAccounts struct {
	DeprovisionRemove *bool    `json:"deprovision_remove,omitempty"`
	Groups            []string `json:"groups,omitempty"`
	UseraddCmd        string   `json:"useradd_cmd,omitempty"`
	UserdelCmd        string   `json:"userdel_cmd,omitempty"`
	UsermodCmd        string   `json:"usermod_cmd,omitempty"`
	GpasswdAddCmd     string   `json:"gpasswd_add_cmd,omitempty"`
	GpasswdRemoveCmd  string   `json:"gpasswd_remove_cmd,omitempty"`
	GroupaddCmd       string   `json:"groupadd_cmd,omitempty"`
}

type GcpGuestAgentConfigDaemons struct {
	AccountsDaemon  *bool `json:"accounts_daemon,omitempty"`
	ClockSkewDaemon *bool `json:"clock_skew_daemon,omitempty"`
	NetworkDaemon   *bool `json:"network_daemon,omitempty"`
}

type GcpGuestAgentConfigInstanceSetup struct {
	HostKeyTypes     []string `json:"host_key_types,omitempty"`
	OptimizeLocalSsd *bool    `json:"optimize_local_ssd,omitempty"`
	NetworkEnabled   *bool    `json:"network_enabled,omitempty"`
	SetBotoConfig    *bool    `json:"set_boto_config,omitempty"`
	SetHostKeys      *bool    `json:"set_host_keys,omitempty"`
	SetMultiqueue    *bool    `json:"set_multiqueue,omitempty"`
}

type GcpGuestAgentConfigIpForwarding struct {
	EthernetProtoId   string `json:"ethernet_proto_id,omitempty"`
	IpAliases         *bool  `json:"ip_aliases,omitempty"`
	TargetInstanceIps *bool  `json:"target_instance_ips,omitempty"`
}

type GcpGuestAgentConfigMetadataScripts struct {
	DefaultShell string `json:"default_shell,omitempty"`
	RunDir       string `json:"run_dir,omitempty"`
	Startup      *bool  `json:"startup,omitempty"`
	Shutdown     *bool  `json:"shutdown,omitempty"`
}

type GcpGuestAgentConfigNetworkInterfaces struct {
	Setup        *bool  `json:"setup,omitempty"`
	IpForwarding *bool  `json:"ip_forwarding,omitempty"`
	DhcpCommand  string `json:"dhcp_command,omitempty"`
}

type GcpGuestAgentConfig struct {
	Accounts          *GcpGuestAgentConfigAccounts          `json:"Accounts,omitempty"`
	Daemons           *GcpGuestAgentConfigDaemons           `json:"Daemons,omitempty"`
	InstanceSetup     *GcpGuestAgentConfigInstanceSetup     `json:"InstanceSetup,omitempty"`
	IpForwarding      *GcpGuestAgentConfigIpForwarding      `json:"IpForwarding,omitempty"`
	MetadataScripts   *GcpGuestAgentConfigMetadataScripts   `json:"MetadataScripts,omitempty"`
	NetworkInterfaces *GcpGuestAgentConfigNetworkInterfaces `json:"NetworkInterfaces,omitempty"`
}

type GcpGuestAgentConfigOptions struct {
	ConfigScope GcpGuestAgentConfigScopeValue `json:"config_scope,omitempty"`
	Config      *GcpGuestAgentConfig          `json:"config"`
}

func (GcpGuestAgentConfigOptions) isStageOptions() {}

func (o GcpGuestAgentConfigOptions) validate() error {
	allowedScopeValues := []GcpGuestAgentConfigScopeValue{
		GcpGuestAgentConfigScopeDistro,
		GcpGuestAgentConfigScopeInstance,
		"", // default empty value when the option is not set
	}

	valid := false
	for _, value := range allowedScopeValues {
		if o.ConfigScope == value {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf(
			"scope %q doesn't conform to schema (%s). Must be one of [distro, instance]",
			o.ConfigScope,
			repoFilenameRegex,
		)
	}

	if o.Config == nil {
		return fmt.Errorf("config property is required")
	}

	if (GcpGuestAgentConfig{}) == *o.Config {
		return fmt.Errorf("at least one config section must be defined")
	}

	return nil
}

func NewGcpGuestAgentConfigStage(options *GcpGuestAgentConfigOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.gcp.guest-agent.conf",
		Options: options,
	}
}
