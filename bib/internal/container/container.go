package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/osbuild/bootc-image-builder/bib/internal/util"
)

// Container is a simpler wrapper around a running podman container.
// This type isn't meant as a general-purpose container management tool, but
// as an opinonated library for bootc-image-builder.
type Container struct {
	id   string
	root string
}

// New creates a new running container from the given image reference.
//
// NB:
// - --net host is used to make networking work in a nested container
// - /run/secrets is mounted from the host to make sure RHSM credentials are available
func New(ref string) (*Container, error) {
	const secretDir = "/run/secrets"
	secretVolume := fmt.Sprintf("%s:%s", secretDir, secretDir)

	args := []string{
		"run",
		"--rm",
		"--init", // If sleep infinity is run as PID 1, it doesn't get signals, thus we cannot easily stop the container
		"--detach",
		"--net", "host", // Networking in a nested container doesn't work without re-using this container's network
		"--entrypoint", "sleep", // The entrypoint might be arbitrary, so let's just override it with sleep, we don't want to run anything
	}

	// Re-mount the secret directory if it exists
	if _, err := os.Stat(secretDir); err == nil {
		args = append(args, "--volume", secretVolume)
	}

	args = append(args, ref, "infinity")

	output, err := exec.Command("podman", args...).Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("running %s container failed: %w\nstderr:\n%s", ref, e, e.Stderr)
		}
		return nil, fmt.Errorf("running %s container failed with generic error: %w", ref, err)
	}

	c := &Container{}
	c.id = strings.TrimSpace(string(output))
	// Ensure that the container is stopped when this function errors
	defer func() {
		if err != nil {
			if stopErr := c.Stop(); stopErr != nil {
				err = fmt.Errorf("%w\nstopping the container failed too: %s", err, stopErr)
			}
			c = nil
		}
	}()

	output, err = exec.Command("podman", "mount", c.id).Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("mounting %s container failed: %w\nstderr:\n%s", ref, err, err.Stderr)
		}
		return nil, fmt.Errorf("mounting %s container failed with generic error: %w", ref, err)
	}
	c.root = strings.TrimSpace(string(output))

	return c, err
}

// Stop stops the container. Since New() creates a container with --rm, this
// removes the container as well.
func (c *Container) Stop() error {
	if output, err := exec.Command("podman", "stop", c.id).CombinedOutput(); err != nil {
		return fmt.Errorf("stopping %s container failed: %w\noutput:\n%s", c.id, err, output)
	}
	// when the container is stopped by podman it may not honor the "--rm"
	// that was passed in `New()` so manually remove the container here if it is still available
	if output, err := exec.Command("podman", "rm", "--ignore", c.id).CombinedOutput(); err != nil {
		return fmt.Errorf("removing %s container failed: %w\noutput:\n%s", c.id, err, output)
	}

	return nil
}

// Root returns the root directory of the container as available on the host.
func (c *Container) Root() string {
	return c.root
}

// Reads a file from the container
func (c *Container) ReadFile(path string) ([]byte, error) {
	output, err := exec.Command("podman", "exec", c.id, "cat", path).Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("reading %s from %s container failed: %w\nstderr:\n%s", path, c.id, err, err.Stderr)
		}
		return nil, fmt.Errorf("reading %s from %s container failed with generic error: %w", path, c.id, err)
	}

	return output, nil
}

// CopyInto copies a file into the container.
func (c *Container) CopyInto(src, dest string) error {
	if output, err := exec.Command("podman", "cp", src, c.id+":"+dest).CombinedOutput(); err != nil {
		return fmt.Errorf("copying %s into %s container failed: %w\noutput:\n%s", src, c.id, err, output)
	}

	return nil
}

func (c *Container) ExecArgv() []string {
	return []string{"podman", "exec", "-i", c.id}
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

func (c *Container) InitDepSolveDNF() ([]string, error) {
	if err := c.CopyInto("/usr/libexec/osbuild-depsolve-dnf", "/osbuild-depsolve-dnf"); err != nil {
		return nil, fmt.Errorf("cannot prepare depsolve in the container: %w", err)
	}
	// XXX: hardcoded python3.12
	if err := c.CopyInto("/usr/lib//python3.12/site-packages/osbuild", "/"); err != nil {
		return nil, fmt.Errorf("cannot prepare depsolve python-modules in the container: %w", err)
	}
	return append(c.ExecArgv(), "/osbuild-depsolve-dnf"), nil
}

// DefaultRootfsType returns the default rootfs type (e.g. "ext4") as
// specified by the bootc container install configuration. An empty
// string is valid and means the container sets no default.
func (c *Container) DefaultRootfsType() (string, error) {
	output, err := exec.Command("podman", "exec", c.id, "bootc", "install", "print-configuration").Output()
	if err != nil {
		return "", fmt.Errorf("failed to run bootc install print-configuration: %w", util.OutputErr(err))
	}

	var bootcConfig struct {
		Filesystem struct {
			Root struct {
				Type string `json:"type"`
			} `json:"root"`
		} `json:"filesystem"`
	}

	if err := json.Unmarshal(output, &bootcConfig); err != nil {
		return "", fmt.Errorf("failed to unmarshal bootc configuration: %w", err)
	}

	// filesystem.root.type is the preferred way instead of the old root-fs-type top-level key.
	// See https://github.com/containers/bootc/commit/558cd4b1d242467e0ffec77fb02b35166469dcc7
	fsType := bootcConfig.Filesystem.Root.Type
	// Note that these are the only filesystems that the "images" library
	// knows how to handle, i.e. how to construct the required osbuild
	// stages for.
	// TODO: move this into a helper in "images" so that there is only
	// a single place that needs updating when we add e.g. btrfs or
	// bcachefs
	supportedFS := []string{"ext4", "xfs", "btrfs"}

	if fsType == "" {
		return "", nil
	}
	if !slices.Contains(supportedFS, fsType) {
		return "", fmt.Errorf("unsupported root filesystem type: %s, supported: %s", fsType, strings.Join(supportedFS, ", "))
	}

	return fsType, nil
}
