package source

import (
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/distro"
)

type OSRelease struct {
	PlatformID string
	ID         string
	VersionID  string
	Name       string
	VariantID  string
}

type Info struct {
	OSRelease  OSRelease
	UEFIVendor string
}

func validateOSRelease(osrelease map[string]string) error {
	// VARIANT_ID is optional
	for _, key := range []string{"ID", "VERSION_ID", "NAME", "PLATFORM_ID"} {
		if _, ok := osrelease[key]; !ok {
			return fmt.Errorf("missing %s in os-release", key)
		}
	}
	return nil
}

func uefiVendor(root string) (string, error) {
	bootupdEfiDir := path.Join(root, "usr/lib/bootupd/updates/EFI")
	l, err := os.ReadDir(bootupdEfiDir)
	if err != nil {
		return "", fmt.Errorf("cannot read bootupd EFI directory %s: %w", bootupdEfiDir, err)
	}

	// best-effort search: return the first directory that's not "BOOT"
	for _, entry := range l {
		if !entry.IsDir() {
			continue
		}

		if entry.Name() == "BOOT" {
			continue
		}

		return entry.Name(), nil
	}

	return "", fmt.Errorf("cannot find UEFI vendor in %s", bootupdEfiDir)
}

func LoadInfo(root string) (*Info, error) {
	osrelease, err := distro.ReadOSReleaseFromTree(root)
	if err != nil {
		return nil, err
	}
	if err := validateOSRelease(osrelease); err != nil {
		return nil, err
	}

	vendor, err := uefiVendor(root)
	if err != nil {
		logrus.Debugf("cannot read UEFI vendor: %v, setting it to none", err)
	}

	return &Info{
		OSRelease: OSRelease{
			ID:         osrelease["ID"],
			VersionID:  osrelease["VERSION_ID"],
			Name:       osrelease["NAME"],
			PlatformID: osrelease["PLATFORM_ID"],
			VariantID:  osrelease["VARIANT_ID"],
		},

		UEFIVendor: vendor,
	}, nil
}
