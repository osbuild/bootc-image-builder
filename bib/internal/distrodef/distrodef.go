package distrodef

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"

	"github.com/hashicorp/go-version"
)

// ImageDef is a structure containing extra information needed to build an image that cannot be extracted
// from the container image itself. Currently, this is only the list of packages needed for the installer
// ISO.
type ImageDef struct {
	Packages []string `yaml:"packages"`
}

func findDistroDef(defDirs []string, distro, wantedVerStr string) (string, error) {
	var bestFuzzyMatch string

	bestFuzzyVer := &version.Version{}
	wantedVer, err := version.NewVersion(wantedVerStr)
	if err != nil {
		return "", fmt.Errorf("cannot parse wanted version string: %w", err)
	}

	for _, defDir := range defDirs {
		// exact match
		matches, err := filepath.Glob(filepath.Join(defDir, fmt.Sprintf("%s-%s.yaml", distro, wantedVerStr)))
		if err != nil {
			return "", err
		}
		if len(matches) == 1 {
			return matches[0], nil
		}

		// fuzzy match
		matches, err = filepath.Glob(filepath.Join(defDir, fmt.Sprintf("%s-[0-9]*.yaml", distro)))
		if err != nil {
			return "", err
		}
		for _, m := range matches {
			baseNoExt := strings.TrimSuffix(filepath.Base(m), ".yaml")
			haveVerStr := strings.SplitN(baseNoExt, "-", 2)[1]
			// this should never error (because of the glob above) but be defensive
			haveVer, err := version.NewVersion(haveVerStr)
			if err != nil {
				return "", fmt.Errorf("cannot parse distro version from %q: %w", m, err)
			}
			if wantedVer.Compare(haveVer) > 0 && haveVer.Compare(bestFuzzyVer) > 0 {
				bestFuzzyVer = haveVer
				bestFuzzyMatch = m
			}
		}
	}
	if bestFuzzyMatch == "" {
		return "", fmt.Errorf("could not find def file for distro %s-%s", distro, wantedVerStr)
	}

	return bestFuzzyMatch, nil
}

func loadFile(defDirs []string, distro, ver string) ([]byte, error) {
	defPath, err := findDistroDef(defDirs, distro, ver)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("could not read def file %s for distro %s-%s: %v", defPath, distro, ver, err)
	}
	return content, nil
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
