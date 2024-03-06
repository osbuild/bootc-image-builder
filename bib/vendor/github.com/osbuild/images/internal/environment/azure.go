package environment

type Azure struct {
	BaseEnvironment
}

func (p *Azure) GetPackages() []string {
	return []string{
		"cloud-init",
		"WALinuxAgent",
	}
}

func (p *Azure) GetServices() []string {
	return []string{
		"cloud-init.service",
		"cloud-config.service",
		"cloud-final.service",
		"cloud-init-local.service",
		"waagent",
	}
}
