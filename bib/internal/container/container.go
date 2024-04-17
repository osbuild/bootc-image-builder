package container

import (
	"bytes"
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

	cmd := exec.Command("podman", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %s container failed: %w\nstderr:\n%s", ref, err, stderr.String())
	}

	c = &Container{}
	c.id = strings.TrimSpace(stdout.String())
	// Ensure that the container is stopped when this function errors
	defer func() {
		if err != nil {
			if stopErr := c.Stop(); stopErr != nil {
				err = fmt.Errorf("%w\nstopping the container failed too: %s", err, stopErr)
			}
			c = nil
		}
	}()

	stdout.Reset()
	stderr.Reset()
	cmd = exec.Command("podman", "mount", c.id)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		err = fmt.Errorf("mounting %s container (%s) failed: %w\nstderr:\n%s", c.id, ref, runErr, stderr.String())
		return
	}
	c.root = strings.TrimSpace(stdout.String())

	return
}

// Stop stops the container. Since New() creates a container with --rm, this
// removes the container as well.
func (c *Container) Stop() error {
	cmd := exec.Command("podman", "stop", c.id)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stopping %s container failed: %w\nstderr:\n%s", c.id, err, stderr.String())
	}

	return nil
}

// Root returns the root directory of the container as available on the host.
func (c *Container) Root() string {
	return c.root
}
