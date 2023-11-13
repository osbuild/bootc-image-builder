package osbuild

import (
	"encoding/json"
	"fmt"
)

// TmpfilesdStageOptions represents a single tmpfiles.d configuration file.
type TmpfilesdStageOptions struct {
	// Filename of the configuration file to be created. Must end with '.conf'.
	Filename string `json:"filename"`
	// List of configuration directives. The list must contain at least one item.
	Config []TmpfilesdConfigLine `json:"config"`
}

func (TmpfilesdStageOptions) isStageOptions() {}

// NewTmpfilesdStageOptions creates a new Tmpfilesd Stage options object.
func NewTmpfilesdStageOptions(filename string, config []TmpfilesdConfigLine) *TmpfilesdStageOptions {
	return &TmpfilesdStageOptions{
		Filename: filename,
		Config:   config,
	}
}

// Unexported alias for use in TmpfilesdStageOptions's MarshalJSON() to prevent recursion
type tmpfilesdStageOptions TmpfilesdStageOptions

func (o TmpfilesdStageOptions) MarshalJSON() ([]byte, error) {
	if len(o.Config) == 0 {
		return nil, fmt.Errorf("the 'Config' list must contain at least one item")
	}
	options := tmpfilesdStageOptions(o)
	return json.Marshal(options)
}

// NewTmpfilesdStage creates a new Tmpfilesd Stage object.
func NewTmpfilesdStage(options *TmpfilesdStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.tmpfilesd",
		Options: options,
	}
}

// TmpfilesdConfigLine represents a single line in a tmpfiles.d configuration.
type TmpfilesdConfigLine struct {
	// The file system path type
	Type string `json:"type"`
	// Absolute file system path
	Path string `json:"path"`
	// The file access mode when creating the file or directory
	Mode string `json:"mode,omitempty"`
	// The user to use for the file or directory
	User string `json:"user,omitempty"`
	// The group to use for the file or directory
	Group string `json:"group,omitempty"`
	// Date field used to decide what files to delete when cleaning
	Age string `json:"age,omitempty"`
	// Argument with its meaning being specific to the path type
	Argument string `json:"argument,omitempty"`
}
