package osbuild

// The SELinuxConfigStageOptions describe the desired SELinux policy state
// and type on the system.
type SELinuxConfigStageOptions struct {
	State SELinuxPolicyState `json:"state,omitempty"`
	Type  SELinuxPolicyType  `json:"type,omitempty"`
}

func (SELinuxConfigStageOptions) isStageOptions() {}

// Valid SELinux Policy states
type SELinuxPolicyState string

const (
	SELinuxStateEnforcing  SELinuxPolicyState = "enforcing"
	SELinuxStatePermissive SELinuxPolicyState = "permissive"
	SELinuxStateDisabled   SELinuxPolicyState = "disabled"
)

// Valid SELinux Policy types
type SELinuxPolicyType string

const (
	SELinuxTypeTargeted SELinuxPolicyType = "targeted"
	SELinuxTypeMinimum  SELinuxPolicyType = "minimum"
	SELinuxTypeMLS      SELinuxPolicyType = "mls"
)

// NewSELinuxConfigStage creates a new SELinuxConfig Stage object.
func NewSELinuxConfigStage(options *SELinuxConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.selinux.config",
		Options: options,
	}
}
