package osbuild

import (
	"fmt"
)

type SkopeoIndexSource struct {
	Items map[string]SkopeoIndexSourceItem `json:"items"`
}

func (SkopeoIndexSource) isSource() {}

type SkopeoIndexSourceImage struct {
	Name      string `json:"name"`
	TLSVerify *bool  `json:"tls-verify,omitempty"`
}

type SkopeoIndexSourceItem struct {
	Image SkopeoIndexSourceImage `json:"image"`
}

func (item SkopeoIndexSourceItem) validate() error {

	if item.Image.Name == "" {
		return fmt.Errorf("source item has empty name")
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
func (source *SkopeoIndexSource) AddItem(name, image string, tlsVerify *bool) {
	item := SkopeoIndexSourceItem{
		Image: SkopeoIndexSourceImage{
			Name:      name,
			TLSVerify: tlsVerify,
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
