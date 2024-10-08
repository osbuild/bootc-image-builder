package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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

func makeFakeRunSecrets(t *testing.T) string {
	fakeSecretsRoot := filepath.Join(t.TempDir(), "/run/secrets")
	fakeSecretsFile := filepath.Join(fakeSecretsRoot, "/etc-pki-entitlement/12345.pem")
	err := os.MkdirAll(filepath.Dir(fakeSecretsFile), 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(fakeSecretsFile), []byte("very-secret"), 0600)
	require.NoError(t, err)

	restore := MockSecretDirSrc(fakeSecretsRoot)
	t.Cleanup(restore)

	return fakeSecretsRoot
}

func TestSecretsAvailableInsideContainer(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test; not running as root")
	}

	// fake /run/secrets as we would have it when running bib in a
	// subscribed machine
	makeFakeRunSecrets(t)

	cnt, err := New(testingImage)
	require.NoError(t, err)
	defer cnt.Stop()

	// ensure secrets are available when running "cat" inside the container
	secretContent, err := cnt.ReadFile("/run/secrets/etc-pki-entitlement/12345.pem")
	require.NoError(t, err)
	assert.Equal(t, "very-secret", string(secretContent))

	// ensure other /run content is available still
	_, err = cnt.ReadFile("/run/motd")
	require.NoError(t, err)

	// ensure secrets are available when accessing the container root
	root := cnt.Root()
	secretContent, err = os.ReadFile(filepath.Join(root, "/run/secrets/etc-pki-entitlement/12345.pem"))
	require.NoError(t, err)
	assert.Equal(t, "very-secret", string(secretContent))
}

func TestReadFile(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test; not running as root")
	}

	c, err := New(testingImage)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = c.Stop()
		assert.NoError(t, err)
	})

	content, err := c.ReadFile("/etc/os-release")
	require.NoError(t, err)
	require.Contains(t, string(content), `ID="rhel"`)
}

func TestCopyInto(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test; not running as root")
	}

	tmpdir := t.TempDir()
	testfile := path.Join(tmpdir, "testfile")
	require.NoError(t, os.WriteFile(testfile, []byte("Hello, world!"), 0644))

	c, err := New(testingImage)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = c.Stop()
		assert.NoError(t, err)
	})

	err = c.CopyInto(testfile, "/testfile")
	require.NoError(t, err)

	root := c.Root()
	testfileInContainer := path.Join(root, "testfile")
	testfileContent, err := os.ReadFile(testfileInContainer)
	require.NoError(t, err)
	require.Equal(t, "Hello, world!", string(testfileContent))
}

func makeFakePodman(t *testing.T, content string) {
	tmpdir := t.TempDir()
	t.Setenv("PATH", tmpdir+":"+os.Getenv("PATH"))

	err := os.WriteFile(filepath.Join(tmpdir, "podman"), []byte(content), 0755)
	assert.NoError(t, err)
}
func TestNewFakedUnhappy(t *testing.T) {
	fakePodman := `#!/bin/sh
if [ "$1" = "mount" ]; then
    >&2 echo "forced-crash"
    exit 2
fi
exec /usr/bin/podman "$@"
`
	makeFakePodman(t, fakePodman)
	_, err := New(testingImage)
	assert.ErrorContains(t, err, fmt.Sprintf("mounting %s container failed: ", testingImage))
	assert.ErrorContains(t, err, "stderr:\nforced-crash")
}

func TestRootfsTypeHappy(t *testing.T) {
	for _, tc := range []string{"", "ext4", "xfs"} {
		jsonStr := "{}"
		if tc != "" {
			jsonStr = fmt.Sprintf(`{"filesystem": {"root": {"type": "%s"}}}`, tc)
		}
		makeFakePodman(t, fmt.Sprintf(`#!/bin/sh
echo '%s'
`, jsonStr))
		cnt := Container{}
		rootfs, err := cnt.DefaultRootfsType()
		assert.NoError(t, err)
		assert.Equal(t, tc, rootfs)
	}
}

func TestRootfsTypeSad(t *testing.T) {
	for _, tc := range []string{"ext1"} {
		jsonStr := fmt.Sprintf(`{"filesystem": {"root": {"type": "%s"}}}`, tc)
		makeFakePodman(t, fmt.Sprintf(`#!/bin/sh
echo '%s'
`, jsonStr))
		cnt := Container{}
		_, err := cnt.DefaultRootfsType()
		assert.ErrorContains(t, err, "unsupported root filesystem type: ext1, supported: ")
	}
}
