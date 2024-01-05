package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/osbuild/bootc-image-builder/bib/internal/podmanutil"
	"github.com/osbuild/bootc-image-builder/bib/internal/utils"
	"golang.org/x/sys/unix"
)

// EnsureEnvironment mutates external filesystem state as necessary
// to run in a container environment.  This function is idempotent.
func EnsureEnvironment() error {
	osbuildPath := "/usr/bin/osbuild"
	if utils.IsMountpoint(osbuildPath) {
		return nil
	}

	// A hardcoded security label from Fedora derivatives
	// TODO: Avoid hardcoding this by using either host policy lookup
	// Or eventually depend on privileged containers just having this capability.
	//
	// We need this in order to get `install_t` that has `CAP_MAC_ADMIN` for creating SELinux
	// labels unknown to the host.
	//
	// Note that the transition to `install_t` must happen at this point. Osbuild stages run in `bwrap` that creates
	// a nosuid, no_new_privs environment. In such an environment, we cannot transition from `unconfined_t` to `install_t`,
	// because we would get more privileges.
	installType := "system_u:object_r:install_exec_t:s0"
	// Where we dump temporary files; this must be an overlayfs as we cannot
	// write security contexts on overlayfs.
	runTmp := "/run/osbuild/"

	if err := os.MkdirAll(runTmp, 0o755); err != nil {
		return err
	}
	if !utils.IsMountpoint(runTmp) {
		if err := exec.Command("mount", "-t", "tmpfs", "tmpfs", runTmp).Run(); err != nil {
			return fmt.Errorf("failed to mount tmpfs to %s: %w", runTmp, err)
		}
	}
	src, err := os.Open("/usr/bin/osbuild")
	if err != nil {
		return err
	}
	defer src.Close()
	destPath := filepath.Join(runTmp, "osbuild")
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	if err := dst.Chmod(0o755); err != nil {
		return err
	}
	dst.Close()
	if err := exec.Command("chcon", installType, destPath).Run(); err != nil {
		return fmt.Errorf("failed to chcon: %w", err)
	}

	// Create a bind mount into our target location; we can't copy it because
	// again we have to perserve the SELinux label.
	if err := exec.Command("mount", "--bind", destPath, osbuildPath).Run(); err != nil {
		return fmt.Errorf("failed to bind mount to %s: %w", osbuildPath, err)
	}
	return nil
}

// Validate checks that the environment is supported (e.g. caller set up the
// container correctly)
func Validate() error {
	isRootless, err := podmanutil.IsRootless()
	if err != nil {
		return fmt.Errorf("checking rootless: %w", err)
	}
	if isRootless {
		return fmt.Errorf("this command must be run in rootful (not rootless) podman")
	}

	// Having /sys be writable is an easy to check proxy for privileges; more effective
	// is really looking for CAP_SYS_ADMIN, but that involves more Go libraries.
	var stvfsbuf unix.Statfs_t
	if err := unix.Statfs("/sys", &stvfsbuf); err != nil {
		return fmt.Errorf("failed to stat /sys: %w", err)
	}
	if (stvfsbuf.Flags & unix.ST_RDONLY) > 0 {
		return fmt.Errorf("this command requires a privileged container")
	}

	return nil
}
