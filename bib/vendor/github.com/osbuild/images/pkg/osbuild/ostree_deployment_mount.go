package osbuild

type OSTreeMountOptions struct {
	Deployment OSTreeMountDeployment `json:"deployment"`
}

func (OSTreeMountOptions) isMountOptions() {}

type OSTreeMountDeployment struct {
	// Name of the stateroot to be used in the deployment
	OSName string `json:"osname"`

	// OStree ref to create and use for deployment
	Ref string `json:"ref"`

	// The deployment serial (usually '0')
	Serial *int `json:"serial,omitempty"`
}

func NewOSTreeDeploymentMount(name, osName, ref string, serial int) *Mount {
	return &Mount{
		Type: "org.osbuild.ostree.deployment",
		Name: name,
		Options: &OSTreeMountOptions{
			Deployment: OSTreeMountDeployment{
				OSName: osName,
				Ref:    ref,
				Serial: &serial,
			},
		},
	}
}
