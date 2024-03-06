package osbuild

import (
	"fmt"
	"regexp"
)

var skopeoDigestPattern = regexp.MustCompile(`sha256:[0-9a-f]{64}`)

const DockerTransport = "docker"
const ContainersStorageTransport = "containers-storage"

type SkopeoSource struct {
	Items map[string]SkopeoSourceItem `json:"items"`
}

func (SkopeoSource) isSource() {}

type SkopeopSourceImage struct {
	Name                string  `json:"name,omitempty"`
	Digest              string  `json:"digest,omitempty"`
	TLSVerify           *bool   `json:"tls-verify,omitempty"`
	ContainersTransport *string `json:"containers-transport,omitempty"`
	StorageLocation     *string `json:"storage-location,omitempty"`
}

type SkopeoSourceItem struct {
	Image SkopeopSourceImage `json:"image"`
}

// NewSkopeoSourceItem creates a new source item for name and digest
func NewSkopeoSourceItem(name, digest string, tlsVerify *bool, containersTransport *string, storageLocation *string) SkopeoSourceItem {
	item := SkopeoSourceItem{
		Image: SkopeopSourceImage{
			Name:                name,
			Digest:              digest,
			TLSVerify:           tlsVerify,
			ContainersTransport: containersTransport,
			StorageLocation:     storageLocation,
		},
	}
	if err := item.validate(); err != nil {
		panic(err)
	}
	return item
}

func (item SkopeoSourceItem) validate() error {
	if item.Image.Name == "" {
		return fmt.Errorf("source item %#v has empty name", item)
	}

	if !skopeoDigestPattern.MatchString(item.Image.Digest) {
		return fmt.Errorf("source item %#v has invalid digest", item)
	}

	if err := validateTransport(item.Image.ContainersTransport); err != nil {
		return err
	}

	return nil
}

// NewSkopeoSource creates a new and empty SkopeoSource
func NewSkopeoSource() *SkopeoSource {
	return &SkopeoSource{
		Items: make(map[string]SkopeoSourceItem),
	}
}

// AddItem adds a source item to the source; will panic
// if any of the supplied options are invalid or missing
func (source *SkopeoSource) AddItem(name, digest, image string, tlsVerify *bool, containersTransport *string, storageLocation *string) {
	item := NewSkopeoSourceItem(name, digest, tlsVerify, containersTransport, storageLocation)
	if !skopeoDigestPattern.MatchString(image) {
		panic(fmt.Errorf("item %#v has invalid image id", image))
	}
	source.Items[image] = item
}
