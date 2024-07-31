package distrodef

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDefLocation = "test_defs"

func TestLoad(t *testing.T) {
	def, err := LoadImageDef([]string{testDefLocation}, "fedoratest", "41", "anaconda-iso")
	require.NoError(t, err)

	assert.NotEmpty(t, def.Packages)
}

func TestLoadUnhappy(t *testing.T) {
	_, err := LoadImageDef([]string{testDefLocation}, "lizard", "42", "anaconda-iso")
	assert.ErrorContains(t, err, "could not find def file for distro lizard-42")
	_, err = LoadImageDef([]string{testDefLocation}, "fedoratest", "0", "anaconda-iso")
	assert.ErrorContains(t, err, "could not find def file for distro fedoratest-0")

	_, err = LoadImageDef([]string{testDefLocation}, "fedoratest", "41", "anaconda-disk")
	assert.ErrorContains(t, err, "could not find def for distro fedoratest and image type anaconda-disk")
}
