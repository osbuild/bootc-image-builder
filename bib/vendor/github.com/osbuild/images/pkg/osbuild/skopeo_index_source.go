package osbuild

import (
	"fmt"
)

type SkopeoIndexSource struct {
	Items map[string]SkopeoIndexSourceItem `json:"items"`
}

func (SkopeoIndexSource) isSource() {}

type SkopeoIndexSourceImage struct {
	Name                string  `json:"name"`
	TLSVerify           *bool   `json:"tls-verify,omitempty"`
	ContainersTransport *string `json:"containers-transport,omitempty"`
	StorageLocation     *string `json:"storage-location,omitempty"`
}

type SkopeoIndexSourceItem struct {
	Image SkopeoIndexSourceImage `json:"image"`
}

func validateTransport(transport *string) error {
	if transport == nil {
		return nil
	}

	if *transport != DockerTransport && *transport != ContainersStorageTransport {
		return fmt.Errorf("invalid container transport: %s", *transport)
	}

	return nil
}

func (item SkopeoIndexSourceItem) validate() error {

	if item.Image.Name == "" {
		return fmt.Errorf("source item has empty name")
	}

	if err := validateTransport(item.Image.ContainersTransport); err != nil {
		return err
	}

	return nil
}

// NewSkopeoIndexSource creates a new and empty SkopeoIndexSource
func NewSkopeoIndexSource() *SkopeoIndexSource {
	return &SkopeoIndexSource{
		Items: make(map[string]SkopeoIndexSourceItem),
	}
}

// AddItem adds a source item to the source; will panic
// if any of the supplied options are invalid or missing
func (source *SkopeoIndexSource) AddItem(name, image string, tlsVerify *bool, containersTransport *string, storageLocation *string) {
	item := SkopeoIndexSourceItem{
		Image: SkopeoIndexSourceImage{
			Name:                name,
			TLSVerify:           tlsVerify,
			ContainersTransport: containersTransport,
			StorageLocation:     storageLocation,
		},
	}

	if err := item.validate(); err != nil {
		panic(err)
	}

	if !skopeoDigestPattern.MatchString(image) {
		panic("item has invalid image id")
	}

	source.Items[image] = item
}
