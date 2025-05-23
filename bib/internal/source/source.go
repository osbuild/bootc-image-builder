package source

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/bootc-image-builder/bib/internal/buildconfig"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/distro"
)

const bibPathPrefix = "usr/lib/bootc-image-builder"

type OSRelease struct {
	PlatformID string
	ID         string
	VersionID  string
	Name       string
	VariantID  string
	IDLike     []string
}

type Info struct {
	OSRelease          OSRelease
	UEFIVendor         string
	SELinuxPolicy      string
	ImageCustomization *blueprint.Customizations
}

func validateOSRelease(osrelease map[string]string) error {
	// VARIANT_ID, PLATFORM_ID are optional
	for _, key := range []string{"ID", "VERSION_ID", "NAME"} {
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

func readSelinuxPolicy(root string) (string, error) {
	configPath := "etc/selinux/config"
	f, err := os.Open(path.Join(root, configPath))
	if err != nil {
		return "", fmt.Errorf("cannot read selinux config %s: %w", configPath, err)
	}
	// nolint:errcheck
	defer f.Close()

	policy := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return "", errors.New("selinux config: invalid input")
		}
		key := strings.TrimSpace(parts[0])
		if key == "SELINUXTYPE" {
			policy = strings.TrimSpace(parts[1])
		}
	}

	return policy, nil
}

func readImageCustomization(root string) (*blueprint.Customizations, error) {
	prefix := path.Join(root, bibPathPrefix)
	config, err := buildconfig.LoadConfig(path.Join(prefix, "config.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if config == nil {
		config, err = buildconfig.LoadConfig(path.Join(prefix, "config.toml"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	// no config found in either toml/json
	if config == nil {
		return nil, nil
	}

	return config.Customizations, nil
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

	customization, err := readImageCustomization(root)
	if err != nil {
		return nil, err
	}

	selinuxPolicy, err := readSelinuxPolicy(root)
	if err != nil {
		logrus.Debugf("cannot read selinux policy: %v, setting it to none", err)
	}

	var idLike []string
	if osrelease["ID_LIKE"] != "" {
		idLike = strings.Split(osrelease["ID_LIKE"], " ")
	}

	return &Info{
		OSRelease: OSRelease{
			ID:         osrelease["ID"],
			VersionID:  osrelease["VERSION_ID"],
			Name:       osrelease["NAME"],
			PlatformID: osrelease["PLATFORM_ID"],
			VariantID:  osrelease["VARIANT_ID"],
			IDLike:     idLike,
		},

		UEFIVendor:         vendor,
		SELinuxPolicy:      selinuxPolicy,
		ImageCustomization: customization,
	}, nil
}
