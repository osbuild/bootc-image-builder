package osbuild

type NginxConfigStageOptions struct {
	// Config file location
	Path string `json:"path,omitempty"`

	Config *NginxConfig `json:"config,omitempty"`
}

func (NginxConfigStageOptions) isStageOptions() {}

type NginxConfig struct {
	// The address and/or port on which the server will accept requests
	Listen string `json:"listen,omitempty"`

	// The root directory for requests
	Root string `json:"root,omitempty"`

	// File that will store the process ID of the main process
	PID string `json:"pid,omitempty"`

	// Whether nginx should become a daemon
	Daemon *bool `json:"daemon,omitempty"`
}

// NewNingxConfigStage creates a new org.osbuild.nginxconfig stage
func NewNginxConfigStage(options *NginxConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.nginx.conf",
		Options: options,
	}
}
