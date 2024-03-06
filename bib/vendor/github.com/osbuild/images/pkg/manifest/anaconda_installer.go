package manifest

import (
	"fmt"
	"os"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

type AnacondaInstallerType int

const (
	AnacondaInstallerTypeLive AnacondaInstallerType = iota + 1
	AnacondaInstallerTypePayload
)

// An Anaconda represents the installer tree as found on an ISO this can be either
// a payload installer or a live installer depending on `Type`.
type AnacondaInstaller struct {
	Base

	// The type of the Anaconda installer tree to prepare, this can be either
	// a 'live' or a 'payload' and it controls which stages are added to the
	// manifest.
	Type AnacondaInstallerType

	// Packages to install and/or exclude in addition to the ones required by the
	// pipeline.
	ExtraPackages   []string
	ExcludePackages []string

	// Extra repositories to install packages from
	ExtraRepos []rpmmd.RepoConfig

	// Users and Groups to create during installation.
	// If empty, then the user can interactively create users at install time.
	Users  []users.User
	Groups []users.Group

	// Biosdevname indicates whether or not biosdevname should be used to
	// name network devices when booting the installer. This may affect
	// the naming of network devices on the target system.
	Biosdevname bool

	// Variant is the variant of the product being installed, if applicable.
	Variant string

	platform     platform.Platform
	repos        []rpmmd.RepoConfig
	packageSpecs []rpmmd.PackageSpec
	kernelName   string
	kernelVer    string
	product      string
	version      string

	// Interactive defaults is a kickstart stage that can be provided, it
	// will be written to /usr/share/anaconda/interactive-defaults
	InteractiveDefaults *AnacondaInteractiveDefaults

	// Additional anaconda modules to enable
	AdditionalAnacondaModules []string

	// Additional dracut modules and drivers to enable
	AdditionalDracutModules []string
	AdditionalDrivers       []string

	Files []*fsnode.File

	// Temporary
	UseRHELLoraxTemplates bool
}

func NewAnacondaInstaller(installerType AnacondaInstallerType,
	buildPipeline Build,
	platform platform.Platform,
	repos []rpmmd.RepoConfig,
	kernelName,
	product,
	version string) *AnacondaInstaller {
	name := "anaconda-tree"
	p := &AnacondaInstaller{
		Base:       NewBase(name, buildPipeline),
		Type:       installerType,
		platform:   platform,
		repos:      filterRepos(repos, name),
		kernelName: kernelName,
		product:    product,
		version:    version,
	}
	buildPipeline.addDependent(p)
	return p
}

// TODO: refactor - what is required to boot and what to build, and
// do they all belong in this pipeline?
func (p *AnacondaInstaller) anacondaBootPackageSet() []string {
	packages := []string{
		"grub2-tools",
		"grub2-tools-extra",
		"grub2-tools-minimal",
		"efibootmgr",
	}

	switch p.platform.GetArch() {
	case arch.ARCH_X86_64:
		packages = append(packages,
			"grub2-efi-x64",
			"grub2-efi-x64-cdboot",
			"grub2-pc",
			"grub2-pc-modules",
			"shim-x64",
			"syslinux",
			"syslinux-nonlinux",
		)
	case arch.ARCH_AARCH64:
		packages = append(packages,
			"grub2-efi-aa64-cdboot",
			"grub2-efi-aa64",
			"shim-aa64",
		)
	default:
		panic(fmt.Sprintf("unsupported arch: %s", p.platform.GetArch()))
	}

	return packages
}

func (p *AnacondaInstaller) getBuildPackages(Distro) []string {
	packages := p.anacondaBootPackageSet()
	packages = append(packages,
		"rpm",
	)

	if p.UseRHELLoraxTemplates {
		packages = append(packages,
			"lorax-templates-rhel",
		)
	} else {
		packages = append(packages,
			"lorax-templates-generic",
		)
	}

	return packages
}

func (p *AnacondaInstaller) getPackageSetChain(Distro) []rpmmd.PackageSet {
	packages := p.anacondaBootPackageSet()
	if p.Biosdevname {
		packages = append(packages, "biosdevname")
	}
	return []rpmmd.PackageSet{
		{
			Include:         append(packages, p.ExtraPackages...),
			Exclude:         p.ExcludePackages,
			Repositories:    append(p.repos, p.ExtraRepos...),
			InstallWeakDeps: true,
		},
	}
}

func (p *AnacondaInstaller) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *AnacondaInstaller) serializeStart(packages []rpmmd.PackageSpec, _ []container.Spec, _ []ostree.CommitSpec) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = packages
	if p.kernelName != "" {
		p.kernelVer = rpmmd.GetVerStrFromPackageSpecListPanic(p.packageSpecs, p.kernelName)
	}
}

