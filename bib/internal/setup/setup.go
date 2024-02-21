package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/osbuild/bootc-image-builder/bib/internal/podmanutil"
	"github.com/osbuild/bootc-image-builder/bib/internal/util"
	"golang.org/x/sys/unix"
)

var binfmtMiscPath = "/proc/sys/fs/binfmt_misc"

// XXX: binfmt_misc is not namespaced so this alters the host, see also
// https://lore.kernel.org/all/20211028103114.2849140-2-brauner@kernel.org/
var binFmtMiscHandlers = map[string]string{
	"amd64": `:qemu-bib-aarch64:M::\x7f\x45\x4c\x46\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\xb7\x00:\xff\xff\xff\xff\xff\xff\xff\x00\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff\xff:/usr/bin/qemu-aarch64-static:OCPF`,

	"arm64": `:qemu-bib-x86_64:M::\x7f\x45\x4c\x46\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00:\xff\xff\xff\xff\xff\xfe\xfe\xfc\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff\xff:/usr/bin/qemu-x86_64-static:OCPF`,
}

func setupInternalQemu() error {
	handler, ok := binFmtMiscHandlers[runtime.GOARCH]
	if !ok {
		return nil
	}

	if output, err := exec.Command("mount", "-t", "binfmt_misc", "none", binfmtMiscPath).CombinedOutput(); err != nil {
		return fmt.Errorf("mounting binfmt misc failed: %v\noutput:\n%s", err, string(output))
	}
	// XXX: unregistering existing handlers can be done via writing a -1
	// to existing ones and in theory the last one added wins so we could
	// cleanup aftre the build is finished
	f, err := os.OpenFile(filepath.Join(binfmtMiscPath, "register"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	// binfmt-misc is global, so EEXIST is fine, it mean bib ran before
	if _, err := f.WriteString(handler); err != nil && !os.IsExist(err) {
		return err
	}

	return f.Close()
}

// EnsureEnvironment mutates external filesystem state as necessary
// to run in a container environment.  This function is idempotent.
func EnsureEnvironment() error {
	if forceInternal := os.Getenv("BIB_FORCE_INTERNAL_QEMU"); forceInternal == "1" {
		if err := setupInternalQemu(); err != nil {
			return err
		}
	}

	osbuildPath := "/usr/bin/osbuild"
	if util.IsMountpoint(osbuildPath) {
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
	// Create a bind mount into our target location; we can't copy it because
	// again we have to perserve the SELinux label.
	if err := util.RunCmdSync("mount", "--bind", destPath, osbuildPath); err != nil {
		return err
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
