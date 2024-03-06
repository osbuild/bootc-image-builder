package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

type OSTreeEncapsulate struct {
	Base
	filename string

	inputPipeline Pipeline
}

func NewOSTreeEncapsulate(buildPipeline Build, inputPipeline Pipeline, pipelinename string) *OSTreeEncapsulate {
	p := &OSTreeEncapsulate{
		Base:          NewBase(pipelinename, buildPipeline),
		inputPipeline: inputPipeline,
		filename:      "bootable-container.tar",
	}
	buildPipeline.addDependent(p)
	return p
}

func (p OSTreeEncapsulate) Filename() string {
	return p.filename
}

func (p *OSTreeEncapsulate) SetFilename(filename string) {
	p.filename = filename
}

func (p *OSTreeEncapsulate) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	encOptions := &osbuild.OSTreeEncapsulateStageOptions{
		Filename: p.Filename(),
	}
	encStage := osbuild.NewOSTreeEncapsulateStage(encOptions, p.inputPipeline.Name())
	pipeline.AddStage(encStage)

	return pipeline
}

func (p *OSTreeEncapsulate) getBuildPackages(Distro) []string {
	return []string{
		"rpm-ostree",
		"python3-pyyaml",
	}
}

func (p *OSTreeEncapsulate) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-tar"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
