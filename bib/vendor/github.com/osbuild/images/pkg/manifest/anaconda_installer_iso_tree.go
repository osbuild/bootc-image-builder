package manifest

import (
	"fmt"
	"path"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
)

// An AnacondaInstallerISOTree represents a tree containing the anaconda installer,
// configuration in terms of a kickstart file, as well as an embedded
// payload to be installed, this payload can either be an ostree
// CommitSpec or OSPipeline for an OS.
type AnacondaInstallerISOTree struct {
	Base

	// TODO: review optional and mandatory fields and their meaning
	OSName  string
	Release string
	Remote  string
	Users   []users.User
	Groups  []users.Group

	Language *string
	Keyboard *string
	Timezone *string

	// Create a sudoers drop-in file for wheel group with NOPASSWD option
	WheelNoPasswd bool

	// Add kickstart options to make the installation fully unattended
	UnattendedKickstart bool

	PartitionTable *disk.PartitionTable

	anacondaPipeline *AnacondaInstaller
	rootfsPipeline   *ISORootfsImg
	bootTreePipeline *EFIBootTree

	// The location of the kickstart file, if it will be added to the
	// bootiso-tree.
	// Otherwise, it should be defined in the interactive defaults of the
	// Anaconda pipeline.
	KSPath string

	// The path where the payload (tarball, ostree repo, or container) will be stored.
	PayloadPath string

	isoLabel string

	SquashfsCompression string

	OSPipeline         *OS
	OSTreeCommitSource *ostree.SourceSpec

	ostreeCommitSpec *ostree.CommitSpec
	ContainerSource  *container.SourceSpec
	containerSpec    *container.Spec

	KernelOpts []string

	// Enable ISOLinux stage
	ISOLinux bool

	Files []*fsnode.File
}

func NewAnacondaInstallerISOTree(buildPipeline Build, anacondaPipeline *AnacondaInstaller, rootfsPipeline *ISORootfsImg, bootTreePipeline *EFIBootTree) *AnacondaInstallerISOTree {

	// the three pipelines should all belong to the same manifest
	if anacondaPipeline.Manifest() != rootfsPipeline.Manifest() ||
		anacondaPipeline.Manifest() != bootTreePipeline.Manifest() {
		panic("pipelines from different manifests")
	}
	p := &AnacondaInstallerISOTree{
		Base:             NewBase("bootiso-tree", buildPipeline),
		anacondaPipeline: anacondaPipeline,
		rootfsPipeline:   rootfsPipeline,
		bootTreePipeline: bootTreePipeline,
		isoLabel:         bootTreePipeline.ISOLabel,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *AnacondaInstallerISOTree) getOSTreeCommitSources() []ostree.SourceSpec {
	if p.OSTreeCommitSource == nil {
		return nil
	}

	return []ostree.SourceSpec{
		*p.OSTreeCommitSource,
	}
}

func (p *AnacondaInstallerISOTree) getOSTreeCommits() []ostree.CommitSpec {
	if p.ostreeCommitSpec == nil {
		return nil
	}
	return []ostree.CommitSpec{*p.ostreeCommitSpec}
}

func (p *AnacondaInstallerISOTree) getContainerSpecs() []container.Spec {
	if p.containerSpec == nil {
		return []container.Spec{}
	}
	return []container.Spec{*p.containerSpec}
}

func (p *AnacondaInstallerISOTree) getContainerSources() []container.SourceSpec {
	if p.ContainerSource == nil {
		return []container.SourceSpec{}
	}
	return []container.SourceSpec{
		*p.ContainerSource,
	}
}

func (p *AnacondaInstallerISOTree) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.Files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}
func (p *AnacondaInstallerISOTree) getBuildPackages(_ Distro) []string {
	packages := []string{
		"squashfs-tools",
	}

	if p.OSTreeCommitSource != nil {
		packages = append(packages, "rpm-ostree")
	}

	if p.ContainerSource != nil {
		packages = append(packages, "skopeo")
	}

	if p.OSPipeline != nil {
		packages = append(packages, "tar")
	}

	return packages
}

func (p *AnacondaInstallerISOTree) serializeStart(_ []rpmmd.PackageSpec, containers []container.Spec, commits []ostree.CommitSpec) {
	if p.ostreeCommitSpec != nil || p.containerSpec != nil {
		panic("double call to serializeStart()")
	}

	if len(commits) > 1 {
		panic("pipeline supports at most one ostree commit")
	}

	if len(containers) > 1 {
		panic("pipeline supports at most one container")
	}

	if len(commits) > 0 {
		p.ostreeCommitSpec = &commits[0]
	}

	if len(containers) > 0 {
		p.containerSpec = &containers[0]
	}
}

