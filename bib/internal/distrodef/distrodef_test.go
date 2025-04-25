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
	def, err := LoadImageDef([]string{testDefLocation}, "fedoratest", "41", "anaconda-iso", "")
	require.NoError(t, err)
	assert.NotEmpty(t, def.Packages)
}

func TestLoadFuzzy(t *testing.T) {
	def, err := LoadImageDef([]string{testDefLocation}, "fedoratest", "99", "anaconda-iso", "")
	require.NoError(t, err)
	assert.NotEmpty(t, def.Packages)
}

func TestLoadUnhappy(t *testing.T) {
	_, err := LoadImageDef([]string{testDefLocation}, "lizard", "42", "anaconda-iso", "")
	assert.ErrorContains(t, err, "could not find def file for distro lizard-42")
	_, err = LoadImageDef([]string{testDefLocation}, "fedoratest", "0", "anaconda-iso", "")
	assert.ErrorContains(t, err, "could not find def file for distro fedoratest-0")

	_, err = LoadImageDef([]string{testDefLocation}, "fedoratest", "41", "anaconda-disk", "")
	assert.ErrorContains(t, err, "could not find image type \"anaconda-disk\" definition in fedoratest-41 (path: test_defs/fedoratest-41.yaml), available types: anaconda-iso")

	_, err = LoadImageDef([]string{testDefLocation}, "fedoratest", "xxx", "anaconda-disk", "")
	assert.ErrorContains(t, err, `cannot parse wanted version string: `)
}

// Removed the first definition of makeFakeDistrodefRoot as it was duplicated later with a different signature
/*
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
*/

func TestFindDistroDefMultiDirs(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-39.yaml": fakeDefFileContent,
		"b/fedora-41.yaml": fakeDefFileContent,
		"c/fedora-41.yaml": fakeDefFileContent,
	})
	assert.Equal(t, 3, len(defDirs))

	def, err := findDistroDef(defDirs, "fedora", "41", "")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, filepath.Join("b", "fedora-41.yaml")), "Actual path: %s", def)
}

func TestFindDistroDefMultiDirsIgnoreENOENT(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-41.yaml": fakeDefFileContent,
	})
	defDirs = append([]string{"/no/such/path/or/dir"}, defDirs...) // Prepend non-existent path

	def, err := findDistroDef(defDirs, "fedora", "41", "")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, filepath.Join("a", "fedora-41.yaml")))
}

func TestFindDistroDefMultiFuzzy(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-39.yaml":   fakeDefFileContent,
		"b/fedora-41.yaml":   fakeDefFileContent,
		"b/b/fedora-42.yaml": fakeDefFileContent,
		"c/fedora-41.yaml":   fakeDefFileContent,
	})
	// no fedora-99, pick the closest *valid* version <= 99 (which is 42)
	def, err := findDistroDef(defDirs, "fedora", "99", "")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, filepath.Join("b", "b", "fedora-42.yaml")), "Actual path: %s", def)
}

func TestFindDistroDefMultiFuzzyMinorReleases(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/centos-8.9.yaml":    fakeDefFileContent,
		"b/centos-7.yaml":      fakeDefFileContent,
		"c/centos-9.1.yaml":    fakeDefFileContent,
		"d/centos-9.1.1.yaml":  fakeDefFileContent,
		"b/b/centos-9.10.yaml": fakeDefFileContent,
	})
	// Want 9.11, highest available <= 9.11 is 9.10

	def, err := findDistroDef(defDirs, "centos", "9.11", "")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, filepath.Join("b", "b", "centos-9.10.yaml")), "Actual path: %s", def)
}

func TestFindDistroDefMultiFuzzyMinorReleasesIsZero(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/centos-9.yaml":  fakeDefFileContent,
		"a/centos-10.yaml": fakeDefFileContent,
	})
	// Want 10.0, highest available <= 10.0 is 10
	def, err := findDistroDef(defDirs, "centos", "10.0", "")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, filepath.Join("a", "centos-10.yaml")), "Actual path: %s", def)
}

