package osbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
)

type OCIArchiveStageOptions struct {
	// The CPU architecture of the image
	Architecture string `json:"architecture"`

	// Resulting image filename
	Filename string `json:"filename"`

	// The execution parameters
	Config *OCIArchiveConfig `json:"config,omitempty"`
}

type OCIArchiveConfig struct {
	Cmd          []string          `json:"Cmd,omitempty"`
	Env          []string          `json:"Env,omitempty"`
	ExposedPorts []string          `json:"ExposedPorts,omitempty"`
	User         string            `json:"User,omitempty"`
	Labels       map[string]string `json:"Labels,omitempty"`
	StopSignal   string            `json:"StopSignal,omitempty"`
	Volumes      []string          `json:"Volumes,omitempty"`
	WorkingDir   string            `json:"WorkingDir,omitempty"`
}

func (OCIArchiveStageOptions) isStageOptions() {}

type OCIArchiveStageInputs struct {
	// Base layer for the container
	Base *TreeInput `json:"base"`
	// Additional layers in ascending order
	Layers []TreeInput `json:",omitempty"`
}

func (OCIArchiveStageInputs) isStageInputs() {}

// A new OCIArchiveStage to to assemble an OCI image archive
func NewOCIArchiveStage(options *OCIArchiveStageOptions, inputs *OCIArchiveStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.oci-archive",
		Options: options,
		Inputs:  inputs,
	}
}

// Custom marshaller for OCIArchiveStageInputs, needed to generate keys of the
// form "layer.N", (where N = 1, 2, ...) for the Layers property
func (inputs *OCIArchiveStageInputs) MarshalJSON() ([]byte, error) {
	if inputs == nil {
		return json.Marshal(inputs)
	}

	layers := inputs.Layers
	inputsMap := make(map[string]TreeInput, len(layers)+1)
	if inputs.Base != nil {
		inputsMap["base"] = *inputs.Base
	}

	for idx, input := range layers {
		key := fmt.Sprintf("layer.%d", idx+1)
		inputsMap[key] = input
	}

	return json.Marshal(inputsMap)
}

// Get the sorted keys that match the pattern "layer.N" (for N > 0)
func layerKeys(layers map[string]TreeInput) ([]string, error) {
	keys := make([]string, 0, len(layers))
	for key := range layers {
		re := regexp.MustCompile(`layer\.[1-9]\d*`)
		if key == "base" {
			continue
		}
		if !re.MatchString(key) {
			return nil, fmt.Errorf("invalid key: %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

// Custom unmarshaller for OCIArchiveStageInputs, needed to handle keys of the
// form "layer.N", (where N = 1, 2, ...) for the Layers property
func (inputs *OCIArchiveStageInputs) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if inputs == nil {
		inputs = new(OCIArchiveStageInputs)
	}

	inputsMap := make(map[string]TreeInput)

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&inputsMap); err != nil {
		return err
	}

	// "base" layer is required
	base, ok := inputsMap["base"]
	if !ok {
		return fmt.Errorf("missing required key \"base\"")
	}

	inputs.Base = &base
	keys, err := layerKeys(inputsMap)
	if err != nil {
		return err
	}
	inputs.Layers = make([]TreeInput, len(inputsMap)-1)
	for idx, key := range keys {
		inputs.Layers[idx] = inputsMap[key]
	}

	return nil
}
