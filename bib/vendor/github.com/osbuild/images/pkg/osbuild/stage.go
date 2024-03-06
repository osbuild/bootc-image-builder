package osbuild

// Single stage of a pipeline executing one step
type Stage struct {
	// Well-known name in reverse domain-name notation, uniquely identifying
	// the stage type.
	Type string `json:"type"`
	// Stage-type specific options fully determining the operations of the

	Inputs  Inputs            `json:"inputs,omitempty"`
	Options StageOptions      `json:"options,omitempty"`
	Devices map[string]Device `json:"devices,omitempty"`
	Mounts  []Mount           `json:"mounts,omitempty"`
}

// StageOptions specify the operations of a given stage-type.
type StageOptions interface {
	isStageOptions()
}

// MountOSTree adds an ostree mount to a stage which makes it run in a deployed
// ostree stateroot.
func (s *Stage) MountOSTree(osName, ref string, serial int) {
	name := "ostree-" + ref
	ostreeMount := NewOSTreeDeploymentMount(name, osName, ref, serial)
	s.Mounts = append(s.Mounts, *ostreeMount)
}
