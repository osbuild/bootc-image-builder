package util_test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/bootc-image-builder/bib/internal/util"
)

func TestOutputErrPassthrough(t *testing.T) {
	err := fmt.Errorf("boom")
	assert.Equal(t, util.OutputErr(err), err)
}

func TestOutputErrExecError(t *testing.T) {
	_, err := exec.Command("bash", "-c", ">&2 echo some-stderr; exit 1").Output()
	assert.Equal(t, "exit status 1, stderr:\nsome-stderr\n", util.OutputErr(err).Error())
}
