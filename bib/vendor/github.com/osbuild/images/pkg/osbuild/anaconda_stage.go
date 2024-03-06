package osbuild

type AnacondaStageOptions struct {
	// Kickstart modules to enable
	KickstartModules []string `json:"kickstart-modules"`
}

func (AnacondaStageOptions) isStageOptions() {}

// Configure basic aspects of the Anaconda installer
func NewAnacondaStage(options *AnacondaStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.anaconda",
		Options: options,
	}
}

func NewAnacondaStageOptions(additionalModules []string) *AnacondaStageOptions {
	modules := []string{
		"org.fedoraproject.Anaconda.Modules.Network",
		"org.fedoraproject.Anaconda.Modules.Payloads",
		"org.fedoraproject.Anaconda.Modules.Storage",
	}

	if len(additionalModules) > 0 {
		modules = append(modules, additionalModules...)
	}

	return &AnacondaStageOptions{
		KickstartModules: modules,
	}
}
