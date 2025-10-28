package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/distro"

	main "github.com/osbuild/bootc-image-builder/bib/cmd/bootc-image-builder"
)

func TestNewDistroYAMLFromError(t *testing.T) {
	si := &osinfo.Info{
		OSRelease: osinfo.OSRelease{
			ID:        "weirdos",
			VersionID: "2.71",
			IDLike:    []string{"waffleos", "barky"},
		},
	}
	_, _, err := main.NewDistroYAMLFrom(si)
	assert.EqualError(t, err, "cannot load distro definitions for weirdos-2.71 or any of [waffleos barky]")
}

func TestNewDistroYAMLFromDirect(t *testing.T) {
	si := &osinfo.Info{
		OSRelease: osinfo.OSRelease{
			ID:        "centos",
			VersionID: "10",
		},
	}
	distroYAML, id, err := main.NewDistroYAMLFrom(si)
	assert.NoError(t, err)
	assert.Equal(t, &distro.ID{Name: "centos", MajorVersion: 10, MinorVersion: -1}, id)
	assert.Equal(t, "centos-10", distroYAML.Name)
}

func TestNewDistroYAMLFromFallback(t *testing.T) {
	si := &osinfo.Info{
		OSRelease: osinfo.OSRelease{
			ID:        "blmblinux",
			VersionID: "9.6",
			IDLike:    []string{"non-existing", "rhel", "centos", "fedora"},
		},
	}
	distroYAML, id, err := main.NewDistroYAMLFrom(si)
	assert.NoError(t, err)
	assert.Equal(t, &distro.ID{Name: "rhel", MajorVersion: 9, MinorVersion: 6}, id)
	assert.Equal(t, "rhel-9.6", distroYAML.Name)
}
