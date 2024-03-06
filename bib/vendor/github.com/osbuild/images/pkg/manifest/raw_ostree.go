package manifest

import (
	"fmt"

	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
)

// A RawOSTreeImage represents a raw ostree image file which can be booted in a
// hypervisor. It is created from an existing OSTreeDeployment.
type RawOSTreeImage struct {
	Base
	treePipeline *OSTreeDeployment
	filename     string
	platform     platform.Platform
}

func (p RawOSTreeImage) Filename() string {
	return p.filename
}

func (p *RawOSTreeImage) SetFilename(filename string) {
	p.filename = filename
}

func NewRawOStreeImage(buildPipeline Build, treePipeline *OSTreeDeployment, platform platform.Platform) *RawOSTreeImage {
	p := &RawOSTreeImage{
		Base:         NewBase("image", buildPipeline),
		treePipeline: treePipeline,
		filename:     "disk.img",
		platform:     platform,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *RawOSTreeImage) getBuildPackages(Distro) []string {
	packages := p.platform.GetBuildPackages()
	packages = append(packages, p.platform.GetPackages()...)
	packages = append(packages, p.treePipeline.PartitionTable.GetBuildPackages()...)
	packages = append(packages,
		"rpm-ostree",

		// these should be defined on the platform
		"dracut-config-generic",
		"efibootmgr",
	)
	return packages
}

func (p *RawOSTreeImage) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pt := p.treePipeline.PartitionTable
	if pt == nil {
		panic("no partition table in live image")
	}

	for _, stage := range osbuild.GenImagePrepareStages(pt, p.Filename(), osbuild.PTSfdisk) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	treeCopyOptions, treeCopyDevices, treeCopyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename(), pt)
	treeCopyInputs := osbuild.NewPipelineTreeInputs(inputName, p.treePipeline.Name())

	pipeline.AddStage(osbuild.NewCopyStage(treeCopyOptions, treeCopyInputs, treeCopyDevices, treeCopyMounts))

	bootFiles := p.platform.GetBootFiles()
	if len(bootFiles) > 0 {
		// we ignore the bootcopyoptions as they contain a full tree copy instead we make our own, we *do* still want all the other
		// information such as mountpoints and devices
		_, bootCopyDevices, bootCopyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename(), pt)
		bootCopyOptions := &osbuild.CopyStageOptions{}

		commit := p.treePipeline.ostreeSpec
		commitChecksum := commit.Checksum

		bootCopyInputs := osbuild.OSTreeCheckoutInputs{
			"ostree-tree": *osbuild.NewOSTreeCheckoutInput("org.osbuild.source", commitChecksum),
		}

		// Find the FS root mount name to use as the destination root
		// for the target when copying the boot files.
		var fsRootMntName string
		for _, mnt := range bootCopyMounts {
			if mnt.Target == "/" {
				fsRootMntName = mnt.Name
				break
			}
		}

		if fsRootMntName == "" {
			panic("no mount found for the filesystem root")
		}

		for _, paths := range bootFiles {
			bootCopyOptions.Paths = append(bootCopyOptions.Paths, osbuild.CopyStagePath{
				From: fmt.Sprintf("input://ostree-tree/%s%s", commitChecksum, paths[0]),
				To:   fmt.Sprintf("mount://%s%s", fsRootMntName, paths[1]),
			})
		}

		pipeline.AddStage(osbuild.NewCopyStage(bootCopyOptions, bootCopyInputs, bootCopyDevices, bootCopyMounts))
	}

	for _, stage := range osbuild.GenImageFinishStages(pt, p.Filename()) {
		pipeline.AddStage(stage)
	}

	if p.treePipeline.UseBootupd {
		p.addBootupdStage(&pipeline)
	} else {
		p.maybeAddGrubInstStage(&pipeline)
	}

	return pipeline
}

func (p *RawOSTreeImage) addBootupdStage(pipeline *osbuild.Pipeline) {
	pt := p.treePipeline.PartitionTable

	treeBootupdDevices, treeBootupdMounts := osbuild.GenBootupdDevicesMounts(p.Filename(), pt)
	opts := &osbuild.BootupdStageOptions{
		Deployment: &osbuild.OSTreeDeployment{
			OSName: p.treePipeline.osName,
			Ref:    p.treePipeline.ref,
		},
		StaticConfigs: true,
	}
	if legacyBios := p.treePipeline.platform.GetBIOSPlatform(); legacyBios != "" {
		opts.Bios = &osbuild.BootupdStageOptionsBios{
			Device: "disk",
		}
	}
	bootupd, err := osbuild.NewBootupdStage(opts, treeBootupdDevices, treeBootupdMounts)
	if err != nil {
		panic(err)
	}

	pipeline.AddStage(bootupd)
}

func (p *RawOSTreeImage) maybeAddGrubInstStage(pipeline *osbuild.Pipeline) {
	pt := p.treePipeline.PartitionTable

	if grubLegacy := p.treePipeline.platform.GetBIOSPlatform(); grubLegacy != "" {
		pipeline.AddStage(osbuild.NewGrub2InstStage(osbuild.NewGrub2InstStageOption(p.Filename(), pt, grubLegacy)))
	}
}

func (p *RawOSTreeImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
