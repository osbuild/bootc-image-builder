package osbuild

import "os"

// Options for the org.osbuild.ostree.config stage.
type MkdirStageOptions struct {
	Paths []MkdirStagePath `json:"paths"`
}

type MkdirStagePath struct {
	Path    string       `json:"path"`
	Mode    *os.FileMode `json:"mode,omitempty"`     // If not specified, the default mode is 0777
	Parents bool         `json:"parents,omitempty"`  // If true, create parent directories as needed
	ExistOk bool         `json:"exist_ok,omitempty"` // If true, do not fail if the target directory already exists
}

func (MkdirStageOptions) isStageOptions() {}

// NewMkdirStage creates a new org.osbuild.mkdir stage to create FS directories
func NewMkdirStage(options *MkdirStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkdir",
		Options: options,
	}
}
