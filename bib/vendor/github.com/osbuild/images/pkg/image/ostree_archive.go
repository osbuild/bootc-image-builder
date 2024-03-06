package image

import (
	"math/rand"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type OSTreeArchive struct {
	Base
	Platform         platform.Platform
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Workload         workload.Workload

	// OSTreeParent specifies the source for an optional parent commit for the
	// new commit being built.
	OSTreeParent *ostree.SourceSpec

	// OSTreeRef is the ref of the commit that will be built.
	OSTreeRef string

	OSVersion string
	Filename  string

	InstallWeakDeps bool

	BootContainer bool
}

func NewOSTreeArchive(ref string) *OSTreeArchive {
	return &OSTreeArchive{
		Base:            NewBase("ostree-archive"),
		OSTreeRef:       ref,
		InstallWeakDeps: true,
	}
}

func (img *OSTreeArchive) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload
	osPipeline.OSTreeParent = img.OSTreeParent
	osPipeline.OSTreeRef = img.OSTreeRef
	osPipeline.InstallWeakDeps = img.InstallWeakDeps

	ostreeCommitPipeline := manifest.NewOSTreeCommit(buildPipeline, osPipeline, img.OSTreeRef)
	ostreeCommitPipeline.OSVersion = img.OSVersion

	var artifact *artifact.Artifact
	if img.BootContainer {
		osPipeline.Bootupd = true
		encapsulatePipeline := manifest.NewOSTreeEncapsulate(buildPipeline, ostreeCommitPipeline, "ostree-encapsulate")
		encapsulatePipeline.SetFilename(img.Filename)
		artifact = encapsulatePipeline.Export()
	} else {
		tarPipeline := manifest.NewTar(buildPipeline, ostreeCommitPipeline, "commit-archive")
		tarPipeline.SetFilename(img.Filename)
		artifact = tarPipeline.Export()
	}

	return artifact, nil
}
