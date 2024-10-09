package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/rpmmd"

	"github.com/osbuild/bootc-image-builder/bib/internal/source"
)

const (
	testingImage    = "registry.access.redhat.com/ubi9-micro:latest"
	dnfTestingImage = "registry.access.redhat.com/ubi9:latest"
)

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

func subscribeMachine(t *testing.T) (restore func()) {
	if _, err := exec.LookPath("subscription-manager"); err != nil {
		t.Skip("no subscription-manager found")
		return func() {}
	}

	matches, err := filepath.Glob("/etc/pki/entitlement/*.pem")
	if err == nil && len(matches) > 0 {
		return func() {}
	}

	rhsmOrg := os.Getenv("RHSM_ORG")
	rhsmActivationKey := os.Getenv("RHSM_ACTIVATION_KEY")
	if rhsmOrg == "" || rhsmActivationKey == "" {
		t.Skip("no RHSM_{ORG,ACTIVATION_KEY} env vars found")
		return func() {}
	}

	err = exec.Command("subscription-manager", "register",
		"--org", rhsmOrg,
		"--activationkey", rhsmActivationKey).Run()
	require.NoError(t, err)

	return func() {
		exec.Command("subscription-manager", "unregister").Run()
	}
}

func TestDNFInitGivesAccessToSubscribedContent(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test; not running as root")
	}
	if runtime.GOARCH != "amd64" {
		t.Skip("skipping test; only runs on x86_64")
	}

	restore := subscribeMachine(t)
	defer restore()

	cnt, err := New(dnfTestingImage)
	require.NoError(t, err)
	err = cnt.InitDNF()
	require.NoError(t, err)

	content, err := cnt.ReadFile("/etc/yum.repos.d/redhat.repo")
	require.NoError(t, err)
	assert.Contains(t, string(content), "rhel-9-for-x86_64-baseos-rpms")
}

// XXX: should tihs be in a different file, it's more an integration test
func TestDNFJsonWorkWithSubscribedContent(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test; not running as root")
	}
	if runtime.GOARCH != "amd64" {
		t.Skip("skipping test; only runs on x86_64")
	}
	if _, err := os.Stat("/usr/libexec/osbuild-depsolve-dnf"); err != nil {
		t.Skip("cannot find /usr/libexec/osbuild-depsolve-dnf")
	}
	cacheRoot := t.TempDir()

	restore := subscribeMachine(t)
	defer restore()

	cnt, err := New(dnfTestingImage)
	require.NoError(t, err)
	err = cnt.InitDNF()
	require.NoError(t, err)
	depsolverCmd, err := cnt.InitDepSolveDNF()
	require.NoError(t, err)

	sourceInfo, err := source.LoadInfo(cnt.Root())
	require.NoError(t, err)
	solver := dnfjson.NewSolver(
		sourceInfo.OSRelease.PlatformID,
		sourceInfo.OSRelease.VersionID,
		"x86_64",
		fmt.Sprintf("%s-%s", sourceInfo.OSRelease.ID, sourceInfo.OSRelease.VersionID),
		cacheRoot)
	solver.SetDNFJSONPath(depsolverCmd[0], depsolverCmd[1:]...)
	solver.SetRootDir("/")
	res, err := solver.Depsolve([]rpmmd.PackageSet{
		{
			Include: []string{"coreutils"},
		},
	}, 0)
	require.NoError(t, err)
	assert.True(t, len(res.Packages) > 0)
}
