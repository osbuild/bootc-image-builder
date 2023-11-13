package osbuild

// RHSMStageOptions describes configuration of the RHSM stage.
//
// The RHSM stage allows configuration of Red Hat Subscription Manager (RHSM)
// related components. Currently it allows only configuration of the enablement
// state of DNF plugins used by the Subscription Manager.
type RHSMStageOptions struct {
	YumPlugins *RHSMStageOptionsDnfPlugins `json:"yum-plugins,omitempty"`
	DnfPlugins *RHSMStageOptionsDnfPlugins `json:"dnf-plugins,omitempty"`
	SubMan     *RHSMStageOptionsSubMan     `json:"subscription-manager,omitempty"`
}

func (RHSMStageOptions) isStageOptions() {}

// NewRHSMStage creates a new RHSM stage
func NewRHSMStage(options *RHSMStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.rhsm",
		Options: options,
	}
}

// RHSMStageOptionsDnfPlugins describes configuration of all RHSM DNF plugins
type RHSMStageOptionsDnfPlugins struct {
	ProductID           *RHSMStageOptionsDnfPlugin `json:"product-id,omitempty"`
	SubscriptionManager *RHSMStageOptionsDnfPlugin `json:"subscription-manager,omitempty"`
}

// RHSMStageOptionsDnfPlugin describes configuration of a specific RHSM DNF
// plugin
//
// Only the enablement state of a DNF plugin can be currenlty  set.
type RHSMStageOptionsDnfPlugin struct {
	Enabled bool `json:"enabled"`
}

// Subscription-manager configuration (/etc/rhsm/rhsm.conf)
type RHSMStageOptionsSubMan struct {
	Rhsm      *SubManConfigRHSMSection      `json:"rhsm,omitempty"`
	Rhsmcertd *SubManConfigRHSMCERTDSection `json:"rhsmcertd,omitempty"`
}

// RHSM configuration section of /etc/rhsm/rhsm.conf
type SubManConfigRHSMSection struct {
	// Whether subscription-manager should manage DNF repos file
	ManageRepos *bool `json:"manage_repos,omitempty"`
}

// RHSMCERTD configuration section of /etc/rhsm/rhsm.conf
type SubManConfigRHSMCERTDSection struct {
	// Automatic system registration
	AutoRegistration *bool `json:"auto_registration,omitempty"`
}
