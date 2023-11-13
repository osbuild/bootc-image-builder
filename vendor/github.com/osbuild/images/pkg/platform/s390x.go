package platform

type S390X struct {
	BasePlatform
	Zipl bool
}

func (p *S390X) GetArch() Arch {
	return ARCH_S390X
}

func (p *S390X) GetZiplSupport() bool {
	return p.Zipl
}

func (p *S390X) GetPackages() []string {
	packages := p.BasePlatform.FirmwarePackages

	if p.Zipl {
		packages = append(packages,
			"dracut-config-generic",
			"s390utils-base",
			"s390utils-core",
		)
	}

	return packages
}

func (p *S390X) GetBuildPackages() []string {
	packages := []string{}

	if p.Zipl {
		packages = append(packages, "s390utils-base")
	}

	return packages
}
