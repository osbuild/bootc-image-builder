package osbuild

import (
	"fmt"
)

type CloudInitStageOptions struct {
	Filename string              `json:"filename"`
	Config   CloudInitConfigFile `json:"config"`
}

func (CloudInitStageOptions) isStageOptions() {}

func NewCloudInitStage(options *CloudInitStageOptions) *Stage {
	if err := options.Config.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    "org.osbuild.cloud-init",
		Options: options,
	}
}

// Represents a cloud-init configuration file
type CloudInitConfigFile struct {
	SystemInfo     *CloudInitConfigSystemInfo `json:"system_info,omitempty"`
	Reporting      *CloudInitConfigReporting  `json:"reporting,omitempty"`
	Datasource     *CloudInitConfigDatasource `json:"datasource,omitempty"`
	DatasourceList []string                   `json:"datasource_list,omitempty"`
	Output         *CloudInitConfigOutput     `json:"output,omitempty"`
}

// Represents the 'system_info' configuration section
type CloudInitConfigSystemInfo struct {
	DefaultUser *CloudInitConfigDefaultUser `json:"default_user,omitempty"`
}

// Represents the 'reporting' configuration section
type CloudInitConfigReporting struct {
	Logging   *CloudInitConfigReportingHandlers `json:"logging,omitempty"`
	Telemetry *CloudInitConfigReportingHandlers `json:"telemetry,omitempty"`
}

type CloudInitConfigReportingHandlers struct {
	Type string `json:"type"`
}

// Represents the 'datasource' configuration section
type CloudInitConfigDatasource struct {
	Azure *CloudInitConfigDatasourceAzure `json:"Azure,omitempty"`
}

type CloudInitConfigDatasourceAzure struct {
	ApplyNetworkConfig bool `json:"apply_network_config"`
}

// Represents the 'output' configuration section
type CloudInitConfigOutput struct {
	Init   *string `json:"init,omitempty"`
	Config *string `json:"config,omitempty"`
	Final  *string `json:"final,omitempty"`
	All    *string `json:"all,omitempty"`
}

// Configuration of the 'default' user created by cloud-init.
type CloudInitConfigDefaultUser struct {
	Name string `json:"name,omitempty"`
}

func (c CloudInitConfigFile) validate() error {
	if c.SystemInfo == nil && c.Reporting == nil && c.Datasource == nil && len(c.DatasourceList) == 0 && c.Output == nil {
		return fmt.Errorf("at least one cloud-init configuration option must be specified")
	}
	if c.SystemInfo != nil {
		if err := c.SystemInfo.validate(); err != nil {
			return err
		}
	}
	if c.Reporting != nil {
		if err := c.Reporting.validate(); err != nil {
			return err
		}
	}
	if c.Datasource != nil {
		if err := c.Datasource.validate(); err != nil {
			return err
		}
	}
	if len(c.DatasourceList) > 0 {
		for _, d := range c.DatasourceList {
			if d != "Azure" {
				return fmt.Errorf("only 'Azure' is allowed as an item in the datasource_list")
			}
		}
	}
	if c.Output != nil {
		if err := c.Output.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (si CloudInitConfigSystemInfo) validate() error {
	if si.DefaultUser == nil {
		return fmt.Errorf("at least one configuration option must be specified for 'system_info' section")
	} else {
		return si.DefaultUser.validate()
	}
}

func (r CloudInitConfigReporting) validate() error {
	if r.Logging == nil && r.Telemetry == nil {
		return fmt.Errorf("at least one configuration option must be specified for 'reporting' section")
	}
	if r.Logging != nil {
		if err := r.Logging.validate(); err != nil {
			return err
		}
	}
	if r.Telemetry != nil {
		if err := r.Telemetry.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (r CloudInitConfigReportingHandlers) validate() error {
	allowed_values := []string{"log", "print", "webhook", "hyperv"}
	for _, v := range allowed_values {
		if v == r.Type {
			return nil
		}
	}
	return fmt.Errorf("reporting parameters must be one of 'log', 'print', 'webhook', 'hyperv'")
}

func (d CloudInitConfigDatasource) validate() error {
	if d.Azure == nil {
		return fmt.Errorf("at least one configuration option must be specified for 'datasource' section")
	}
	return nil
}

func (o CloudInitConfigOutput) validate() error {
	if o.Init == nil && o.Config == nil && o.Final == nil && o.All == nil {
		return fmt.Errorf("at least one configuration option must be specified for 'output' section")
	}
	return nil
}

func (du CloudInitConfigDefaultUser) validate() error {
	if du.Name == "" {
		return fmt.Errorf("at least one configuration option must be specified for 'default_user' section")
	}
	return nil
}
