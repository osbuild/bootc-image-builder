package environment

type EC2 struct {
	BaseEnvironment
}

func (p *EC2) GetPackages() []string {
	return []string{"cloud-init"}
}

func (p *EC2) GetServices() []string {
	return []string{
		"cloud-init.service",
		"cloud-config.service",
		"cloud-final.service",
		"cloud-init-local.service",
	}
}
