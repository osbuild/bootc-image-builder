package setup_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/osbuild/bootc-image-builder/bib/internal/setup"
)

func TestValidateCanRunTargetArchTrivial(t *testing.T) {
	for _, arch := range []string{runtime.GOARCH, ""} {
		err := setup.ValidateCanRunTargetArch(arch)
		assert.NoError(t, err)
	}
}

func TestValidateCanRunTargetArchUnsupportedCanary(t *testing.T) {
	var logbuf bytes.Buffer
	logrus.SetOutput(&logbuf)

	err := setup.ValidateCanRunTargetArch("unsupported-arch")
	assert.NoError(t, err)
	assert.Contains(t, logbuf.String(), `level=warning msg="cannot check architecture support for unsupported-arch: no canary binary found"`)
}

func makeFakeBinary(t *testing.T, binary, content string) {
	tmpdir := t.TempDir()
	t.Setenv("PATH", tmpdir+":"+os.Getenv("PATH"))
	err := os.WriteFile(filepath.Join(tmpdir, binary), []byte(content), 0o755)
	assert.NoError(t, err)
}

func makeFakeCanary(t *testing.T, content string) {
	makeFakeBinary(t, "bib-canary-fakearch", content)
}

func TestValidateCanRunTargetArchHappy(t *testing.T) {
	var logbuf bytes.Buffer
	logrus.SetOutput(&logbuf)

	makeFakeCanary(t, "#!/bin/sh\necho ok")

	err := setup.ValidateCanRunTargetArch("fakearch")
	assert.NoError(t, err)
	assert.Equal(t, "", logbuf.String())
}

func TestValidateCanRunTargetArchExecFormatError(t *testing.T) {
	makeFakeCanary(t, "")

	err := setup.ValidateCanRunTargetArch("fakearch")
	assert.ErrorContains(t, err, `cannot run canary binary for "fakearch", do you have 'qemu-user-static' installed?`)
	assert.ErrorContains(t, err, `: exec format error`)
}

func TestValidateCanRunTargetArchUnexpectedOutput(t *testing.T) {
	makeFakeCanary(t, "#!/bin/sh\necho xxx")

	err := setup.ValidateCanRunTargetArch("fakearch")
	assert.ErrorContains(t, err, `internal error: unexpected output`)
}

var (
	fakePodmanOutputCentosBootc = `map[containers.bootc:1 io.buildah.version:1.29.1 org.opencontainers.image.version:stream9.20240319.0 ostree.bootable:true ostree.commit:97d619eae2a5474a9c363c78e3ad6caec14acba54a0b077c7cb69d00a4f800a5 ostree.final-diffid:sha256:12787d84fa137cd5649a9005efe98ec9d05ea46245fdc50aecb7dd007f2035b1 ostree.linux:5.14.0-430.el9.x86_64 redhat.compose-id:CentOS-Stream-9-20240304.d.0 redhat.id:centos redhat.version-id:9 rpmostree.inputhash:a5c67fd4e9465e47e01922171c6ab8edf261d2d381e590b5cd7fed81ea8d4dbe]`

	fakePodmanOutputCentos = `map[io.buildah.version:1.33.7 org.label-schema.build-date:20240618 org.label-schema.license:GPLv2 org.label-schema.name:CentOS Stream 9 Base Image org.label-schema.schema-version:1.0 org.label-schema.vendor:CentOS]`

	emptyPodmanOutput = `map[]`
)

func TestValidateTags(t *testing.T) {
	for _, tc := range []struct {
		imageref    string
		fakeOutput  string
		expectedErr string
	}{
		{
			"quay.io/centos-bootc/centos-bootc:stream9",
			fakePodmanOutputCentosBootc,
			"",
		},
		{
			"quay.io/centos/centos:stream9",
			fakePodmanOutputCentos,
			"image quay.io/centos/centos:stream9 is not a bootc image",
		},
		{
			"fake/image",
			emptyPodmanOutput,
			"image fake/image is not a bootc image",
		},
	} {
		podmanArgsFile := filepath.Join(t.TempDir(), "args.txt")
		fakePodman := fmt.Sprintf(`#!/bin/sh -e
echo "$@" > '%s'
echo '%s'
`, podmanArgsFile, tc.fakeOutput)
		makeFakeBinary(t, "podman", fakePodman)
		err := setup.ValidateHasContainerTags(tc.imageref)
		if tc.expectedErr == "" {
			assert.NoError(t, err)
		} else {
			assert.EqualError(t, err, tc.expectedErr)
		}
	}
}
