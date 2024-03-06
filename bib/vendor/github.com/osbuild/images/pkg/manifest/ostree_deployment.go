package manifest

import (
	"fmt"
	"os"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

// OSTreeDeployment represents the filesystem tree of a target image based
// on a deployed ostree commit.
type OSTreeDeployment struct {
	Base

	Remote ostree.Remote

	OSVersion string

	// commitSource represents the source that will be used to retrieve the
	// ostree commit for this pipeline.
	commitSource *ostree.SourceSpec

	// ostreeSpec is the resolved commit that will be deployed in this pipeline.
	ostreeSpec *ostree.CommitSpec

	// containerSource represents the source that will be used to retrieve the
	// ostree native container for this pipeline.
	containerSource *container.SourceSpec

	// containerSpec is the resolved ostree native container that will be
	// deployed in this pipeline.
	containerSpec *container.Spec

	SysrootReadOnly bool

	osName string
	ref    string

	KernelOptionsAppend []string
	Keyboard            string
	Locale              string

	Users  []users.User
	Groups []users.Group

	platform platform.Platform

	PartitionTable *disk.PartitionTable

	// Specifies the ignition platform to use.
	// If empty, ignition is not enabled.
	IgnitionPlatform string

	Directories []*fsnode.Directory
	Files       []*fsnode.File

	EnabledServices  []string
	DisabledServices []string

	FIPS bool

	// Lock the root account in the deployment unless the user defined root
	// user options in the build configuration.
	LockRoot bool

	// Use bootupd instead of grub2 as the bootloader
	UseBootupd bool
}

// NewOSTreeCommitDeployment creates a pipeline for an ostree deployment from a
// commit.
func NewOSTreeCommitDeployment(buildPipeline Build,
	commit *ostree.SourceSpec,
	osName string,
	platform platform.Platform) *OSTreeDeployment {

	p := &OSTreeDeployment{
		Base:         NewBase("ostree-deployment", buildPipeline),
		commitSource: commit,
		osName:       osName,
		platform:     platform,
	}
	buildPipeline.addDependent(p)
	return p
}

// NewOSTreeDeployment creates a pipeline for an ostree deployment from a
// container
func NewOSTreeContainerDeployment(buildPipeline Build,
	container *container.SourceSpec,
	ref string,
	osName string,
	platform platform.Platform) *OSTreeDeployment {

	p := &OSTreeDeployment{
		Base:            NewBase("ostree-deployment", buildPipeline),
		containerSource: container,
		osName:          osName,
		ref:             ref,
		platform:        platform,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *OSTreeDeployment) getBuildPackages(Distro) []string {
	packages := []string{
		"rpm-ostree",
	}
	return packages
}

func (p *OSTreeDeployment) getOSTreeCommits() []ostree.CommitSpec {
	if p.ostreeSpec == nil {
		return []ostree.CommitSpec{}
	}
	return []ostree.CommitSpec{*p.ostreeSpec}
}

func (p *OSTreeDeployment) getOSTreeCommitSources() []ostree.SourceSpec {
	if p.commitSource == nil {
		return []ostree.SourceSpec{}
	}
	return []ostree.SourceSpec{
		*p.commitSource,
	}
}

func (p *OSTreeDeployment) getContainerSpecs() []container.Spec {
	if p.containerSpec == nil {
		return []container.Spec{}
	}
	return []container.Spec{*p.containerSpec}
}

func (p *OSTreeDeployment) getContainerSources() []container.SourceSpec {
	if p.containerSource == nil {
		return []container.SourceSpec{}
	}
	return []container.SourceSpec{
		*p.containerSource,
	}
}

func (p *OSTreeDeployment) serializeStart(packages []rpmmd.PackageSpec, containers []container.Spec, commits []ostree.CommitSpec) {
	if p.ostreeSpec != nil || p.containerSpec != nil {
		panic("double call to serializeStart()")
	}

	switch {
	case len(commits) == 1:
		p.ostreeSpec = &commits[0]
	case len(containers) == 1:
		p.containerSpec = &containers[0]
	default:
		panic(fmt.Sprintf("pipeline %s requires exactly one ostree commit or one container (have commits: %v; containers: %v)", p.Name(), commits, containers))
	}
}

func (p *OSTreeDeployment) serializeEnd() {
	switch {
	case p.ostreeSpec == nil && p.containerSpec == nil:
		panic("serializeEnd() call when serialization not in progress")
	case p.ostreeSpec != nil && p.containerSpec != nil:
		panic("serializeEnd() multiple payload sources defined")
	}

	p.ostreeSpec = nil
	p.containerSpec = nil
}

func (p *OSTreeDeployment) doOSTreeSpec(pipeline *osbuild.Pipeline, repoPath string, kernelOpts []string) string {
	commit := *p.ostreeSpec
	ref := commit.Ref
	pipeline.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repoPath, Remote: p.Remote.Name},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", commit.Checksum, ref),
	))

	pipeline.AddStage(osbuild.NewOSTreeDeployStage(
		&osbuild.OSTreeDeployStageOptions{
			OsName: p.osName,
			Ref:    ref,
			Remote: p.Remote.Name,
			Mounts: []string{"/boot", "/boot/efi"},
			Rootfs: osbuild.Rootfs{
				Label: "root",
			},
			KernelOpts: kernelOpts,
		},
	))

	if p.Remote.URL != "" {
		pipeline.AddStage(osbuild.NewOSTreeRemotesStage(
			&osbuild.OSTreeRemotesStageOptions{
				Repo: "/ostree/repo",
				Remotes: []osbuild.OSTreeRemote{
					{
						Name:        p.Remote.Name,
						URL:         p.Remote.URL,
						ContentURL:  p.Remote.ContentURL,
						GPGKeyPaths: p.Remote.GPGKeyPaths,
					},
				},
			},
		))
	}

	pipeline.AddStage(osbuild.NewOSTreeFillvarStage(
		&osbuild.OSTreeFillvarStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: p.osName,
				Ref:    ref,
			},
		},
	))

	return ref
}

