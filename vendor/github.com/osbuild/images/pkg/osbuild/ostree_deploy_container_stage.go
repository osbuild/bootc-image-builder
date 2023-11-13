package osbuild

import (
	"fmt"
	"regexp"
)

const ostreeContainerTargetImgrefRegex = "^(ostree-remote-registry|ostree-image-signed|ostree-unverified-registry):.*$"

// Options for the org.osbuild.ostree.deploy.container stage.
type OSTreeDeployContainerStageOptions struct {

	// Name of the stateroot to be used in the deployment
	OsName string `json:"osname"`

	// Additional kernel command line options
	KernelOpts []string `json:"kernel_opts,omitempty"`

	// Image ref used as the source of truth for updates
	TargetImgref string `json:"target_imgref"`

	// Identifier to locate the root file system (uuid or label)
	Rootfs *Rootfs `json:"rootfs,omitempty"`

	// Mount points of the final file system
	Mounts []string `json:"mounts,omitempty"`
}

func (OSTreeDeployContainerStageOptions) isStageOptions() {}

func (options OSTreeDeployContainerStageOptions) validate() error {
	exp := regexp.MustCompile(ostreeContainerTargetImgrefRegex)
	if !exp.MatchString(options.TargetImgref) {
		return fmt.Errorf("'target_imgref' %q doesn't conform to schema (%s)", options.TargetImgref, exp.String())
	}
	return nil
}

type OSTreeDeployContainerInputs struct {
	Images ContainersInput `json:"images"`
}

func (OSTreeDeployContainerInputs) isStageInputs() {}

func (inputs OSTreeDeployContainerInputs) validate() error {
	if ncontainers := inputs.Images.References.Len(); ncontainers != 1 {
		return fmt.Errorf("stage requires exactly 1 input container (got %d)", ncontainers)
	}
	return nil
}

// A new org.osbuild.ostree.init stage to create an OSTree repository
func NewOSTreeDeployContainerStage(options *OSTreeDeployContainerStageOptions, images ContainersInput) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	inputs := OSTreeDeployContainerInputs{
		Images: images,
	}
	if err := inputs.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    "org.osbuild.ostree.deploy.container",
		Options: options,
		Inputs:  inputs,
	}
}
