package distrodef

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDefLocation = "test_defs"

func TestLoadSimple(t *testing.T) {
	def, err := LoadImageDef([]string{testDefLocation}, "fedoratest", "41", "anaconda-iso")
	require.NoError(t, err)
	assert.NotEmpty(t, def.Packages)
}

func TestLoadFuzzy(t *testing.T) {
	def, err := LoadImageDef([]string{testDefLocation}, "fedoratest", "99", "anaconda-iso")
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

	_, err = LoadImageDef([]string{testDefLocation}, "fedoratest", "xxx", "anaconda-disk")
	assert.ErrorContains(t, err, `cannot parse wanted version string: `)
}

const fakeDefFileContent = "anaconda-iso:\n packages:  \n    - foo\n"

func makeFakeDistrodefRoot(t *testing.T, defFiles []string) (searchPaths []string) {
	tmp := t.TempDir()

	for _, defFile := range defFiles {
		p := filepath.Join(tmp, defFile)
		err := os.MkdirAll(filepath.Dir(p), 0755)
		require.NoError(t, err)
		err = os.WriteFile(p, []byte(fakeDefFileContent), 0644)
		require.NoError(t, err)

		if !slices.Contains(searchPaths, filepath.Dir(p)) {
			searchPaths = append(searchPaths, filepath.Dir(p))
		}
	}

	return searchPaths
}

func TestFindDistroDefMultiDirs(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, []string{
		"a/fedora-39.yaml",
		"b/fedora-41.yaml",
		"c/fedora-41.yaml",
	})
	assert.Equal(t, 3, len(defDirs))

	def, err := findDistroDef(defDirs, "fedora", "41")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, "b/fedora-41.yaml"))
}

func TestFindDistroDefMultiDirsIgnoreENOENT(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, []string{
		"a/fedora-41.yaml",
	})
	defDirs = append([]string{"/no/such/path"}, defDirs...)

	def, err := findDistroDef(defDirs, "fedora", "41")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, "a/fedora-41.yaml"))
}

func TestFindDistroDefMultiFuzzy(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, []string{
		"a/fedora-39.yaml",
		"b/fedora-41.yaml",
		"b/b/fedora-42.yaml",
		"c/fedora-41.yaml",
	})
	// no fedora-99, pick the closest
	def, err := findDistroDef(defDirs, "fedora", "99")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, "b/b/fedora-42.yaml"))
}

func TestFindDistroDefMultiFuzzyMinorReleases(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, []string{
		"a/centos-8.9.yaml",
		"b/centos-7.yaml",
		"c/centos-9.1.yaml",
		"d/centos-9.1.1.yaml",
		"b/b/centos-9.10.yaml",
	})
	def, err := findDistroDef(defDirs, "centos", "9.11")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, "b/b/centos-9.10.yaml"), def)
}

func TestFindDistroDefMultiFuzzyMinorReleasesIsZero(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, []string{
		"a/centos-9.yaml",
		"a/centos-10.yaml",
	})
	def, err := findDistroDef(defDirs, "centos", "10.0")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, "a/centos-10.yaml"), def)
}

func TestFindDistroDefMultiFuzzyError(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, []string{
		"a/fedora-40.yaml",
	})
	// the best version we have is newer than what is requested, this
	// is an error
	_, err := findDistroDef(defDirs, "fedora", "30")
	assert.ErrorContains(t, err, "could not find def file for distro fedora-30")
}

func TestFindDistroDefBadNumberIgnoresBadFiles(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, []string{
		"a/fedora-NaN.yaml",
	})
	_, err := findDistroDef(defDirs, "fedora", "40")
	assert.ErrorContains(t, err, "could not find def file for distro fedora-40")
}

func TestFindDistroDefCornerCases(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, []string{
		"a/fedora-.yaml",
		"b/fedora-1.yaml",
		"c/fedora.yaml",
	})
	def, err := findDistroDef(defDirs, "fedora", "2")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, "b/fedora-1.yaml"))
}
