package osbuild

import (
	"encoding/json"
	"fmt"
	"regexp"
)

const modprobeCfgFilenameRegex = "^[\\w.-]{1,250}\\.conf$"

type ModprobeStageOptions struct {
	Filename string                `json:"filename"`
	Commands ModprobeConfigCmdList `json:"commands"`
}

func (ModprobeStageOptions) isStageOptions() {}

func (o ModprobeStageOptions) validate() error {
	if len(o.Commands) == 0 {
		return fmt.Errorf("at least one command is required")
	}

	nameRegex := regexp.MustCompile(modprobeCfgFilenameRegex)
	if !nameRegex.MatchString(o.Filename) {
		return fmt.Errorf("modprobe configuration filename %q doesn't conform to schema (%s)", o.Filename, nameRegex.String())
	}

	return nil
}

func NewModprobeStage(options *ModprobeStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.modprobe",
		Options: options,
	}
}

type ModprobeConfigCmd interface {
	isModprobeConfigCmd()
}

// ModprobeConfigCmdList represents a modprobe configuration file, which contains
// a list of commands.
type ModprobeConfigCmdList []ModprobeConfigCmd

func (configFile *ModprobeConfigCmdList) UnmarshalJSON(data []byte) error {
	var rawConfigFile []interface{}

	if err := json.Unmarshal(data, &rawConfigFile); err != nil {
		return err
	}

	for _, rawConfigCmd := range rawConfigFile {
		var modprobeCmd ModprobeConfigCmd

		// The command object structure depends on the value of "command"
		// item, therefore make no assumptions on the structure.
		configCmdMap, ok := rawConfigCmd.(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected modprobe configuration file format")
		}
		command, ok := configCmdMap["command"].(string)
		if !ok {
			return fmt.Errorf("'command' item should be string, not %T", configCmdMap["command"])
		}

		switch command {
		case "blacklist":
			modulename, ok := configCmdMap["modulename"].(string)
			if !ok {
				return fmt.Errorf("'modulename' item should be string, not %T", configCmdMap["modulename"])
			}
			modprobeCmd = NewModprobeConfigCmdBlacklist(modulename)
		case "install":
			modulename, ok := configCmdMap["modulename"].(string)
			if !ok {
				return fmt.Errorf("'modulename' item should be string, not %T", configCmdMap["modulename"])
			}
			cmdline, ok := configCmdMap["cmdline"].(string)
			if !ok {
				return fmt.Errorf("'cmdline' item should be string, not %T", configCmdMap["cmdline"])
			}
			modprobeCmd = NewModprobeConfigCmdInstall(modulename, cmdline)
		default:
			return fmt.Errorf("unexpected modprobe command: %s", command)
		}

		*configFile = append(*configFile, modprobeCmd)
	}

	return nil
}

// ModprobeConfigCmdBlacklist represents the 'blacklist' command in the
// modprobe configuration.
type ModprobeConfigCmdBlacklist struct {
	Command    string `json:"command"`
	Modulename string `json:"modulename"`
}

func (ModprobeConfigCmdBlacklist) isModprobeConfigCmd() {}

func (c ModprobeConfigCmdBlacklist) validate() error {
	if c.Command != "blacklist" {
		return fmt.Errorf("'command' must have 'blacklist' value set")
	}
	if c.Modulename == "" {
		return fmt.Errorf("'modulename' must not be empty")
	}
	return nil
}

// NewModprobeConfigCmdBlacklist creates a new instance of ModprobeConfigCmdBlacklist
// for the provided modulename.
func NewModprobeConfigCmdBlacklist(modulename string) *ModprobeConfigCmdBlacklist {
	cmd := &ModprobeConfigCmdBlacklist{
		Command:    "blacklist",
		Modulename: modulename,
	}
	if err := cmd.validate(); err != nil {
		panic(err)
	}
	return cmd
}

// ModprobeConfigCmdInstall represents the 'install' command in the
// modprobe configuration.
type ModprobeConfigCmdInstall struct {
	Command    string `json:"command"`
	Modulename string `json:"modulename"`
	Cmdline    string `json:"cmdline"`
}

func (ModprobeConfigCmdInstall) isModprobeConfigCmd() {}

func (c ModprobeConfigCmdInstall) validate() error {
	if c.Command != "install" {
		return fmt.Errorf("'command' must have 'install' value set")
	}
	if c.Modulename == "" {
		return fmt.Errorf("'modulename' must not be empty")
	}
	if c.Cmdline == "" {
		return fmt.Errorf("'cmdline' must not be empty")
	}
	return nil
}

// NewModprobeConfigCmdInstall creates a new instance of ModprobeConfigCmdInstall
// for the provided modulename.
func NewModprobeConfigCmdInstall(modulename, cmdline string) *ModprobeConfigCmdInstall {
	cmd := &ModprobeConfigCmdInstall{
		Command:    "install",
		Modulename: modulename,
		Cmdline:    cmdline,
	}
	if err := cmd.validate(); err != nil {
		panic(err)
	}
	return cmd
}
