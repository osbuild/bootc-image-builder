package buildconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml"

	"github.com/osbuild/images/pkg/blueprint"
)

type BuildConfig struct {
	Blueprint *blueprint.Blueprint `json:"blueprint,omitempty" toml:"blueprint"`
}

// configRootDir is only overriden in tests
var configRootDir = "/"

func decodeJsonBuildConfig(r io.Reader, what string) (*BuildConfig, error) {
	dec := json.NewDecoder(r)
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
	if err := dec.Decode(&conf); err != nil {
		return nil, fmt.Errorf("cannot decode %q: %w", what, err)
	}
	return &conf, nil
}

func loadConfig(path string) (*BuildConfig, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	switch filepath.Ext(path) {
	case ".json":
		return decodeJsonBuildConfig(fp, path)
	case ".toml":
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
