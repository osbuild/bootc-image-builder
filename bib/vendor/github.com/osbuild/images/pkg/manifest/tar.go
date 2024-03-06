package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// A Tar represents the contents of another pipeline in a tar file
type Tar struct {
	Base
	filename string

	Format   osbuild.TarArchiveFormat
	RootNode osbuild.TarRootNode
	Paths    []string
	ACLs     *bool
	SELinux  *bool
	Xattrs   *bool

	inputPipeline Pipeline
}

func (p Tar) Filename() string {
	return p.filename
}

func (p *Tar) SetFilename(filename string) {
	p.filename = filename
}

// NewTar creates a new TarPipeline. The inputPipeline represents the
// filesystem tree which will be the contents of the tar file. The pipelinename
// is the name of the pipeline. The filename is the name of the output tar file.
func NewTar(buildPipeline Build, inputPipeline Pipeline, pipelinename string) *Tar {
	p := &Tar{
		Base:          NewBase(pipelinename, buildPipeline),
		inputPipeline: inputPipeline,
		filename:      "image.tar",
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *Tar) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	tarOptions := &osbuild.TarStageOptions{
		Filename: p.Filename(),
		Format:   p.Format,
		ACLs:     p.ACLs,
		SELinux:  p.SELinux,
		Xattrs:   p.Xattrs,
		RootNode: p.RootNode,
		Paths:    p.Paths,
	}
	tarStage := osbuild.NewTarStage(tarOptions, p.inputPipeline.Name())
	pipeline.AddStage(tarStage)

	return pipeline
}

func (p *Tar) getBuildPackages(Distro) []string {
	return []string{"tar"}
}

func (p *Tar) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-tar"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
