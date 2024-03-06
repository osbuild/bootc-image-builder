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

type OSTreeContainer struct {
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

	OSVersion              string
	ExtraContainerPackages rpmmd.PackageSet // FIXME: this is never read
	ContainerLanguage      string
	Filename               string
}

func NewOSTreeContainer(ref string) *OSTreeContainer {
	return &OSTreeContainer{
		Base:      NewBase("ostree-container"),
		OSTreeRef: ref,
	}
}

func (img *OSTreeContainer) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload
	osPipeline.OSTreeRef = img.OSTreeRef
	osPipeline.OSTreeParent = img.OSTreeParent

	commitPipeline := manifest.NewOSTreeCommit(buildPipeline, osPipeline, img.OSTreeRef)
	commitPipeline.OSVersion = img.OSVersion

	nginxConfigPath := "/etc/nginx.conf"
	listenPort := "8080"

	serverPipeline := manifest.NewOSTreeCommitServer(
		buildPipeline,
		img.Platform,
		repos,
		commitPipeline,
		nginxConfigPath,
		listenPort,
	)
	serverPipeline.Language = img.ContainerLanguage

	containerPipeline := manifest.NewOCIContainer(buildPipeline, serverPipeline)
	containerPipeline.Cmd = []string{"nginx", "-c", nginxConfigPath}
	containerPipeline.ExposedPorts = []string{listenPort}
	containerPipeline.SetFilename(img.Filename)
	artifact := containerPipeline.Export()

	return artifact, nil
}