func (p *AnacondaInstaller) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.kernelVer = ""
	p.packageSpecs = nil
}

func installerRootUser() osbuild.UsersStageOptionsUser {
	return osbuild.UsersStageOptionsUser{
		Password: common.ToPtr(""),
	}
}

func (p *AnacondaInstaller) serialize() osbuild.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}

	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(p.repos), osbuild.NewRpmStageSourceFilesInputs(p.packageSpecs)))
	pipeline.AddStage(osbuild.NewBuildstampStage(&osbuild.BuildstampStageOptions{
		Arch:    p.platform.GetArch().String(),
		Product: p.product,
		Variant: p.Variant,
		Version: p.version,
		Final:   true,
	}))
	pipeline.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))

	// Let's do a bunch of sanity checks that are dependent on the installer type
	// being serialized
	switch p.Type {
	case AnacondaInstallerTypeLive:
		if len(p.Users) != 0 || len(p.Groups) != 0 {
			panic("anaconda installer type live does not support users and groups customization")
		}
		if p.InteractiveDefaults != nil {
			panic("anaconda installer type live does not support interactive defaults")
		}
		pipeline.AddStages(p.liveStages()...)
	case AnacondaInstallerTypePayload:
		pipeline.AddStages(p.payloadStages()...)
	default:
		panic("invalid anaconda installer type")
	}

	return pipeline
}

func (p *AnacondaInstaller) payloadStages() []*osbuild.Stage {
	stages := make([]*osbuild.Stage, 0)

	installUID := 0
	installGID := 0
	installHome := "/root"
	installShell := "/usr/libexec/anaconda/run-anaconda"
	installPassword := ""
	installUser := osbuild.UsersStageOptionsUser{
		UID:      &installUID,
		GID:      &installGID,
		Home:     &installHome,
		Shell:    &installShell,
		Password: &installPassword,
	}

	usersStageOptions := &osbuild.UsersStageOptions{
		Users: map[string]osbuild.UsersStageOptionsUser{
			"root":    installerRootUser(),
			"install": installUser,
		},
	}
	stages = append(stages, osbuild.NewUsersStage(usersStageOptions))

	var LoraxPath string

	if p.UseRHELLoraxTemplates {
		LoraxPath = "80-rhel/runtime-postinstall.tmpl"
	} else {
		LoraxPath = "99-generic/runtime-postinstall.tmpl"
	}

	stages = append(stages, osbuild.NewAnacondaStage(osbuild.NewAnacondaStageOptions(p.AdditionalAnacondaModules)))
	stages = append(stages, osbuild.NewLoraxScriptStage(&osbuild.LoraxScriptStageOptions{
		Path:     LoraxPath,
		BaseArch: p.platform.GetArch().String(),
	}))

	dracutModules := append(
		p.AdditionalDracutModules,
		"anaconda",
		"rdma",
		"rngd",
		"multipath",
		"fcoe",
		"fcoe-uefi",
		"iscsi",
		"lunmask",
		"nfs",
	)
	dracutOptions := dracutStageOptions(p.kernelVer, p.Biosdevname, dracutModules)
	dracutOptions.AddDrivers = p.AdditionalDrivers
	stages = append(stages, osbuild.NewDracutStage(dracutOptions))

	stages = append(stages, osbuild.NewSELinuxConfigStage(&osbuild.SELinuxConfigStageOptions{State: osbuild.SELinuxStatePermissive}))

	if p.InteractiveDefaults != nil {
		kickstartOptions, err := osbuild.NewKickstartStageOptionsWithLiveIMG(
			osbuild.KickstartPathInteractiveDefaults,
			p.Users,
			p.Groups,
			p.InteractiveDefaults.TarPath,
		)

		if err != nil {
			panic(fmt.Sprintf("failed to create kickstart stage options for interactive defaults: %v", err))
		}

		stages = append(stages, osbuild.NewKickstartStage(kickstartOptions))
	}

	return stages
}

