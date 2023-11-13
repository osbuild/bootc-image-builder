package manifest

import (
	"fmt"
	"path"

	"github.com/osbuild/images/internal/users"
	"github.com/osbuild/images/pkg/container"
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
	Users   []users.User
	Groups  []users.Group

	PartitionTable *disk.PartitionTable

	anacondaPipeline *AnacondaInstaller
	rootfsPipeline   *ISORootfsImg
	bootTreePipeline *EFIBootTree

	// The location of the kickstart file, if it will be added to the
	// bootiso-tree.
	// Otherwise, it should be defined in the interactive defaults of the
	// Anaconda pipeline.
	KSPath string

	// The path where the payload (tarball or ostree repo) will be stored.
	PayloadPath string

	isoLabel string

	SquashfsCompression string

	OSPipeline         *OS
	OSTreeCommitSource *ostree.SourceSpec

	ostreeCommitSpec *ostree.CommitSpec

	KernelOpts []string

	// Enable ISOLinux stage
	ISOLinux bool
}

func NewAnacondaInstallerISOTree(buildPipeline *Build, anacondaPipeline *AnacondaInstaller, rootfsPipeline *ISORootfsImg, bootTreePipeline *EFIBootTree) *AnacondaInstallerISOTree {

	// the three pipelines should all belong to the same manifest
	if anacondaPipeline.Manifest() != rootfsPipeline.Manifest() ||
		anacondaPipeline.Manifest() != bootTreePipeline.Manifest() {
		panic("pipelines from different manifests")
	}
	p := &AnacondaInstallerISOTree{
		Base:             NewBase(anacondaPipeline.Manifest(), "bootiso-tree", buildPipeline),
		anacondaPipeline: anacondaPipeline,
		rootfsPipeline:   rootfsPipeline,
		bootTreePipeline: bootTreePipeline,
		isoLabel:         bootTreePipeline.ISOLabel,
	}
	buildPipeline.addDependent(p)
	anacondaPipeline.Manifest().addPipeline(p)
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

func (p *AnacondaInstallerISOTree) getBuildPackages(_ Distro) []string {
	packages := []string{
		"squashfs-tools",
	}

	if p.OSTreeCommitSource != nil {
		packages = append(packages, "rpm-ostree")
	}

	if p.OSPipeline != nil {
		packages = append(packages, "tar")
	}

	return packages
}

func (p *AnacondaInstallerISOTree) serializeStart(_ []rpmmd.PackageSpec, _ []container.Spec, commits []ostree.CommitSpec) {
	if len(commits) == 0 {
		// nothing to do
		return
	}

	if len(commits) > 1 {
		panic("pipeline supports at most one ostree commit")
	}

	p.ostreeCommitSpec = &commits[0]
}

func (p *AnacondaInstallerISOTree) serializeEnd() {
	p.ostreeCommitSpec = nil
}

func (p *AnacondaInstallerISOTree) serialize() osbuild.Pipeline {
	// If the anaconda pipeline is a payload then we need one of two payload types
	if p.anacondaPipeline.Type == AnacondaInstallerTypePayload {
		if p.ostreeCommitSpec == nil && p.OSPipeline == nil {
			panic("missing ostree or ospipeline parameters in ISO tree pipeline")
		}

		// But not both payloads
		if p.ostreeCommitSpec != nil && p.OSPipeline != nil {
			panic("got both ostree and ospipeline parameters in ISO tree pipeline")
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

	if p.ostreeCommitSpec != nil {
		// Set up the payload ostree repo
		pipeline.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: p.PayloadPath}))
		pipeline.AddStage(osbuild.NewOSTreePullStage(
			&osbuild.OSTreePullStageOptions{Repo: p.PayloadPath},
			osbuild.NewOstreePullStageInputs("org.osbuild.source", p.ostreeCommitSpec.Checksum, p.ostreeCommitSpec.Ref),
		))

		// Configure the kickstart file with the payload and any user options
		kickstartOptions, err := osbuild.NewKickstartStageOptions(p.KSPath, "", p.Users, p.Groups, makeISORootPath(p.PayloadPath), p.ostreeCommitSpec.Ref, p.OSName)

		if err != nil {
			panic("failed to create kickstartstage options")
		}

		pipeline.AddStage(osbuild.NewKickstartStage(kickstartOptions))
	}

	if p.OSPipeline != nil {
		// Create the payload tarball
		pipeline.AddStage(osbuild.NewTarStage(&osbuild.TarStageOptions{Filename: p.PayloadPath}, p.OSPipeline.name))

		// If the KSPath is set, we need to add the kickstart stage to this (bootiso-tree) pipeline.
		// If it's not specified here, it should have been added to the InteractiveDefaults in the anaconda-tree.
		if p.KSPath != "" {
			kickstartOptions, err := osbuild.NewKickstartStageOptions(p.KSPath, makeISORootPath(p.PayloadPath), p.Users, p.Groups, "", "", p.OSName)
			if err != nil {
				panic("failed to create kickstartstage options")
			}

			pipeline.AddStage(osbuild.NewKickstartStage(kickstartOptions))
		}
	}

	pipeline.AddStage(osbuild.NewDiscinfoStage(&osbuild.DiscinfoStageOptions{
		BaseArch: p.anacondaPipeline.platform.GetArch().String(),
		Release:  p.Release,
	}))

	return pipeline
}

// makeISORootPath return a path that can be used to address files and folders
// in the root of the iso
func makeISORootPath(p string) string {
	fullpath := path.Join("/run/install/repo", p)
	return fmt.Sprintf("file://%s", fullpath)
}
