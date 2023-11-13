package osbuild

import (
	"fmt"
	"regexp"
)

var skopeoDigestPattern = regexp.MustCompile(`sha256:[0-9a-f]{64}`)

type SkopeoSource struct {
	Items map[string]SkopeoSourceItem `json:"items"`
}

func (SkopeoSource) isSource() {}

type SkopeopSourceImage struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	TLSVerify *bool  `json:"tls-verify,omitempty"`
}

type SkopeoSourceItem struct {
	Image SkopeopSourceImage `json:"image"`
}

// NewSkopeoSourceItem creates a new source item for name and digest
func NewSkopeoSourceItem(name, digest string, tlsVerify *bool) SkopeoSourceItem {
	item := SkopeoSourceItem{
		Image: SkopeopSourceImage{
			Name:      name,
			Digest:    digest,
			TLSVerify: tlsVerify,
		},
	}
	if err := item.validate(); err != nil {
		panic(err)
	}
	return item
}

func (item SkopeoSourceItem) validate() error {

	if item.Image.Name == "" {
		return fmt.Errorf("source item has empty name")
	}

	if !skopeoDigestPattern.MatchString(item.Image.Digest) {
		return fmt.Errorf("source item has invalid digest")
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
func (source *SkopeoSource) AddItem(name, digest, image string, tlsVerify *bool) {
	item := NewSkopeoSourceItem(name, digest, tlsVerify)
	if !skopeoDigestPattern.MatchString(image) {
		panic("item has invalid image id")
	}
	source.Items[image] = item
}