func (p *AnacondaInstaller) liveStages() []*osbuild.Stage {
	stages := make([]*osbuild.Stage, 0)

	usersStageOptions := &osbuild.UsersStageOptions{
		Users: map[string]osbuild.UsersStageOptionsUser{
			"root": installerRootUser(),
		},
	}
	stages = append(stages, osbuild.NewUsersStage(usersStageOptions))

	systemdStageOptions := &osbuild.SystemdStageOptions{
		EnabledServices: []string{
			"livesys.service",
			"livesys-late.service",
		},
	}

	stages = append(stages, osbuild.NewSystemdStage(systemdStageOptions))

	livesysMode := os.FileMode(int(0644))
	livesysFile, err := fsnode.NewFile("/etc/sysconfig/livesys", &livesysMode, "root", "root", []byte("livesys_session=\"gnome\""))

	if err != nil {
		panic(err)
	}

	p.Files = []*fsnode.File{livesysFile}

	stages = append(stages, osbuild.GenFileNodesStages(p.Files)...)

	dracutModules := append(
		p.AdditionalDracutModules,
		"anaconda",
		"rdma",
		"rngd",
	)
	dracutOptions := dracutStageOptions(p.kernelVer, p.Biosdevname, dracutModules)
	dracutOptions.AddDrivers = p.AdditionalDrivers
	stages = append(stages, osbuild.NewDracutStage(dracutOptions))

	stages = append(stages, osbuild.NewSELinuxConfigStage(&osbuild.SELinuxConfigStageOptions{State: osbuild.SELinuxStatePermissive}))

	return stages
}

func dracutStageOptions(kernelVer string, biosdevname bool, additionalModules []string) *osbuild.DracutStageOptions {
	kernel := []string{kernelVer}
	modules := []string{
		"bash",
		"systemd",
		"fips",
		"systemd-initrd",
		"modsign",
		"nss-softokn",
		"i18n",
		"convertfs",
		"network-manager",
		"network",
		"ifcfg",
		"url-lib",
		"drm",
		"plymouth",
		"crypt",
		"dm",
		"dmsquash-live",
		"kernel-modules",
		"kernel-modules-extra",
		"kernel-network-modules",
		"livenet",
		"lvm",
		"mdraid",
		"qemu",
		"qemu-net",
		"resume",
		"rootfs-block",
		"terminfo",
		"udev-rules",
		"dracut-systemd",
		"pollcdrom",
		"usrmount",
		"base",
		"fs-lib",
		"img-lib",
		"shutdown",
		"uefi-lib",
	}

	if biosdevname {
		modules = append(modules, "biosdevname")
	}

	modules = append(modules, additionalModules...)
	return &osbuild.DracutStageOptions{
		Kernel:  kernel,
		Modules: modules,
		Install: []string{"/.buildstamp"},
	}
}

func (p *AnacondaInstaller) Platform() platform.Platform {
	return p.platform
}

type AnacondaInteractiveDefaults struct {
	TarPath string
}

func NewAnacondaInteractiveDefaults(tarPath string) *AnacondaInteractiveDefaults {
	i := &AnacondaInteractiveDefaults{
		TarPath: tarPath,
	}

	return i
}

func (p *AnacondaInstaller) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.Files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}
