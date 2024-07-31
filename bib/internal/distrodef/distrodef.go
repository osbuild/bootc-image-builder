package distrodef

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"
)

// ImageDef is a structure containing extra information needed to build an image that cannot be extracted
// from the container image itself. Currently, this is only the list of packages needed for the installer
// ISO.
type ImageDef struct {
	Packages []string `yaml:"packages"`
}

func loadFile(defDirs []string, distro, ver string) ([]byte, error) {
	for _, loc := range defDirs {
		p := filepath.Join(loc, fmt.Sprintf("%s-%s.yaml", distro, ver))
		content, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not read def file %s for distro %s-%s: %v", p, distro, ver, err)
		}

		return content, nil
	}

	return nil, fmt.Errorf("could not find def file for distro %s-%s", distro, ver)
}

// Loads a definition file for a given distro and image type
func LoadImageDef(defDirs []string, distro, ver, it string) (*ImageDef, error) {
	data, err := loadFile(defDirs, distro, ver)
	if err != nil {
		return nil, err
	}

	var defs map[string]ImageDef
	if err := yaml.Unmarshal(data, &defs); err != nil {
		return nil, fmt.Errorf("could not unmarshal def file for distro %s: %v", distro, err)
	}

	d, ok := defs[it]
	if !ok {
		return nil, fmt.Errorf("could not find def for distro %s and image type %s, available types: %s", distro, it, strings.Join(maps.Keys(defs), ", "))
	}

	return &d, nil
}
