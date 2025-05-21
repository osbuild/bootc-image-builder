package source

import (
	"fmt"
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

func createImageCustomization(root, custType string) error {
	bibDir := path.Join(root, "usr/lib/bootc-image-builder/")
	err := os.MkdirAll(bibDir, 0755)
	if err != nil {
		return err
	}

	var buf string
	var filename string
	switch custType {
	case "json":
		buf = `{
			"customizations": {
				"disk": {
					"partitions": [
						{
							"label": "var",
							"mountpoint": "/var",
							"fs_type": "ext4",
							"minsize": "3 GiB",
							"part_type": "01234567-89ab-cdef-0123-456789abcdef"
							}
					]
				}
			}
		}`
		filename = "config.json"
	case "toml":
		buf = `[[customizations.disk.partitions]]
label = "var"
mountpoint = "/var"
fs_type = "ext4"
minsize = "3 GiB"
part_type = "01234567-89ab-cdef-0123-456789abcdef"
`
		filename = "config.toml"
	case "broken":
		buf = "{"
		filename = "config.json"
	default:
		return fmt.Errorf("unsupported customization type %s", custType)
	}

	return os.WriteFile(path.Join(bibDir, filename), []byte(buf), 0644)
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
		custType   string
		errorStr   string
	}{
		{"happy", "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos", "", "json", ""},
		{"happy-no-uefi", "fedora", "40", "Fedora Linux", "", "platform:f40", "coreos", "", "json", ""},
		{"happy-no-variant_id", "fedora", "40", "Fedora Linux", "", "platform:f40", "", "", "json", ""},
		{"happy-no-id", "fedora", "43", "Fedora Linux", "fedora", "", "", "", "json", ""},
		{"happy-with-id-like", "centos", "9", "CentOS Stream", "", "platform:el9", "", "rhel fedora", "json", ""},
		{"happy-no-cust", "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos", "", "", ""},
		{"happy-toml", "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos", "", "toml", ""},
		{"sad-no-id", "", "40", "Fedora Linux", "fedora", "platform:f40", "", "", "json", "missing ID in os-release"},
		{"sad-no-id", "fedora", "", "Fedora Linux", "fedora", "platform:f40", "", "", "json", "missing VERSION_ID in os-release"},
		{"sad-no-id", "fedora", "40", "", "fedora", "platform:f40", "", "", "json", "missing NAME in os-release"},
		{"sad-broken-json", "fedora", "40", "Fedora Linux", "fedora", "platform:f40", "coreos", "", "broken", "cannot decode \"$ROOT/usr/lib/bootc-image-builder/config.json\": unexpected EOF"},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, writeOSRelease(root, c.id, c.versionID, c.name, c.platformID, c.variantID, c.idLike))
			if c.uefiVendor != "" {
				require.NoError(t, createBootupdEFI(root, c.uefiVendor))

			}
			if c.custType != "" {
				require.NoError(t, createImageCustomization(root, c.custType))

			}

			info, err := LoadInfo(root)

			if c.errorStr != "" {
				require.EqualError(t, err, strings.ReplaceAll(c.errorStr, "$ROOT", root))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.id, info.OSRelease.ID)
			assert.Equal(t, c.versionID, info.OSRelease.VersionID)
			assert.Equal(t, c.name, info.OSRelease.Name)
			assert.Equal(t, c.uefiVendor, info.UEFIVendor)
			assert.Equal(t, c.platformID, info.OSRelease.PlatformID)
			assert.Equal(t, c.variantID, info.OSRelease.VariantID)
			if c.custType != "" {
				assert.NotNil(t, info.ImageCustomization)
				assert.NotNil(t, info.ImageCustomization.Disk)
				assert.NotEmpty(t, info.ImageCustomization.Disk.Partitions)
				part := info.ImageCustomization.Disk.Partitions[0]
				assert.Equal(t, part.Label, "var")
				assert.Equal(t, part.MinSize, uint64(3*1024*1024*1024))
				assert.Equal(t, part.FSType, "ext4")
				assert.Equal(t, part.Mountpoint, "/var")
				// TODO: Validate part.PartType when it is fixed
			} else {
				assert.Nil(t, info.ImageCustomization)
			}
			if c.idLike == "" {
				assert.Equal(t, len(info.OSRelease.IDLike), 0)
			} else {
				expected := strings.Split(c.idLike, " ")
				assert.Equal(t, expected, info.OSRelease.IDLike)
			}
		})
	}
}
