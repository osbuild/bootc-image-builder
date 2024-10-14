package buildconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/blueprint"
)

// legacyBuildConfig is the json based configuration that was used in
// bootc-image-builder before PR#395. It was essentially a blueprint
// with just the extra layer of "blueprint". Supporting it still makes
// the transition of existing users/docs easier.
type legacyBuildConfig struct {
	Blueprint *json.RawMessage `json:"blueprint"`
}

type BuildConfig blueprint.Blueprint

// configRootDir is only overriden in tests
var configRootDir = "/"

func decodeJsonBuildConfig(r io.Reader, what string) (*BuildConfig, error) {
	content, err := io.ReadAll(r)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("cannot read %q: %w", what, err)
	}

	// support for legacy json before 2024/05
	var legacyBC legacyBuildConfig
	if err := json.Unmarshal(content, &legacyBC); err == nil {
		if legacyBC.Blueprint != nil {
			logrus.Warningf("Using legacy config")
			content = *legacyBC.Blueprint
		}
	}

	dec := json.NewDecoder(bytes.NewBuffer(content))
	dec.DisallowUnknownFields()

	var conf BuildConfig
	if err := dec.Decode(&conf); err != nil {
		return nil, fmt.Errorf("cannot decode %q: %w", what, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("multiple configuration objects or extra data found in %q", what)
	}
	return &conf, nil
}

func decodeTomlBuildConfig(r io.Reader, what string) (*BuildConfig, error) {
	dec := toml.NewDecoder(r)

	var conf BuildConfig
	_, err := dec.Decode(&conf)
	if err != nil {
		return nil, fmt.Errorf("cannot decode %q: %w", what, err)
	}

	return &conf, nil
}

var osStdin = os.Stdin

func loadConfig(path string) (*BuildConfig, error) {
	var fp *os.File
	var err error

	if path == "-" {
		fp = osStdin
	} else {
		fp, err = os.Open(path)
		if err != nil {
			return nil, err
		}
		defer fp.Close()
	}

	switch {
	case path == "-", filepath.Ext(path) == ".json":
		return decodeJsonBuildConfig(fp, path)
	case filepath.Ext(path) == ".toml":
		return decodeTomlBuildConfig(fp, path)
	default:
		return nil, fmt.Errorf("unsupported file extension for %q", path)
	}
}

func ReadWithFallback(userConfig string) (*BuildConfig, error) {
	// user asked for an explicit config
	if userConfig != "" {
		return loadConfig(userConfig)
	}

	// check default configs
	var foundConfig string
	for _, dflConfigFile := range []string{"config.toml", "config.json"} {
		cnfPath := filepath.Join(configRootDir, dflConfigFile)
		if _, err := os.Stat(cnfPath); err == nil {
			if foundConfig != "" {
				return nil, fmt.Errorf("found %q and also %q, only a single one is supported", dflConfigFile, filepath.Base(foundConfig))
			}
			foundConfig = cnfPath
		}
	}
	if foundConfig == "" {
		return &BuildConfig{}, nil
	}

	return loadConfig(foundConfig)
}
