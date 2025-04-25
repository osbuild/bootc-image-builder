package distrodef

import (
	"errors"
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

// findDistroDef searches for the appropriate distro definition file.
// If overrideDefFilename is provided, it searches for that exact filename within defDirs.
// Otherwise, it performs exact version matching and then fuzzy version matching based on distro and wantedVerStr.
func findDistroDef(defDirs []string, distro, wantedVerStr string, overrideDefFilename string) (string, error) {
	if overrideDefFilename != "" {
		// Basic validation: ensure it's just a filename, not a path, and ends with .yaml
		if strings.ContainsAny(overrideDefFilename, string([]rune{filepath.Separator, filepath.ListSeparator})) {
			return "", fmt.Errorf("override definition %q must be a base filename, not a path", overrideDefFilename)
		}
		if !strings.HasSuffix(overrideDefFilename, ".yaml") {
			return "", fmt.Errorf("override definition %q must end with .yaml", overrideDefFilename)
		}

		for _, defDir := range defDirs {
			potentialPath := filepath.Join(defDir, overrideDefFilename)
			_, err := os.Stat(potentialPath)
			if err == nil {
				fmt.Printf("Using overridden definition file: %s\n", potentialPath)
				return potentialPath, nil
			} else if !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "Warning: error checking override path %s: %v\n", potentialPath, err)
			}
		}
		return "", fmt.Errorf("override definition file %q not found in search paths %v", overrideDefFilename, defDirs)
	}

	var bestFuzzyMatch string
	bestFuzzyVer := &version.Version{}
	wantedVer, err := version.NewVersion(wantedVerStr)
	if err != nil {
		return "", fmt.Errorf("cannot parse wanted version string: %q %w", wantedVerStr, err)
	}

	for _, defDir := range defDirs {
		// exact match
		exactMatchPattern := filepath.Join(defDir, fmt.Sprintf("%s-%s.yaml", distro, wantedVerStr))
		matches, err := filepath.Glob(exactMatchPattern)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("error searching for exact match %q: %w", exactMatchPattern, err)
		}
		if len(matches) == 1 {
			return matches[0], nil // Exact match found
		}
		if len(matches) > 1 {
			return "", fmt.Errorf("found multiple exact matches for %s-%s in %s: %v", distro, wantedVerStr, defDir, matches)
		}

		// No exact match in this dir, check for fuzzy matches
		fuzzyMatchPattern := filepath.Join(defDir, fmt.Sprintf("%s-[0-9.]*.yaml", distro))
		matches, err = filepath.Glob(fuzzyMatchPattern)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("error searching for fuzzy matches %q: %w", fuzzyMatchPattern, err)
		}

		for _, m := range matches {
			baseNoExt := strings.TrimSuffix(filepath.Base(m), ".yaml")
			parts := strings.SplitN(baseNoExt, "-", 2)
			if len(parts) < 2 || parts[1] == "" {
				continue
			}
			haveVerStr := parts[1]

			haveVer, err := version.NewVersion(haveVerStr)
			if err != nil {
				return "", fmt.Errorf("cannot parse distro version from %q: %w", m, err)
			}
			if wantedVer.Compare(haveVer) >= 0 && haveVer.Compare(bestFuzzyVer) > 0 {
				bestFuzzyVer = haveVer
				bestFuzzyMatch = m
			}
		}
	}
	if bestFuzzyMatch == "" {
		return "", fmt.Errorf("could not find def file for distro %s-%s", distro, wantedVerStr)
	}

	fmt.Printf("Using best fuzzy match definition file: %s (wanted %s, found %s)\n", bestFuzzyMatch, wantedVerStr, bestFuzzyVer.Original())
	return bestFuzzyMatch, nil
}

func loadFile(defDirs []string, distro, ver string, overrideDefFilename string) ([]byte, string, error) {
	defPath, err := findDistroDef(defDirs, distro, ver, overrideDefFilename)
	if err != nil {
		return nil, "", err
	}

	content, err := os.ReadFile(defPath)
	if err != nil {
		return nil, defPath, fmt.Errorf("could not read def file %s for distro %s-%s: %v", defPath, distro, ver, err)
	}
	return content, defPath, nil
}

// Loads a definition file for a given distro and image type.
// If overrideDefFilename is provided (e.g., "fedora-40.yaml"), it attempts to load that specific file
// from the defDirs, ignoring the distro and ver parameters for file searching but using them for error messages.
func LoadImageDef(defDirs []string, distro, ver, it string, overrideDefFilename string) (*ImageDef, error) {
	data, defPath, err := loadFile(defDirs, distro, ver, overrideDefFilename) // Pass override down
	if err != nil {
		return nil, err
	}

	var defs map[string]ImageDef
	if err := yaml.Unmarshal(data, &defs); err != nil {
		return nil, fmt.Errorf("could not unmarshal definition file %s: %w", defPath, err)
	}

	d, ok := defs[it]
	if !ok {
		distroIdentifier := fmt.Sprintf("%s-%s", distro, ver)
		if overrideDefFilename != "" {
			distroIdentifier = fmt.Sprintf("overridden file %s", overrideDefFilename)
		}
		return nil, fmt.Errorf("could not find image type %q definition in %s (path: %s), available types: %s",
			it, distroIdentifier, defPath, strings.Join(maps.Keys(defs), ", "))
	}

	return &d, nil
}
