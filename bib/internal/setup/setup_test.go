package setup_test

import (
	"bytes"
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

func makeFakeCanary(t *testing.T, content string) {
	tmpdir := t.TempDir()
	t.Setenv("PATH", os.Getenv("PATH")+":"+tmpdir)
	err := os.WriteFile(filepath.Join(tmpdir, "bib-canary-fakearch"), []byte(content), 0755)
	assert.NoError(t, err)
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