func TestFindDistroDefMultiFuzzyError(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-40.yaml": fakeDefFileContent,
	})
	// the best version we have (40) is newer than what is requested (30), this is an error
	_, err := findDistroDef(defDirs, "fedora", "30", "")
	assert.ErrorContains(t, err, "could not find def file for distro fedora-30")
}

func TestFindDistroDefBadNumberIgnoresBadFiles(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-NaN.yaml": fakeDefFileContent, // Invalid version
		"b/fedora-40.yaml":  fakeDefFileContent, // Valid version
	})
	// Should ignore NaN and find 40 when asking for 41 (fuzzy)
	def, err := findDistroDef(defDirs, "fedora", "41", "")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, filepath.Join("b", "fedora-40.yaml")), "Actual path: %s", def)

	// Should fail if only NaN is available and we ask for 40
	defDirsOnlyBad := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-NaN.yaml": fakeDefFileContent,
	})
	_, err = findDistroDef(defDirsOnlyBad, "fedora", "40", "")
	assert.ErrorContains(t, err, "could not find def file for distro fedora-40")
}

func TestFindDistroDefCornerCases(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-.yaml":  fakeDefFileContent, // Invalid version part
		"b/fedora-1.yaml": fakeDefFileContent, // Valid
		"c/fedora.yaml":   fakeDefFileContent, // No version part
	})
	// Want 2, highest available <= 2 is 1
	def, err := findDistroDef(defDirs, "fedora", "2", "")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(def, filepath.Join("b", "fedora-1.yaml")), "Actual path: %s", def)
}

const fakeDefFileContent = "anaconda-iso:\n packages:  \n    - foo\n"
const fakeDefFileContentOther = "anaconda-iso:\n packages:  \n    - bar\n    - baz\n" // Different content for override tests

func makeFakeDistrodefRoot(t *testing.T, defFiles map[string]string) (searchPaths []string) {
	tmp := t.TempDir()

	for defFile, content := range defFiles {
		p := filepath.Join(tmp, defFile)
		dir := filepath.Dir(p)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(p, []byte(content), 0644)
		require.NoError(t, err)

		if !slices.Contains(searchPaths, dir) {
			searchPaths = append(searchPaths, dir)
		}
	}
	slices.Sort(searchPaths)
	return searchPaths
}

func TestLoadImageDefOverrideSuccessMinimal(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-40.yaml": fakeDefFileContent,
		"b/fedora-41.yaml": fakeDefFileContentOther, // Different content, would be fuzzy match otherwise
	})

	// Ask for version 99 (would fuzzy match 41), but override to 40
	// Pass dummy values for distro/ver as they are ignored by findDistroDef when override is used
	def, err := LoadImageDef(defDirs, "ignored", "ignored", "anaconda-iso", "fedora-40.yaml")
	require.NoError(t, err)
	assert.Equal(t, []string{"foo"}, def.Packages, "Should load content from fedora-40.yaml")
}

func TestLoadImageDefOverrideNotFoundMinimal(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-40.yaml": fakeDefFileContent,
	})

	_, err := LoadImageDef(defDirs, "ignored", "ignored", "anaconda-iso", "nonexistent-def.yaml")
	assert.ErrorContains(t, err, `override definition file "nonexistent-def.yaml" not found in search paths`)
}

func TestLoadImageDefOverrideInvalidNameMinimal(t *testing.T) {
	defDirs := makeFakeDistrodefRoot(t, map[string]string{
		"a/fedora-40.yaml": fakeDefFileContent,
	})

	_, err := LoadImageDef(defDirs, "ignored", "ignored", "anaconda-iso", "a/fedora-40.yaml")
	assert.ErrorContains(t, err, `override definition "a/fedora-40.yaml" must be a base filename`)

	_, err = LoadImageDef(defDirs, "ignored", "ignored", "anaconda-iso", "fedora-40") // Missing .yaml
	assert.ErrorContains(t, err, `override definition "fedora-40" must end with .yaml`)
}
