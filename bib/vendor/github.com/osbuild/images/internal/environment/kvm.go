package environment

type KVM struct {
	BaseEnvironment
}

func (e *KVM) GetPackages() []string {
	return []string{
		"cloud-init",
		"qemu-guest-agent",
	}
}

func (e *KVM) GetServices() []string {
	return []string{
		"cloud-init.service",
		"cloud-config.service",
		"cloud-final.service",
		"cloud-init-local.service",
	}
}