func (p *AnacondaInstallerISOTree) serializeEnd() {
	p.ostreeCommitSpec = nil
	p.containerSpec = nil
}

func (p *AnacondaInstallerISOTree) serialize() osbuild.Pipeline {
	// If the anaconda pipeline is a payload then we need one of three payload types
	if p.anacondaPipeline.Type == AnacondaInstallerTypePayload {
		count := 0

		if p.ostreeCommitSpec != nil {
			count++
		}

		if p.containerSpec != nil {
			count++
		}

		if p.OSPipeline != nil {
			count++
		}

		if count == 0 {
			panic("missing ostree, container, or ospipeline parameters in ISO tree pipeline")
		}

		// But not more than one payloads
		if count > 1 {
			panic("got multiple payloads in ISO tree pipeline")
		}
	}

	pipeline := p.Base.serialize()

	kernelOpts := []string{}

	if p.anacondaPipeline.Type == AnacondaInstallerTypePayload {
		kernelOpts = append(kernelOpts, fmt.Sprintf("inst.stage2=hd:LABEL=%s", p.isoLabel))
		if p.KSPath != "" {
			kernelOpts = append(kernelOpts, fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", p.isoLabel, p.KSPath))
		}
	}

	if len(p.KernelOpts) > 0 {
		kernelOpts = append(kernelOpts, p.KernelOpts...)
	}

	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: "/images",
			},
			{
				Path: "/images/pxeboot",
			},
		},
	}))

	if p.anacondaPipeline.Type == AnacondaInstallerTypeLive {
		pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
			Paths: []osbuild.MkdirStagePath{
				{
					Path: "/LiveOS",
				},
			},
		}))
	}

	inputName := "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/boot/vmlinuz-%s", inputName, p.anacondaPipeline.kernelVer),
				To:   "tree:///images/pxeboot/vmlinuz",
			},
			{
				From: fmt.Sprintf("input://%s/boot/initramfs-%s.img", inputName, p.anacondaPipeline.kernelVer),
				To:   "tree:///images/pxeboot/initrd.img",
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.anacondaPipeline.Name())
	copyStage := osbuild.NewCopyStageSimple(copyStageOptions, copyStageInputs)
	pipeline.AddStage(copyStage)

	var squashfsOptions osbuild.SquashfsStageOptions

	if p.anacondaPipeline.Type == AnacondaInstallerTypePayload {
		squashfsOptions = osbuild.SquashfsStageOptions{
			Filename: "images/install.img",
		}
	} else if p.anacondaPipeline.Type == AnacondaInstallerTypeLive {
		squashfsOptions = osbuild.SquashfsStageOptions{
			Filename: "LiveOS/squashfs.img",
		}
	}

	if p.SquashfsCompression != "" {
		squashfsOptions.Compression.Method = p.SquashfsCompression
	} else {
		// default to xz if not specified
		squashfsOptions.Compression.Method = "xz"
	}

	if squashfsOptions.Compression.Method == "xz" {
		squashfsOptions.Compression.Options = &osbuild.FSCompressionOptions{
			BCJ: osbuild.BCJOption(p.anacondaPipeline.platform.GetArch().String()),
		}
	}

	squashfsStage := osbuild.NewSquashfsStage(&squashfsOptions, p.rootfsPipeline.Name())
	pipeline.AddStage(squashfsStage)

	if p.ISOLinux {
		isoLinuxOptions := &osbuild.ISOLinuxStageOptions{
			Product: osbuild.ISOLinuxProduct{
				Name:    p.anacondaPipeline.product,
				Version: p.anacondaPipeline.version,
			},
			Kernel: osbuild.ISOLinuxKernel{
				Dir:  "/images/pxeboot",
				Opts: kernelOpts,
			},
		}

		isoLinuxStage := osbuild.NewISOLinuxStage(isoLinuxOptions, p.anacondaPipeline.Name())
		pipeline.AddStage(isoLinuxStage)
	}

	filename := "images/efiboot.img"
	pipeline.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{
		Filename: filename,
		Size:     fmt.Sprintf("%d", p.PartitionTable.Size),
	}))

	efibootDevice := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: filename})
	for _, stage := range osbuild.GenMkfsStages(p.PartitionTable, efibootDevice) {
		pipeline.AddStage(stage)
	}

	inputName = "root-tree"
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.bootTreePipeline.Name())
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.bootTreePipeline.Name(), filename, p.PartitionTable)
	pipeline.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	copyInputs = osbuild.NewPipelineTreeInputs(inputName, p.bootTreePipeline.Name())
	pipeline.AddStage(osbuild.NewCopyStageSimple(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: fmt.Sprintf("input://%s/EFI", inputName),
					To:   "tree:///",
				},
			},
		},
		copyInputs,
	))

	if p.anacondaPipeline.Type == AnacondaInstallerTypePayload {
		// the following pipelines are only relevant for payload installers
		switch {
		case p.ostreeCommitSpec != nil:
			pipeline.AddStages(p.ostreeCommitStages()...)
		case p.containerSpec != nil:
			pipeline.AddStages(p.ostreeContainerStages()...)
		case p.OSPipeline != nil:
			pipeline.AddStages(p.tarPayloadStages()...)
		default:
			// this should have been caught at the top of the function, but
			// let's check again in case we refactor the function.
			panic("missing ostree, container, or ospipeline parameters in ISO tree pipeline")
		}
	}

	pipeline.AddStage(osbuild.NewDiscinfoStage(&osbuild.DiscinfoStageOptions{
		BaseArch: p.anacondaPipeline.platform.GetArch().String(),
		Release:  p.Release,
	}))

	return pipeline
}

