package container

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/dnfjson"

	"github.com/osbuild/bootc-image-builder/bib/internal/source"
)

// InitDNF initializes dnf in the container. This is necessary when
// the caller wants to read the image's dnf repositories, but they are
// not static, but rather configured by dnf dynamically. The primaru
// use-case for this is RHEL and subscription-manager.
//
// The implementation is simple: We just run plain `dnf` in the
// container so that the subscription-manager gets initialized. For
// compatibility with both dnf and dnf5 we cannot just run "dnf" as
// dnf5 will error and do nothing in this case. So we use "dnf check
// --duplicates" as this is fast on both dnf4/dnf5 (just doing "dnf5
// check" without arguments takes around 25s so that is not a great
// option).
func (c *Container) InitDNF() error {
	if output, err := exec.Command("podman", "exec", c.id, "dnf", "check", "--duplicates").CombinedOutput(); err != nil {
		return fmt.Errorf("initializing dnf in %s container failed: %w\noutput:\n%s", c.id, err, string(output))
	}

	return nil
}

func (cnt *Container) injectDNFJson() ([]string, error) {
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

func (cnt *Container) NewContainerSolver(cacheRoot string, architecture arch.Arch, sourceInfo *source.Info) (*dnfjson.Solver, error) {
	depsolverCmd, err := cnt.injectDNFJson()
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
