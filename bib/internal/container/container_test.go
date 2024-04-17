package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testingImage = "registry.access.redhat.com/ubi9-micro:latest"

type containerInfo struct {
	State string `json:"State"`
	Image string `json:"Image"`
}

type invalidContainerCountError struct {
	id    string
	count int
}

func (e invalidContainerCountError) Error() string {
	return fmt.Sprintf("expected 1 container info for %s, got %d", e.id, e.count)
}

func getContainerInfo(id string) (containerInfo, error) {
	cmd := exec.Command("podman", "ps", "--filter", "id="+id, "--format", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return containerInfo{}, fmt.Errorf("checking status of %s failed: %w\nstderr:\n%s", id, err, stderr.String())
	}

	var infos []containerInfo
	if err := json.Unmarshal(stdout.Bytes(), &infos); err != nil {
		return containerInfo{}, fmt.Errorf("unmarshalling %s info failed: %w\nstdout:\n%s", id, err, stdout.String())
	}

	if len(infos) != 1 {
		return containerInfo{}, invalidContainerCountError{id: id, count: len(infos)}
	}

	return infos[0], nil
}

func TestNew(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test; not running as root")
	}

	c, err := New(testingImage)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = c.Stop()
		assert.NoError(t, err)

		// double-check that the container indeed doesn't exist
		_, infoErr := getContainerInfo(c.id)
		assert.ErrorIs(t, infoErr, invalidContainerCountError{id: c.id, count: 0})
	})

	info, err := getContainerInfo(c.id)
	require.NoError(t, err)
	assert.Equal(t, testingImage, info.Image)
	assert.Equal(t, "running", info.State)

	root := c.Root()
	osRelease, err := os.ReadFile(path.Join(root, "etc/os-release"))
	require.NoError(t, err)

	assert.Contains(t, string(osRelease), `ID="rhel"`)
}
