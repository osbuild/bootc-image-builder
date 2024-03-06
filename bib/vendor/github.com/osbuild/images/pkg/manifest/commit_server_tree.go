package manifest

import (
	"path/filepath"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

// An OSTreeCommitServer contains an nginx server serving
// an embedded ostree commit.
type OSTreeCommitServer struct {
	Base
	// Packages to install in addition to the ones required by the
	// pipeline.
	ExtraPackages []string
	// Extra repositories to install packages from
	ExtraRepos []rpmmd.RepoConfig
	// TODO: should this be configurable?
	Language string

	platform        platform.Platform
	repos           []rpmmd.RepoConfig
	packageSpecs    []rpmmd.PackageSpec
	commitPipeline  *OSTreeCommit
	nginxConfigPath string
	listenPort      string
}

// NewOSTreeCommitServer creates a new pipeline. The content
// is built from repos and packages, which must contain nginx. commitPipeline
// is a pipeline producing an ostree commit to be served. nginxConfigPath
// is the path to the main nginx config file and listenPort is the port
// nginx will be listening on.
func NewOSTreeCommitServer(buildPipeline Build,
	platform platform.Platform,
	repos []rpmmd.RepoConfig,
	commitPipeline *OSTreeCommit,
	nginxConfigPath,
	listenPort string) *OSTreeCommitServer {
	name := "container-tree"
	p := &OSTreeCommitServer{
		Base:            NewBase(name, buildPipeline),
		platform:        platform,
		repos:           filterRepos(repos, name),
		commitPipeline:  commitPipeline,
		nginxConfigPath: nginxConfigPath,
		listenPort:      listenPort,
		Language:        "en_US",
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *OSTreeCommitServer) getPackageSetChain(Distro) []rpmmd.PackageSet {
	// FIXME: container package is defined here
	packages := []string{"nginx"}
	return []rpmmd.PackageSet{
		{
			Include:         append(packages, p.ExtraPackages...),
			Repositories:    append(p.repos, p.ExtraRepos...),
			InstallWeakDeps: true,
		},
	}
}

func (p *OSTreeCommitServer) getBuildPackages(Distro) []string {
	packages := []string{
		"rpm",
		"rpm-ostree",
	}
	return packages
}

func (p *OSTreeCommitServer) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *OSTreeCommitServer) serializeStart(packages []rpmmd.PackageSpec, _ []container.Spec, _ []ostree.CommitSpec) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = packages
}

func (p *OSTreeCommitServer) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.packageSpecs = nil
}

func (p *OSTreeCommitServer) serialize() osbuild.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(p.repos), osbuild.NewRpmStageSourceFilesInputs(p.packageSpecs)))
	pipeline.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: p.Language}))

	htmlRoot := "/usr/share/nginx/html"
	repoPath := filepath.Join(htmlRoot, "repo")
	pipeline.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: repoPath}))

	pipeline.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repoPath},
		osbuild.NewOstreePullStageInputs("org.osbuild.pipeline", "name:"+p.commitPipeline.Name(), p.commitPipeline.ref),
	))

	// make nginx log and lib directories world writeable, otherwise nginx can't start in
	// an unprivileged container
	pipeline.AddStage(osbuild.NewChmodStage(chmodStageOptions("/var/log/nginx", "a+rwX", true)))
	pipeline.AddStage(osbuild.NewChmodStage(chmodStageOptions("/var/lib/nginx", "a+rwX", true)))

	pipeline.AddStage(osbuild.NewNginxConfigStage(nginxConfigStageOptions(p.nginxConfigPath, htmlRoot, p.listenPort)))

	return pipeline
}

func nginxConfigStageOptions(path, htmlRoot, listen string) *osbuild.NginxConfigStageOptions {
	// configure nginx to work in an unprivileged container
	cfg := &osbuild.NginxConfig{
		Listen: listen,
		Root:   htmlRoot,
		Daemon: common.ToPtr(false),
		PID:    "/tmp/nginx.pid",
	}
	return &osbuild.NginxConfigStageOptions{
		Path:   path,
		Config: cfg,
	}
}

func chmodStageOptions(path, mode string, recursive bool) *osbuild.ChmodStageOptions {
	return &osbuild.ChmodStageOptions{
		Items: map[string]osbuild.ChmodStagePathOptions{
			path: {Mode: mode, Recursive: recursive},
		},
	}
}

func (p *OSTreeCommitServer) Platform() platform.Platform {
	return p.platform
}
