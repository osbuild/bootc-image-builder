package osbuild

import "github.com/osbuild/images/pkg/ostree"

// The commits to fetch indexed their checksum
type OSTreeSource struct {
	Items map[string]OSTreeSourceItem `json:"items"`
}

func (OSTreeSource) isSource() {}

type OSTreeSourceItem struct {
	Remote OSTreeSourceRemote `json:"remote"`
}

type OSTreeSourceRemote struct {
	// URL of the repository.
	URL        string `json:"url"`
	ContentURL string `json:"contenturl,omitempty"`
	// GPG keys to verify the commits
	GPGKeys []string                   `json:"gpgkeys,omitempty"`
	Secrets *OSTreeSourceRemoteSecrets `json:"secrets,omitempty"`
}

type OSTreeSourceRemoteSecrets struct {
	Name string `json:"name"`
}

func NewOSTreeSource() *OSTreeSource {
	return &OSTreeSource{
		Items: make(map[string]OSTreeSourceItem),
	}
}

func NewOSTreeSourceItem(commit ostree.CommitSpec) *OSTreeSourceItem {
	item := new(OSTreeSourceItem)
	item.Remote.URL = commit.URL
	item.Remote.ContentURL = commit.ContentURL
	if commit.Secrets == "org.osbuild.rhsm.consumer" {
		item.Remote.Secrets = &OSTreeSourceRemoteSecrets{
			Name: "org.osbuild.rhsm.consumer",
		}
	}
	return item
}

func (source *OSTreeSource) AddItem(commit ostree.CommitSpec) {
	item := NewOSTreeSourceItem(commit)
	source.Items[commit.Checksum] = *item
}
