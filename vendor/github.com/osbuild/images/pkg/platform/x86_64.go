package platform

type X86BootLoader uint64

type X86 struct {
	BasePlatform
	BIOS       bool
	UEFIVendor string
}

func (p *X86) GetArch() Arch {
	return ARCH_X86_64
}

func (p *X86) GetBIOSPlatform() string {
	if p.BIOS {
		return "i386-pc"
	}
	return ""
}

func (p *X86) GetUEFIVendor() string {
	return p.UEFIVendor
}

func (p *X86) GetPackages() []string {
	packages := p.BasePlatform.FirmwarePackages

	if p.BIOS {
		packages = append(packages,
			"dracut-config-generic",
			"grub2-pc")
	}

	if p.UEFIVendor != "" {
		packages = append(packages,
			"dracut-config-generic",
			"efibootmgr",
			"grub2-efi-x64",
			"shim-x64")
	}

	return packages
}

func (p *X86) GetBuildPackages() []string {
	packages := []string{}
	if p.BIOS {
		packages = append(packages, "grub2-pc")
	}
	return packages
}
