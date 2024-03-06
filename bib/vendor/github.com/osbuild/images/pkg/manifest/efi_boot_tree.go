package manifest

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
)

type EFIBootTree struct {
	Base

	Platform platform.Platform

	product string
	version string

	UEFIVendor string
	ISOLabel   string

	KernelOpts []string
}

func NewEFIBootTree(buildPipeline Build, product, version string) *EFIBootTree {
	p := &EFIBootTree{
		Base:    NewBase("efiboot-tree", buildPipeline),
		product: product,
		version: version,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *EFIBootTree) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	a := p.Platform.GetArch().String()
	var architectures []string
	if a == arch.ARCH_X86_64.String() {
		architectures = []string{"X64"}
	} else if a == arch.ARCH_AARCH64.String() {
		architectures = []string{"AA64"}
	} else {
		panic("unsupported architecture")
	}

	grubOptions := &osbuild.GrubISOStageOptions{
		Product: osbuild.Product{
			Name:    p.product,
			Version: p.version,
		},
		Kernel: osbuild.ISOKernel{
			Dir:  "/images/pxeboot",
			Opts: p.KernelOpts,
		},
		ISOLabel:      p.ISOLabel,
		Architectures: architectures,
		Vendor:        p.UEFIVendor,
	}
	grub2Stage := osbuild.NewGrubISOStage(grubOptions)
	pipeline.AddStage(grub2Stage)
	return pipeline
}
