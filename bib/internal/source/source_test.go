package source

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeOSRelease(root, id, versionID, name, platformID, variantID, idLike string) error {
	err := os.MkdirAll(path.Join(root, "etc"), 0755)
	if err != nil {
		return err
	}

	var buf string
	if id != "" {
		buf += "ID=" + id + "\n"
	}
	if versionID != "" {
		buf += "VERSION_ID=" + versionID + "\n"
	}
	if name != "" {
		buf += "NAME=" + name + "\n"
	}
	if platformID != "" {
		buf += "PLATFORM_ID=" + platformID + "\n"
	}
	if variantID != "" {
		buf += "VARIANT_ID=" + variantID + "\n"
	}
	if idLike != "" {
		buf += "ID_LIKE=" + idLike + "\n"
	}

	return os.WriteFile(path.Join(root, "etc/os-release"), []byte(buf), 0644)
}

func createBootupdEFI(root, uefiVendor string) error {
	err := os.MkdirAll(path.Join(root, "usr/lib/bootupd/updates/EFI/BOOT"), 0755)
	if err != nil {
		return err
	}
	return os.Mkdir(path.Join(root, "usr/lib/bootupd/updates/EFI", uefiVendor), 0755)
}

func TestLoadInfo(t *testing.T) {
	cases := []struct {
		desc       string
		id         string
		versionID  string
		name       string
		uefiVendor string
		platformID string
		variantID  string
		idLike     string
		errorStr   string
	}{
		{"happy", "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos", "", ""},
		{"happy-no-uefi", "fedora", "40", "Fedora Linux", "", "platform:f40", "coreos", "", ""},
		{"happy-no-variant_id", "fedora", "40", "Fedora Linux", "", "platform:f40", "", "", ""},
		{"happy-no-id", "fedora", "43", "Fedora Linux", "fedora", "", "", "", ""},
		{"happy-with-id-like", "centos", "9", "CentOS Stream", "", "platform:el9", "", "rhel fedora", ""},
		{"sad-no-id", "", "40", "Fedora Linux", "fedora", "platform:f40", "", "", "missing ID in os-release"},
		{"sad-no-id", "fedora", "", "Fedora Linux", "fedora", "platform:f40", "", "", "missing VERSION_ID in os-release"},
		{"sad-no-id", "fedora", "40", "", "fedora", "platform:f40", "", "", "missing NAME in os-release"},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, writeOSRelease(root, c.id, c.versionID, c.name, c.platformID, c.variantID, c.idLike))
			if c.uefiVendor != "" {
				require.NoError(t, createBootupdEFI(root, c.uefiVendor))

			}

			info, err := LoadInfo(root)

			if c.errorStr != "" {
				require.EqualError(t, err, c.errorStr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.id, info.OSRelease.ID)
			assert.Equal(t, c.versionID, info.OSRelease.VersionID)
			assert.Equal(t, c.name, info.OSRelease.Name)
			assert.Equal(t, c.uefiVendor, info.UEFIVendor)
			assert.Equal(t, c.platformID, info.OSRelease.PlatformID)
			assert.Equal(t, c.variantID, info.OSRelease.VariantID)
			if c.idLike == "" {
				assert.Equal(t, len(info.OSRelease.IDLike), 0)
			} else {
				expected := strings.Split(c.idLike, " ")
				assert.Equal(t, expected, info.OSRelease.IDLike)
			}
		})
	}
}
