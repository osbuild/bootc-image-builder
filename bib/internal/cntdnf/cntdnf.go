package cntdnf

import (
	"fmt"
	"path/filepath"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/dnfjson"

	"github.com/osbuild/bootc-image-builder/bib/internal/container"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
)

func injectDNFJson(cnt *container.Container) ([]string, error) {
	if err := cnt.CopyInto("/usr/libexec/osbuild-depsolve-dnf", "/osbuild-depsolve-dnf"); err != nil {
		return nil, fmt.Errorf("cannot prepare depsolve in the container: %w", err)
	}
	// copy the python module too
	globPath := "/usr/lib/*/site-packages/osbuild"
	matches, err := filepath.Glob(globPath)
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("cannot find osbuild python module in %q: %w", globPath, err)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("unexpected number of osbuild python module matches: %v", matches)
	}
	if err := cnt.CopyInto(matches[0], "/"); err != nil {
		return nil, fmt.Errorf("cannot prepare depsolve python-modules in the container: %w", err)
	}
	return append(cnt.ExecArgv(), "/osbuild-depsolve-dnf"), nil
}

func NewContainerSolver(cacheRoot string, cnt *container.Container, architecture arch.Arch, sourceInfo *source.Info) (*dnfjson.Solver, error) {
	depsolverCmd, err := injectDNFJson(cnt)
	if err != nil {
		return nil, fmt.Errorf("cannot inject depsolve into the container: %w", err)
	}

	solver := dnfjson.NewSolver(
		sourceInfo.OSRelease.PlatformID,
		sourceInfo.OSRelease.VersionID,
		architecture.String(),
		fmt.Sprintf("%s-%s", sourceInfo.OSRelease.ID, sourceInfo.OSRelease.VersionID),
		cacheRoot)
	solver.SetDNFJSONPath(depsolverCmd[0], depsolverCmd[1:]...)
	solver.SetRootDir("/")
	return solver, nil
}
