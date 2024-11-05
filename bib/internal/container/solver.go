package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/dnfjson"

	"github.com/osbuild/bootc-image-builder/bib/internal/source"
)

func forceSymlink(symlinkPath, target string) error {
	if output, err := exec.Command("ln", "-sf", target, symlinkPath).CombinedOutput(); err != nil {
		return fmt.Errorf("cannot run ln: %w, output:\n%s", err, output)
	}
	return nil
}

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

func (cnt *Container) hasRunSecrets() bool {
	_, err := os.Stat(filepath.Join(cnt.root, "/run/secrets/redhat.repo"))
	return err == nil
}

// setupRunSecretsBindMount will synthesise a /run/secrets dir
// in the container root
func (cnt *Container) setupRunSecrets() error {
	if cnt.hasRunSecrets() {
		return nil
	}
	dst := filepath.Join(cnt.root, "/run/secrets")
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// We cannot just bind mount here because
	// /usr/share/rhel/secrets contains a bunch of relative symlinks
	// that will point to the container root not the host when resolved
	// from the outside (via the host container mount).
	//
	// So instead of bind mounting we create a copy of the
	// /run/secrets/ - they are static so that should be fine.
	//
	// We want to support /usr/share/rhel/secrets too to be able
	// to run "bootc-image-builder manifest" directly on the host
	// (which is useful for e.g. composer).
	for _, src := range []string{"/run/secrets", "/usr/share/rhel/secrets"} {
		if st, err := os.Stat(src); err != nil || !st.IsDir() {
			continue
		}

		dents, err := filepath.Glob(src + "/*")
		if err != nil {
			return err
		}
		for _, ent := range dents {
			// Check if the target file actually exists (i.e. for
			// symlinks that they are valid) and only copy if so.
			// This covers unsubscribed machines.
			if _, err := os.Stat(ent); err != nil {
				continue
			}

			// Note the use of "-L" here to dereference/copy links
			if output, err := exec.Command("cp", "-rvL", ent, dst).CombinedOutput(); err != nil {
				return fmt.Errorf("failed to setup /run/secrets: %w, output:\n%s", err, string(output))
			}
		}
	}

	// workaround broken containers (like f41) that use absolute symlinks
	// to point to the entitlements-host and rhsm-host, they need to be
	// relative so that the "SetRootdir()" from the resolver works, i.e.
	// they need to point into the mounted container.
	symlink := filepath.Join(cnt.root, "/etc/pki/entitlement-host")
	target := "../../run/secrets/etc-pki-entitlement"
	if err := forceSymlink(symlink, target); err != nil {
		return err
	}
	symlink = filepath.Join(cnt.root, "/etc/rhsm-host")
	target = "../run/secrets/rhsm"
	if err := forceSymlink(symlink, target); err != nil {
		return err
	}
	return nil
}

func (cnt *Container) NewContainerSolver(cacheRoot string, architecture arch.Arch, sourceInfo *source.Info) (*dnfjson.Solver, error) {
	solver := dnfjson.NewSolver(
		sourceInfo.OSRelease.PlatformID,
		sourceInfo.OSRelease.VersionID,
		architecture.String(),
		fmt.Sprintf("%s-%s", sourceInfo.OSRelease.ID, sourceInfo.OSRelease.VersionID),
		cacheRoot)

	// we copy the data directly into the cnt.root, no need to
	// cleanup here because podman stop will remove the dir
	if err := cnt.setupRunSecrets(); err != nil {
		return nil, err
	}
	solver.SetRootDir(cnt.root)
	return solver, nil
}
