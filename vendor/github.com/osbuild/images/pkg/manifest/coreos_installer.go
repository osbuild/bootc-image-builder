package manifest

import (
	"fmt"

	"github.com/osbuild/images/internal/fdo"
	"github.com/osbuild/images/internal/ignition"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

type CoreOSInstaller struct {
	Base

	// Packages to install or exclude in addition to the ones required by the
	// pipeline.
	ExtraPackages   []string
	ExcludePackages []string

	// Extra repositories to install packages from
	ExtraRepos []rpmmd.RepoConfig

	platform     platform.Platform
	repos        []rpmmd.RepoConfig
	packageSpecs []rpmmd.PackageSpec
	kernelName   string
	kernelVer    string
	product      string
	version      string
	Variant      string

	// Biosdevname indicates whether or not biosdevname should be used to
	// name network devices when booting the installer. This may affect
	// the naming of network devices on the target system.
	Biosdevname bool

	FDO *fdo.Options

	// For the coreos-installer we only have EmbeddedOptions for ignition
	Ignition *ignition.EmbeddedOptions

	AdditionalDracutModules []string
}

// NewCoreOSInstaller creates an CoreOS installer pipeline object.
func NewCoreOSInstaller(m *Manifest,
	buildPipeline *Build,
	platform platform.Platform,
	repos []rpmmd.RepoConfig,
	kernelName,
	product,
	version string) *CoreOSInstaller {
	name := "coi-tree"
	p := &CoreOSInstaller{
		Base:       NewBase(m, name, buildPipeline),
		platform:   platform,
		repos:      filterRepos(repos, name),
		kernelName: kernelName,
		product:    product,
		version:    version,
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

// TODO: refactor:
// - what is required to boot and what to build?
// - do they all belong in this pipeline?
// - should these be moved to the platform for the image type?
func (p *CoreOSInstaller) getBootPackages() []string {
	packages := []string{
		"grub2-tools",
		"grub2-tools-extra",
		"grub2-tools-minimal",
		"efibootmgr",
	}

	packages = append(packages, p.platform.GetPackages()...)

	// TODO: Move these to the platform?
	// For Fedora, this will add a lot of duplicates, but we also add them here
	// for RHEL and CentOS.
	switch p.platform.GetArch() {
	case platform.ARCH_X86_64:
		packages = append(packages,
			"grub2-efi-x64",
			"grub2-efi-x64-cdboot",
			"grub2-pc",
			"grub2-pc-modules",
			"shim-x64",
			"syslinux",
			"syslinux-nonlinux",
		)
	case platform.ARCH_AARCH64:
		packages = append(packages,
			"grub2-efi-aa64-cdboot",
			"grub2-efi-aa64",
			"shim-aa64",
		)
	default:
		panic(fmt.Sprintf("unsupported arch: %s", p.platform.GetArch()))
	}

	if p.Biosdevname {
		packages = append(packages, "biosdevname")
	}

	return packages
}

func (p *CoreOSInstaller) getBuildPackages(Distro) []string {
	packages := p.getBootPackages()
	packages = append(packages,
		"rpm",
		"lorax-templates-generic",
	)
	return packages
}

func (p *CoreOSInstaller) getPackageSetChain(Distro) []rpmmd.PackageSet {
	packages := p.getBootPackages()
	return []rpmmd.PackageSet{
		{
			Include:         append(packages, p.ExtraPackages...),
			Exclude:         p.ExcludePackages,
			Repositories:    append(p.repos, p.ExtraRepos...),
			InstallWeakDeps: true,
		},
	}
}

func (p *CoreOSInstaller) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *CoreOSInstaller) serializeStart(packages []rpmmd.PackageSpec, _ []container.Spec, _ []ostree.CommitSpec) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = packages
	if p.kernelName != "" {
		p.kernelVer = rpmmd.GetVerStrFromPackageSpecListPanic(p.packageSpecs, p.kernelName)
	}
}

func (p *CoreOSInstaller) getInline() []string {
	inlineData := []string{}
	// inline data for FDO cert
	if p.FDO != nil && p.FDO.DiunPubKeyRootCerts != "" {
		inlineData = append(inlineData, p.FDO.DiunPubKeyRootCerts)
	}
	// inline data for ignition embedded (url or data)
	if p.Ignition != nil {
		if p.Ignition.Config != "" {
			inlineData = append(inlineData, p.Ignition.Config)
		}
	}
	return inlineData
}

func (p *CoreOSInstaller) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.kernelVer = ""
	p.packageSpecs = nil
}

func (p *CoreOSInstaller) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(p.repos), osbuild.NewRpmStageSourceFilesInputs(p.packageSpecs)))
	pipeline.AddStage(osbuild.NewBuildstampStage(&osbuild.BuildstampStageOptions{
		Arch:    p.platform.GetArch().String(),
		Product: p.product,
		Variant: p.Variant,
		Version: p.version,
		Final:   true,
	}))
	pipeline.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "C.UTF-8"}))

	dracutModules := append(
		p.AdditionalDracutModules,
		"coreos-installer",
		"fdo",
	)

	dracutStageOptions := dracutStageOptions(p.kernelVer, p.Biosdevname, dracutModules)
	if p.FDO != nil && p.FDO.DiunPubKeyRootCerts != "" {
		pipeline.AddStage(osbuild.NewFDOStageForRootCerts(p.FDO.DiunPubKeyRootCerts))
		dracutStageOptions.Install = []string{"/fdo_diun_pub_key_root_certs.pem"}
	}
	pipeline.AddStage(osbuild.NewDracutStage(dracutStageOptions))
	return pipeline
}

func (p *CoreOSInstaller) Platform() platform.Platform {
	return p.platform
}
