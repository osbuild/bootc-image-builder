package osbuild

type Mount struct {
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Source    string       `json:"source,omitempty"`
	Target    string       `json:"target,omitempty"`
	Options   MountOptions `json:"options,omitempty"`
	Partition *int         `json:"partition,omitempty"`
}

type MountOptions interface {
	isMountOptions()
}
