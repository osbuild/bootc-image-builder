package subscription

// The ImageOptions specify subscription-specific image options
// ServerUrl denotes the host to register the system with
// BaseUrl specifies the repository URL for DNF
type ImageOptions struct {
	Organization  string
	ActivationKey string
	ServerUrl     string
	BaseUrl       string
	Insights      bool
	Rhc           bool
}

type RHSMStatus string

const (
	RHSMConfigWithSubscription RHSMStatus = "with-subscription"
	RHSMConfigNoSubscription   RHSMStatus = "no-subscription"
)