func (p *OSTreeDeployment) doOSTreeContainerSpec(pipeline *osbuild.Pipeline, repoPath string, kernelOpts []string) string {
	cont := *p.containerSpec
	ref := p.ref

	var targetImgref string
	// The ostree-remote case is unusual; it may be used by FCOS/Silverblue for example to handle
	// embedded GPG signatures
	if p.Remote.Name != "" {
		targetImgref = fmt.Sprintf("ostree-remote-registry:%s:%s", p.Remote.Name, p.containerSpec.LocalName)
	} else {
		targetImgref = fmt.Sprintf("ostree-unverified-registry:%s", p.containerSpec.LocalName)
	}

	options := &osbuild.OSTreeDeployContainerStageOptions{
		OsName:       p.osName,
		KernelOpts:   p.KernelOptionsAppend,
		TargetImgref: targetImgref,
		Mounts:       []string{"/boot", "/boot/efi"},
		Rootfs: &osbuild.Rootfs{
			Label: "root",
		},
	}
	images := osbuild.NewContainersInputForSources([]container.Spec{cont})
	pipeline.AddStage(osbuild.NewOSTreeDeployContainerStage(options, images))
	return ref
}

func (p *OSTreeDeployment) serialize() osbuild.Pipeline {
	switch {
	case p.ostreeSpec == nil && p.containerSpec == nil:
		panic("serialization not started")
	case p.ostreeSpec != nil && p.containerSpec != nil:
		panic("serialize() multiple payload sources defined")
	}

	const repoPath = "/ostree/repo"

	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.OSTreeInitFsStage())
	pipeline.AddStage(osbuild.NewOSTreeOsInitStage(
		&osbuild.OSTreeOsInitStageOptions{
			OSName: p.osName,
		},
	))
	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: "/boot/efi",
				Mode: common.ToPtr(os.FileMode(0700)),
			},
		},
	}))
	kernelOpts := osbuild.GenImageKernelOptions(p.PartitionTable)
	kernelOpts = append(kernelOpts, p.KernelOptionsAppend...)

	if p.IgnitionPlatform != "" {
		kernelOpts = append(kernelOpts,
			"ignition.platform.id="+p.IgnitionPlatform,
			"$ignition_firstboot",
		)
	}

	if p.FIPS {
		kernelOpts = append(kernelOpts, osbuild.GenFIPSKernelOptions(p.PartitionTable)...)
	}

	var ref string
	switch {
	case p.ostreeSpec != nil:
		ref = p.doOSTreeSpec(&pipeline, repoPath, kernelOpts)
	case p.containerSpec != nil:
		ref = p.doOSTreeContainerSpec(&pipeline, repoPath, kernelOpts)
	default:
		// this should be caught at the top of the function, but let's check
		// again to avoid bugs from bad refactoring.
		panic("no content source defined for ostree deployment")
	}

	configStage := osbuild.NewOSTreeConfigStage(
		&osbuild.OSTreeConfigStageOptions{
			Repo: repoPath,
			Config: &osbuild.OSTreeConfig{
				Sysroot: &osbuild.SysrootOptions{
					ReadOnly:   &p.SysrootReadOnly,
					Bootloader: "none",
				},
			},
		},
	)
	configStage.MountOSTree(p.osName, ref, 0)
	pipeline.AddStage(configStage)

	fstabOptions := osbuild.NewFSTabStageOptions(p.PartitionTable)
	fstabStage := osbuild.NewFSTabStage(fstabOptions)
	fstabStage.MountOSTree(p.osName, ref, 0)
	pipeline.AddStage(fstabStage)

	if len(p.Users) > 0 {
		usersStage, err := osbuild.GenUsersStage(p.Users, false)
		if err != nil {
			panic("password encryption failed")
		}
		usersStage.MountOSTree(p.osName, ref, 0)
		pipeline.AddStage(usersStage)
	}

	if len(p.Groups) > 0 {
		grpStage := osbuild.GenGroupsStage(p.Groups)
		grpStage.MountOSTree(p.osName, ref, 0)
		pipeline.AddStage(grpStage)
	}

	if p.IgnitionPlatform != "" {
		pipeline.AddStage(osbuild.NewIgnitionStage(&osbuild.IgnitionStageOptions{
			// This is a workaround to make the systemd believe it's firstboot when ignition runs on real firstboot.
			// Right now, since we ship /etc/machine-id, systemd thinks it's not firstboot and ignition depends on it
			// to run on the real firstboot to enable services from presets.
			// Since this only applies to artifacts with ignition and changing machineid-compat at commit creation time may
			// have undesiderable effect, we're doing it here as a stopgap. We may revisit this in the future.
			Network: []string{
				"systemd.firstboot=off",
				"systemd.condition-first-boot=true",
			},
		}))

		// We enable / disable services below using the systemd stage, but its effect
		// may be overridden by systemd which may reset enabled / disabled services on
		// firstboot (which happend on F37+). This behavior, if available, is triggered
		// only when Ignition is used. To prevent this and to not have a special cases
		// in the code based on distro version, we enable / disable services also by
		// creating a preset file.
		if len(p.EnabledServices) != 0 || len(p.DisabledServices) != 0 {
			presetsStage := osbuild.GenServicesPresetStage(p.EnabledServices, p.DisabledServices)
			presetsStage.MountOSTree(p.osName, ref, 0)
			pipeline.AddStage(presetsStage)
		}
	}

	// if no root password is set, lock the root account
	hasRoot := false
	for _, user := range p.Users {
		if user.Name == "root" {
			hasRoot = true
			break
		}
	}

	if p.LockRoot && !hasRoot {
		userOptions := &osbuild.UsersStageOptions{
			Users: map[string]osbuild.UsersStageOptionsUser{
				"root": {
					Password: common.ToPtr("!locked"), // this is treated as crypted and locks/disables the password
				},
			},
		}
		rootLockStage := osbuild.NewUsersStage(userOptions)
		rootLockStage.MountOSTree(p.osName, ref, 0)
		pipeline.AddStage(rootLockStage)
	}

	if p.Keyboard != "" {
		options := &osbuild.KeymapStageOptions{
			Keymap: p.Keyboard,
		}
		keymapStage := osbuild.NewKeymapStage(options)
		keymapStage.MountOSTree(p.osName, ref, 0)
		pipeline.AddStage(keymapStage)
	}

	if p.Locale != "" {
		options := &osbuild.LocaleStageOptions{
			Language: p.Locale,
		}
		localeStage := osbuild.NewLocaleStage(options)
		localeStage.MountOSTree(p.osName, ref, 0)
		pipeline.AddStage(localeStage)
	}

	if p.FIPS {
		p.Files = append(p.Files, osbuild.GenFIPSFiles()...)
		for _, stage := range osbuild.GenFIPSStages() {
			stage.MountOSTree(p.osName, ref, 0)
			pipeline.AddStage(stage)
		}
	}

	if !p.UseBootupd {
		grubOptions := osbuild.NewGrub2StageOptions(p.PartitionTable,
			strings.Join(kernelOpts, " "),
			"",
			p.platform.GetUEFIVendor() != "",
			p.platform.GetBIOSPlatform(),
			p.platform.GetUEFIVendor(), true)
		grubOptions.Greenboot = true
		grubOptions.Ignition = p.IgnitionPlatform != ""
		grubOptions.Config = &osbuild.GRUB2Config{
			Default:        "saved",
			Timeout:        1,
			TerminalOutput: []string{"console"},
		}
		bootloader := osbuild.NewGRUB2Stage(grubOptions)
		bootloader.MountOSTree(p.osName, ref, 0)
		pipeline.AddStage(bootloader)
	}

	// First create custom directories, because some of the files may depend on them
	if len(p.Directories) > 0 {
		dirStages := osbuild.GenDirectoryNodesStages(p.Directories)
		for _, stage := range dirStages {
			stage.MountOSTree(p.osName, ref, 0)
		}
		pipeline.AddStages(dirStages...)
	}

	if len(p.Files) > 0 {
		fileStages := osbuild.GenFileNodesStages(p.Files)
		for _, stage := range fileStages {
			stage.MountOSTree(p.osName, ref, 0)
		}
		pipeline.AddStages(fileStages...)
	}

	if len(p.EnabledServices) != 0 || len(p.DisabledServices) != 0 {
		systemdStage := osbuild.NewSystemdStage(&osbuild.SystemdStageOptions{
			EnabledServices:  p.EnabledServices,
			DisabledServices: p.DisabledServices,
		})
		systemdStage.MountOSTree(p.osName, ref, 0)
		pipeline.AddStage(systemdStage)
	}

	pipeline.AddStage(osbuild.NewOSTreeSelinuxStage(
		&osbuild.OSTreeSelinuxStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: p.osName,
				Ref:    ref,
			},
		},
	))

	return pipeline
}

func (p *OSTreeDeployment) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.Files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}
