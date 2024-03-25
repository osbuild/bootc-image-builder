package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/moby/sys/mountinfo"
	"golang.org/x/sys/unix"

	"github.com/osbuild/bootc-image-builder/bib/internal/podmanutil"
	"github.com/osbuild/bootc-image-builder/bib/internal/util"
)

// EnsureEnvironment mutates external filesystem state as necessary
// to run in a container environment.  This function is idempotent.
func EnsureEnvironment(storePath string) error {
	osbuildPath := "/usr/bin/osbuild"
	if util.IsMountpoint(osbuildPath) {
		return nil
	}

	// Forcibly label the store to ensure we're not grabbing container labels
	rootType := "system_u:object_r:root_t:s0"
	// This papers over the lack of ensuring correct labels for the /ostree root
	// in the existing pipeline
	if err := util.RunCmdSync("chcon", rootType, storePath); err != nil {
		return err
	}

	// A hardcoded security label from Fedora derivatives for osbuild
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
	if !util.IsMountpoint(runTmp) {
		if err := util.RunCmdSync("mount", "-t", "tmpfs", "tmpfs", runTmp); err != nil {
			return err
		}
	}
	destPath := filepath.Join(runTmp, "osbuild")
	if err := util.RunCmdSync("cp", "-p", "/usr/bin/osbuild", destPath); err != nil {
		return err
	}
	if err := util.RunCmdSync("chcon", installType, destPath); err != nil {
		return err
	}

	// Ensure we have devfs inside the container to get dynamic loop
	// loop devices inside the container.
	if err := util.RunCmdSync("mount", "-t", "devtmpfs", "devtmpfs", "/dev"); err != nil {
		return err
	}

	// Create a bind mount into our target location; we can't copy it because
	// again we have to perserve the SELinux label.
	if err := util.RunCmdSync("mount", "--bind", destPath, osbuildPath); err != nil {
		return err
	}
	// NOTE: Don't add new code here, do it before the bind mount which acts as the final success indicator

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

var insideContainer = func() (bool, error) {
	if err := exec.Command("systemd-detect-virt", "-c").Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// not running in a container, just exit
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, err
	}
	return false, nil
}

// ValidateHasContainerStorageMounted checks that the container storage
// is mounted inside the container
func ValidateHasContainerStorageMounted() error {
	inside, err := insideContainer()
	if err != nil {
		return err
	}
	if inside {
		return nil
	}

	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return err
	}
	defer f.Close()
	return validateHasContainerStorageMountedFromReader(f)
}

func validateHasContainerStorageMountedFromReader(r io.Reader) error {
	containersStorage := "/var/lib/containers/storage"
	containerStorageMountFound := false

	mnts, err := mountinfo.GetMountsFromReader(r, nil)
	if err != nil {
		return err
	}
	for _, mnt := range mnts {
		if mnt.Mountpoint != containersStorage {
			continue
		}
		containerStorageMountFound = true
		// on btrfs the containers storage might be on a subvolume
		// so we just compare the final part of the path
		if !strings.HasSuffix(mnt.Root, containersStorage) {
			return fmt.Errorf("cannot find suffix %q in mounted %q", containersStorage, mnt.Root)
		}
	}
	if !containerStorageMountFound {
		return fmt.Errorf("cannot find mount for %q", containersStorage)
	}

	return nil
}