func (p *AnacondaInstallerISOTree) ostreeCommitStages() []*osbuild.Stage {
	stages := make([]*osbuild.Stage, 0)

	// Set up the payload ostree repo
	stages = append(stages, osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: p.PayloadPath}))
	stages = append(stages, osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: p.PayloadPath},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", p.ostreeCommitSpec.Checksum, p.ostreeCommitSpec.Ref),
	))

	// Configure the kickstart file with the payload and any user options
	kickstartOptions, err := osbuild.NewKickstartStageOptionsWithOSTreeCommit(
		p.KSPath,
		p.Users,
		p.Groups,
		makeISORootPath(p.PayloadPath),
		p.ostreeCommitSpec.Ref,
		p.Remote,
		p.OSName)

	if err != nil {
		panic(fmt.Sprintf("failed to create kickstart stage options: %v", err))
	}

	stages = append(stages, p.makeKickstartStages(kickstartOptions)...)

	return stages
}

func (p *AnacondaInstallerISOTree) ostreeContainerStages() []*osbuild.Stage {
	stages := make([]*osbuild.Stage, 0)

	images := osbuild.NewContainersInputForSources([]container.Spec{*p.containerSpec})

	stages = append(stages, osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: p.PayloadPath,
			},
		},
	}))

	// copy the container in
	stages = append(stages, osbuild.NewSkopeoStageWithOCI(
		p.PayloadPath,
		images,
		nil))

	// do what we can in our kickstart stage
	kickstartOptions, err := osbuild.NewKickstartStageOptionsWithOSTreeContainer(
		p.KSPath,
		p.Users,
		p.Groups,
		path.Join("/run/install/repo", p.PayloadPath),
		"oci",
		"",
		"")
	if err != nil {
		panic(fmt.Sprintf("failed to create kickstart stage options: %v", err))
	}

	// NOTE: these are similar to the unattended kickstart options in the
	// other two payload configurations but partitioning is different and
	// we need to add that separately, so we can't use makeKickstartStage
	kickstartOptions.RootPassword = &osbuild.RootPasswordOptions{
		Lock: true,
	}

	// NOTE: These were decided somewhat arbitrarily for the BIB installer. We
	// might want to drop them here and move them into the bib code as
	// project-specific defaults.
	kickstartOptions.Lang = "en_US.UTF-8"
	kickstartOptions.Keyboard = "us"
	kickstartOptions.Timezone = "UTC"
	kickstartOptions.ClearPart = &osbuild.ClearPartOptions{
		All: true,
	}

	stages = append(stages, osbuild.NewKickstartStage(kickstartOptions))

	// and what we can't do in a separate kickstart that we include
	targetContainerTransport := "registry"
	if p.containerSpec.ContainersTransport != nil {
		targetContainerTransport = *p.containerSpec.ContainersTransport
	}
	// Canonicalize to registry, as that's what the bootc stack wants
	if targetContainerTransport == "docker://" {
		targetContainerTransport = "registry"
	}

	// Because osbuild core only supports a subset of options, we append to the
	// base here with some more hardcoded defaults
	// that should very likely become configurable.
	hardcodedKickstartBits := `
reqpart --add-boot

part swap --fstype=swap --size=1024
part / --fstype=ext4 --grow

reboot --eject
`

	// Workaround for lack of --target-imgref in Anaconda, xref https://github.com/osbuild/images/issues/380
	hardcodedKickstartBits += fmt.Sprintf(`%%post
bootc switch --mutate-in-place --transport %s %s
%%end
`, targetContainerTransport, p.containerSpec.LocalName)

	kickstartFile, err := kickstartOptions.IncludeRaw(hardcodedKickstartBits)
	if err != nil {
		panic(err)
	}

	p.Files = []*fsnode.File{kickstartFile}

	stages = append(stages, osbuild.GenFileNodesStages(p.Files)...)
	return stages
}

