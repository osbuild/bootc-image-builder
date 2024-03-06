package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// An OCIContainer represents an OCI container, containing a filesystem
// tree created by another Pipeline.
type OCIContainer struct {
	Base
	filename     string
	Cmd          []string
	ExposedPorts []string

	treePipeline TreePipeline
}

func (p OCIContainer) Filename() string {
	return p.filename
}

func (p *OCIContainer) SetFilename(filename string) {
	p.filename = filename
}

func NewOCIContainer(buildPipeline Build, treePipeline TreePipeline) *OCIContainer {
	p := &OCIContainer{
		Base:         NewBase("container", buildPipeline),
		treePipeline: treePipeline,
		filename:     "oci-archive.tar",
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *OCIContainer) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	options := &osbuild.OCIArchiveStageOptions{
		Architecture: p.treePipeline.Platform().GetArch().String(),
		Filename:     p.Filename(),
		Config: &osbuild.OCIArchiveConfig{
			Cmd:          p.Cmd,
			ExposedPorts: p.ExposedPorts,
		},
	}
	baseInput := osbuild.NewTreeInput("name:" + p.treePipeline.Name())
	inputs := &osbuild.OCIArchiveStageInputs{Base: baseInput}
	pipeline.AddStage(osbuild.NewOCIArchiveStage(options, inputs))

	return pipeline
}

func (p *OCIContainer) getBuildPackages(Distro) []string {
	return []string{"tar"}
}

func (p *OCIContainer) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-tar"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
