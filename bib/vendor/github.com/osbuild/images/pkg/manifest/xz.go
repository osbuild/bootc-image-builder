package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// The XZ pipeline compresses a raw image file using xz.
type XZ struct {
	Base
	filename string

	imgPipeline FilePipeline
}

func (p XZ) Filename() string {
	return p.filename
}

func (p *XZ) SetFilename(filename string) {
	p.filename = filename
}

// NewXZ creates a new XZ pipeline. imgPipeline is the pipeline producing the
// raw image that will be xz compressed.
func NewXZ(buildPipeline Build, imgPipeline FilePipeline) *XZ {
	p := &XZ{
		Base:        NewBase("xz", buildPipeline),
		filename:    "image.xz",
		imgPipeline: imgPipeline,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *XZ) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewXzStage(
		osbuild.NewXzStageOptions(p.Filename()),
		osbuild.NewXzStageInputs(osbuild.NewFilesInputPipelineObjectRef(p.imgPipeline.Name(), p.imgPipeline.Export().Filename(), nil)),
	))

	return pipeline
}

func (p *XZ) getBuildPackages(Distro) []string {
	return []string{"xz"}
}

func (p *XZ) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/xz"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