func (p *AnacondaInstallerISOTree) tarPayloadStages() []*osbuild.Stage {
	stages := make([]*osbuild.Stage, 0)

	// Create the payload tarball
	stages = append(stages, osbuild.NewTarStage(&osbuild.TarStageOptions{Filename: p.PayloadPath}, p.OSPipeline.name))

	// If the KSPath is set, we need to add the kickstart stage to this (bootiso-tree) pipeline.
	// If it's not specified here, it should have been added to the InteractiveDefaults in the anaconda-tree.
	if p.KSPath != "" {
		kickstartOptions, err := osbuild.NewKickstartStageOptionsWithLiveIMG(
			p.KSPath,
			p.Users,
			p.Groups,
			makeISORootPath(p.PayloadPath))

		if err != nil {
			panic(fmt.Sprintf("failed to create kickstart stage options: %v", err))
		}

		stages = append(stages, p.makeKickstartStages(kickstartOptions)...)
	}
	return stages
}

// Create the base kickstart stage with any options required for unattended
// installation if set and with any extra file insertion stage required for
// extra kickstart content.
func (p *AnacondaInstallerISOTree) makeKickstartStages(kickstartOptions *osbuild.KickstartStageOptions) []*osbuild.Stage {
	stages := make([]*osbuild.Stage, 0)
	if p.UnattendedKickstart {
		// set the default options for Unattended kickstart
		kickstartOptions.DisplayMode = "text"

		// override options that can be configured by the image type or the user
		kickstartOptions.Lang = "en_US.UTF-8"
		if p.Language != nil {
			kickstartOptions.Lang = *p.Language
		}

		kickstartOptions.Keyboard = "us"
		if p.Keyboard != nil {
			kickstartOptions.Keyboard = *p.Keyboard
		}

		kickstartOptions.Timezone = "UTC"
		if p.Timezone != nil {
			kickstartOptions.Timezone = *p.Timezone
		}

		kickstartOptions.Reboot = &osbuild.RebootOptions{Eject: true}
		kickstartOptions.RootPassword = &osbuild.RootPasswordOptions{Lock: true}

		kickstartOptions.ZeroMBR = true
		kickstartOptions.ClearPart = &osbuild.ClearPartOptions{All: true, InitLabel: true}
		kickstartOptions.AutoPart = &osbuild.AutoPartOptions{Type: "plain", FSType: "xfs", NoHome: true}

		kickstartOptions.Network = []osbuild.NetworkOptions{
			{BootProto: "dhcp", Device: "link", Activate: common.ToPtr(true), OnBoot: "on"},
		}
	}

	stages = append(stages, osbuild.NewKickstartStage(kickstartOptions))

	if p.WheelNoPasswd {
		// Because osbuild core only supports a subset of options,
		// we append to the base here with hardcoded wheel group with NOPASSWD option
		hardcodedKickstartBits := `
%post
echo -e "%wheel\tALL=(ALL)\tNOPASSWD: ALL" > "/etc/sudoers.d/wheel"
chmod 0440 /etc/sudoers.d/wheel
restorecon -rvF /etc/sudoers.d
%end
`
		kickstartFile, err := kickstartOptions.IncludeRaw(hardcodedKickstartBits)
		if err != nil {
			panic(err)
		}

		p.Files = []*fsnode.File{kickstartFile}

		stages = append(stages, osbuild.GenFileNodesStages(p.Files)...)
	}
	return stages
}

// makeISORootPath return a path that can be used to address files and folders
// in the root of the iso
func makeISORootPath(p string) string {
	fullpath := path.Join("/run/install/repo", p)
	return fmt.Sprintf("file://%s", fullpath)
}
