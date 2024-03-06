package ignition

import (
	"encoding/base64"
	"errors"

	"github.com/osbuild/images/pkg/blueprint"
)

type FirstBootOptions struct {
	ProvisioningURL string
}

func FirstbootOptionsFromBP(bpIgnitionFirstboot blueprint.FirstBootIgnitionCustomization) *FirstBootOptions {
	ignition := FirstBootOptions(bpIgnitionFirstboot)
	return &ignition
}

type EmbeddedOptions struct {
	Config string
}

func EmbeddedOptionsFromBP(bpIgnitionEmbedded blueprint.EmbeddedIgnitionCustomization) (*EmbeddedOptions, error) {
	decodedConfig, err := base64.StdEncoding.DecodeString(bpIgnitionEmbedded.Config)
	if err != nil {
		return nil, errors.New("can't decode Ignition config")
	}
	return &EmbeddedOptions{
		Config: string(decodedConfig),
	}, nil
}
