package osbuild

import (
	"encoding/json"
	"fmt"
)

// Options for the org.osbuild.ostree.deploy stage.
type OSTreeDeployStageOptions struct {
	OsName string `json:"osname"`

	Ref string `json:"ref"`

	Remote string `json:"remote,omitempty"`

	Mounts []string `json:"mounts"`

	Rootfs Rootfs `json:"rootfs"`

	KernelOpts []string `json:"kernel_opts"`
}

type Rootfs struct {
	// Identify the root file system by label
	Label string `json:"label,omitempty"`

	// Identify the root file system by UUID
	UUID string `json:"uuid,omitempty"`
}

func (OSTreeDeployStageOptions) isStageOptions() {}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeDeployStage(options *OSTreeDeployStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.deploy",
		Options: options,
	}
}

// alias for custom marshaller
type ostreeDeployStageOptions OSTreeDeployStageOptions

func (options OSTreeDeployStageOptions) MarshalJSON() ([]byte, error) {
	rootfs := options.Rootfs
	if (len(rootfs.UUID) == 0 && len(rootfs.Label) == 0) || (len(rootfs.UUID) != 0 && len(rootfs.Label) != 0) {
		return nil, fmt.Errorf("exactly one of UUID or Label must be specified")
	}

	aliasOptions := ostreeDeployStageOptions(options)
	return json.Marshal(aliasOptions)
}
