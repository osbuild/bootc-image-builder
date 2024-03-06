package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// A QCOW2 turns a raw image file into qcow2 image.
type QCOW2 struct {
	Base
	filename string
	Compat   string

	imgPipeline FilePipeline
}

func (p QCOW2) Filename() string {
	return p.filename
}

func (p *QCOW2) SetFilename(filename string) {
	p.filename = filename
}

// NewQCOW2 createsa new QCOW2 pipeline. imgPipeline is the pipeline producing the
// raw image. The pipeline name is the name of the new pipeline. Filename is the name
// of the produced qcow2 image.
func NewQCOW2(buildPipeline Build, imgPipeline FilePipeline) *QCOW2 {
	p := &QCOW2{
		Base:        NewBase("qcow2", buildPipeline),
		imgPipeline: imgPipeline,
		filename:    "image.qcow2",
	}
	// qcow2 can run outside the build pipeline for e.g. "bib"
	if buildPipeline != nil {
		buildPipeline.addDependent(p)
	} else {
		imgPipeline.Manifest().addPipeline(p)
	}
	return p
}

func (p *QCOW2) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewQEMUStage(
		osbuild.NewQEMUStageOptions(p.Filename(),
			osbuild.QEMUFormatQCOW2,
			osbuild.QCOW2Options{
				Compat: p.Compat,
			}),
		osbuild.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename()),
	))

	return pipeline
}

func (p *QCOW2) getBuildPackages(Distro) []string {
	return []string{"qemu-img"}
}

func (p *QCOW2) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-qemu-disk"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
