package osbuild

// KernelCmdlineStageOptions describe how to create kernel-cmdline stage
//
// Configures the kernel boot parameters, also known as the kernel command line.
type KernelCmdlineStageOptions struct {
	RootFsUUID string `json:"root_fs_uuid,omitempty"`
	KernelOpts string `json:"kernel_opts,omitempty"`
}

func (KernelCmdlineStageOptions) isStageOptions() {}

// NewKernelCmdlineStage creates a new kernel-cmdline Stage object.
func NewKernelCmdlineStage(options *KernelCmdlineStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.kernel-cmdline",
		Options: options,
	}
}

func NewKernelCmdlineStageOptions(rootUUID string, kernelOptions string) *KernelCmdlineStageOptions {
	return &KernelCmdlineStageOptions{
		RootFsUUID: rootUUID,
		KernelOpts: kernelOptions,
	}
}
