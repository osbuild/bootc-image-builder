package workload

type Custom struct {
	BaseWorkload
	Packages         []string
	Services         []string
	DisabledServices []string
}

func (p *Custom) GetPackages() []string {
	return p.Packages
}

func (p *Custom) GetServices() []string {
	return p.Services
}

// TODO: Does this belong here? What kind of workload requires
// services to be disabled?
func (p *Custom) GetDisabledServices() []string {
	return p.DisabledServices
}
