package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
func New(ref string) (c *Container, err error) {
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

	c = &Container{}
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

	return
}

// Stop stops the container. Since New() creates a container with --rm, this
// removes the container as well.
func (c *Container) Stop() error {
	if output, err := exec.Command("podman", "stop", c.id).CombinedOutput(); err != nil {
		return fmt.Errorf("stopping %s container failed: %w\noutput:\n%s", c.id, err, output)
	}
	// when the container is stopped by podman it will not honor the "--rm"
	// that was passed in `New()` so manually remove the container here.
	if output, err := exec.Command("podman", "rm", c.id).CombinedOutput(); err != nil {
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

// InitDNF initializes dnf in the container. This is necessary when the caller wants to read the image's dnf
// repositories, but they are not static, but rather configured by dnf dynamically. The primaru use-case for
// this is RHEL and subscription-manager.
//
// The implementation is simple: We just run plain `dnf` in the container.
func (c *Container) InitDNF() error {
	if output, err := exec.Command("podman", "exec", c.id, "dnf").CombinedOutput(); err != nil {
		return fmt.Errorf("initializing dnf in %s container failed: %w\noutput:\n%s", c.id, err, output)
	}

	return nil
}
