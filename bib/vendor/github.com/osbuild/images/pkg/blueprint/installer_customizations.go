package blueprint

type InstallerCustomization struct {
	Unattended        bool `json:"unattended,omitempty" toml:"unattended,omitempty"`
	WheelSudoNopasswd bool `json:"wheel-sudo-nopasswd,omitempty" toml:"wheel-sudo-nopasswd,omitempty"`
}
