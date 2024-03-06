package platform

import (
	"github.com/osbuild/images/pkg/arch"
)

type Aarch64 struct {
	BasePlatform
	UEFIVendor string
}

func (p *Aarch64) GetArch() arch.Arch {
	return arch.ARCH_AARCH64
}

func (p *Aarch64) GetUEFIVendor() string {
	return p.UEFIVendor
}

func (p *Aarch64) GetPackages() []string {
	packages := p.BasePlatform.FirmwarePackages

	if p.UEFIVendor != "" {
		packages = append(packages,
			"dracut-config-generic",
			"efibootmgr",
			"grub2-efi-aa64",
			"grub2-tools",
			"shim-aa64")
	}

	return packages
}

type Aarch64_Fedora struct {
	BasePlatform
	UEFIVendor string
	BootFiles  [][2]string
}

func (p *Aarch64_Fedora) GetArch() arch.Arch {
	return arch.ARCH_AARCH64
}

func (p *Aarch64_Fedora) GetUEFIVendor() string {
	return p.UEFIVendor
}

func (p *Aarch64_Fedora) GetPackages() []string {
	packages := p.BasePlatform.FirmwarePackages

	if p.UEFIVendor != "" {
		packages = append(packages,
			"dracut-config-generic",
			"efibootmgr",
			"grub2-efi-aa64",
			"grub2-tools",
			"shim-aa64")
	}

	return packages
}

func (p *Aarch64_Fedora) GetBootFiles() [][2]string {
	return p.BootFiles
}
