package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// A VMDK turns a raw image file or a raw ostree image file into vmdk image.
type VMDK struct {
	Base
	filename string

	imgPipeline Pipeline
}

func (p VMDK) Filename() string {
	return p.filename
}

func (p *VMDK) SetFilename(filename string) {
	p.filename = filename
}

// NewVMDK creates a new VMDK pipeline. imgPipeline is the pipeline producing the
// raw image. imgOstreePipeline is the pipeline producing the raw ostree image.
// Either imgPipeline or imgOStreePipeline are required, but not both at the same time.
// Filename is the name of the produced image.
func NewVMDK(buildPipeline Build, imgPipeline FilePipeline) *VMDK {
	p := &VMDK{
		Base:        NewBase("vmdk", buildPipeline),
		imgPipeline: imgPipeline,
		filename:    "image.vmdk",
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *VMDK) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewQEMUStage(
		osbuild.NewQEMUStageOptions(p.Filename(), osbuild.QEMUFormatVMDK, osbuild.VMDKOptions{
			Subformat: osbuild.VMDKSubformatStreamOptimized,
		}),
		osbuild.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Export().Filename()),
	))

	return pipeline
}

func (p *VMDK) getBuildPackages(Distro) []string {
	return []string{"qemu-img"}
}

func (p *VMDK) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-vmdk"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
