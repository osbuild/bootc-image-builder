package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/sirupsen/logrus"

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
func Validate(targetArch string) error {
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

	// Try to run the cross arch binary
	if err := validateCanRunTargetArch(targetArch); err != nil {
		return fmt.Errorf("cannot run binary in target arch: %w", err)
	}

	return nil
}

// ValidateHasContainerStorageMounted checks that the hostcontainer storage
// is mounted inside the container
func ValidateHasContainerStorageMounted() error {
	// Just look for the overlay backend, which we expect by default.
	// In theory, one could be using a different backend, but we don't
	// really need to worry about this right now.  If it turns out
	// we do need to care, then we can probably handle this by
	// just trying to query the image.
	overlayPath := "/var/lib/containers/storage/overlay"
	if _, err := os.Stat(overlayPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cannot find %q (missing -v /var/lib/containers/storage:/var/lib/containers/storage mount?)", overlayPath)
		}
		return fmt.Errorf("failed to stat %q: %w", overlayPath, err)
	}
	return nil
}

func validateCanRunTargetArch(targetArch string) error {
	if targetArch == runtime.GOARCH || targetArch == "" {
		return nil
	}

	canaryCmd := fmt.Sprintf("bib-canary-%s", targetArch)
	if _, err := exec.LookPath(canaryCmd); err != nil {
		// we could error here but in principle with a working qemu-user
		// any arch should work so let's just warn. the common case
		// (arm64/amd64) is covered properly
		logrus.Warningf("cannot check architecture support for %v: no canary binary found", targetArch)
		return nil
	}
	output, err := exec.Command(canaryCmd).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot run canary binary for %q, do you have 'qemu-user-static' installed?\n%s", targetArch, err)
	}
	if string(output) != "ok\n" {
		return fmt.Errorf("internal error: unexpected output from cross-architecture canary: %q", string(output))
	}

	return nil
}

func ValidateHasContainerTags(imgref string) error {
	output, err := exec.Command("podman", "image", "inspect", imgref, "--format", "{{.Labels}}").Output()
	if err != nil {
		return fmt.Errorf(`failed to inspect the image: %w
bootc-image-builder no longer pulls images, make sure to pull it before running bootc-image-builder:
    sudo podman pull %s`, util.OutputErr(err), imgref)
	}

	tags := string(output)
	if !strings.Contains(tags, "containers.bootc:1") {
		return fmt.Errorf("image %s is not a bootc image", imgref)
	}

	return nil
}
