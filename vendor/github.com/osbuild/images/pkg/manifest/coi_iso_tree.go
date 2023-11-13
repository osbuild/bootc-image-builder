package manifest

import (
	"crypto/sha256"
	"fmt"

	"github.com/osbuild/images/internal/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
)

type CoreOSISOTree struct {
	Base

	// TODO: review optional and mandatory fields and their meaning
	OSName  string
	Release string
	Users   []users.User
	Groups  []users.Group

	PartitionTable *disk.PartitionTable

	payloadPipeline  *XZ
	coiPipeline      *CoreOSInstaller
	bootTreePipeline *EFIBootTree

	// The path where the payload (tarball or ostree repo) will be stored.
	PayloadPath string

	isoLabel string

	// Enable ISOLinux stage
	ISOLinux bool

	KernelOpts []string
}

func NewCoreOSISOTree(
	buildPipeline *Build,
	payloadPipeline *XZ,
	coiPipeline *CoreOSInstaller,
	bootTreePipeline *EFIBootTree) *CoreOSISOTree {

	// the three pipelines should all belong to the same manifest
	if payloadPipeline.Manifest() != coiPipeline.Manifest() ||
		payloadPipeline.Manifest() != bootTreePipeline.Manifest() {
		panic("pipelines from different manifests")
	}

	p := &CoreOSISOTree{
		Base:             NewBase(coiPipeline.Manifest(), "bootiso-tree", buildPipeline),
		payloadPipeline:  payloadPipeline,
		coiPipeline:      coiPipeline,
		bootTreePipeline: bootTreePipeline,
		isoLabel:         bootTreePipeline.ISOLabel,
	}
	buildPipeline.addDependent(p)
	coiPipeline.Manifest().addPipeline(p)
	return p
}

func (p *CoreOSISOTree) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewCopyStageSimple(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: fmt.Sprintf("input://file/%s", p.payloadPipeline.Filename()),
					To:   fmt.Sprintf("tree://%s", p.PayloadPath),
				},
			},
		},
		osbuild.NewXzStageInputs(osbuild.NewFilesInputPipelineObjectRef(p.payloadPipeline.Name(), p.payloadPipeline.Filename(), nil)),
	))

	if p.coiPipeline.Ignition != nil {
		filename := ""
		copyInput := ""
		// These specific filenames in the root of the ISO are expected by
		// coreos-installer-dracut during installation
		if p.coiPipeline.Ignition.Config != "" {
			filename = "ignition_config"
			copyInput = p.coiPipeline.Ignition.Config
		}
		pipeline.AddStage(osbuild.NewCopyStageSimple(
			&osbuild.CopyStageOptions{
				Paths: []osbuild.CopyStagePath{
					{
						From: fmt.Sprintf("input://inlinefile/sha256:%x", sha256.Sum256([]byte(copyInput))),
						To:   fmt.Sprintf("tree:///%s", filename),
					},
				},
			},
			osbuild.NewIgnitionInlineInput(copyInput)))
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

	filename := "images/efiboot.img"
	pipeline.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{
		Filename: filename,
		Size:     fmt.Sprintf("%d", p.PartitionTable.Size),
	}))

	efibootDevice := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: filename})
	for _, stage := range osbuild.GenMkfsStages(p.PartitionTable, efibootDevice) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.bootTreePipeline.Name())
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.bootTreePipeline.Name(), filename, p.PartitionTable)
	pipeline.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	inputName = "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/boot/vmlinuz-%s", inputName, p.coiPipeline.kernelVer),
				To:   "tree:///images/pxeboot/vmlinuz",
			},
			{
				From: fmt.Sprintf("input://%s/boot/initramfs-%s.img", inputName, p.coiPipeline.kernelVer),
				To:   "tree:///images/pxeboot/initrd.img",
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.coiPipeline.Name())
	copyStage := osbuild.NewCopyStageSimple(copyStageOptions, copyStageInputs)
	pipeline.AddStage(copyStage)

	if p.ISOLinux {
		isoLinuxOptions := &osbuild.ISOLinuxStageOptions{
			Product: osbuild.ISOLinuxProduct{
				Name:    p.coiPipeline.product,
				Version: p.coiPipeline.version,
			},
			Kernel: osbuild.ISOLinuxKernel{
				Dir:  "/images/pxeboot",
				Opts: p.KernelOpts,
			},
		}

		isoLinuxStage := osbuild.NewISOLinuxStage(isoLinuxOptions, p.coiPipeline.Name())
		pipeline.AddStage(isoLinuxStage)
	}

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

	return pipeline
}
