package cntdnf_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/rpmmd"

	"github.com/osbuild/bootc-image-builder/bib/internal/cntdnf"
	"github.com/osbuild/bootc-image-builder/bib/internal/container"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
)

const (
	dnfTestingImageRHEL   = "registry.access.redhat.com/ubi9:latest"
	dnfTestingImageCentos = "quay.io/centos/centos:stream9"
)

func TestDNFJsonWorks(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test; not running as root")
	}
	if _, err := os.Stat("/usr/libexec/osbuild-depsolve-dnf"); err != nil {
		t.Skip("cannot find /usr/libexec/osbuild-depsolve-dnf")
	}
	cacheRoot := t.TempDir()

	cnt, err := container.New(dnfTestingImageCentos)
	require.NoError(t, err)
	err = cnt.InitDNF()
	require.NoError(t, err)

	sourceInfo, err := source.LoadInfo(cnt.Root())
	require.NoError(t, err)
	solver, err := cntdnf.NewContainerSolver(cacheRoot, cnt, arch.Current(), sourceInfo)
	require.NoError(t, err)
	res, err := solver.Depsolve([]rpmmd.PackageSet{
		{
			Include: []string{"coreutils"},
		},
	}, 0)
	require.NoError(t, err)
	assert.True(t, len(res.Packages) > 0)
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
		err := exec.Command("subscription-manager", "unregister").Run()
		require.NoError(t, err)
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

	cnt, err := container.New(dnfTestingImageRHEL)
	require.NoError(t, err)
	err = cnt.InitDNF()
	require.NoError(t, err)

	content, err := cnt.ReadFile("/etc/yum.repos.d/redhat.repo")
	require.NoError(t, err)
	assert.Contains(t, string(content), "rhel-9-for-x86_64-baseos-rpms")
}

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

	cnt, err := container.New(dnfTestingImageRHEL)
	require.NoError(t, err)
	err = cnt.InitDNF()
	require.NoError(t, err)

	sourceInfo, err := source.LoadInfo(cnt.Root())
	require.NoError(t, err)
	solver, err := cntdnf.NewContainerSolver(cacheRoot, cnt, arch.ARCH_X86_64, sourceInfo)
	require.NoError(t, err)
	res, err := solver.Depsolve([]rpmmd.PackageSet{
		{
			Include: []string{"coreutils"},
		},
	}, 0)
	require.NoError(t, err)
	assert.True(t, len(res.Packages) > 0)
}
