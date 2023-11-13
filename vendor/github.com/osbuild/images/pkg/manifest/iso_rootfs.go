package manifest

import (
	"fmt"

	"github.com/osbuild/images/pkg/osbuild"
)

type ISORootfsImg struct {
	Base

	Size uint64

	installerPipeline Pipeline
}

func NewISORootfsImg(buildPipeline *Build, installerPipeline Pipeline) *ISORootfsImg {
	p := &ISORootfsImg{
		Base:              NewBase(installerPipeline.Manifest(), "rootfs-image", buildPipeline),
		installerPipeline: installerPipeline,
	}
	buildPipeline.addDependent(p)
	installerPipeline.Manifest().addPipeline(p)
	return p
}

func (p *ISORootfsImg) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: "/LiveOS",
			},
		},
	}))
	pipeline.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{
		Filename: "/LiveOS/rootfs.img",
		Size:     fmt.Sprintf("%d", p.Size),
	}))

	mkfsStageOptions := &osbuild.MkfsExt4StageOptions{
		UUID:  "2fe99653-f7ff-44fd-bea8-fa70107524fb",
		Label: "Anaconda",
	}
	lodevice := osbuild.NewLoopbackDevice(
		&osbuild.LoopbackDeviceOptions{
			Filename: "LiveOS/rootfs.img",
		},
	)

	devName := "device"
	devices := osbuild.Devices{devName: *lodevice}
	mkfsStage := osbuild.NewMkfsExt4Stage(mkfsStageOptions, devices)
	pipeline.AddStage(mkfsStage)

	inputName := "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/", inputName),
				To:   fmt.Sprintf("mount://%s/", devName),
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.installerPipeline.Name())
	copyStageMounts := &osbuild.Mounts{*osbuild.NewExt4Mount(devName, devName, "/")}
	copyStage := osbuild.NewCopyStage(copyStageOptions, copyStageInputs, &devices, copyStageMounts)
	pipeline.AddStage(copyStage)
	return pipeline
}
