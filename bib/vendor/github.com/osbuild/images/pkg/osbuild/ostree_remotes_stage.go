package osbuild

// Configure OSTree remotes for a repository

// Options for the org.osbuild.ostree.remotes stage.
type OSTreeRemotesStageOptions struct {
	// Location of the ostree repo
	Repo string `json:"repo"`

	// Configure remotes for the system repository
	Remotes []OSTreeRemote `json:"remotes,omitempty"`
}

func (OSTreeRemotesStageOptions) isStageOptions() {}

type OSTreeRemote struct {
	// Identifier for the remote
	Name string `json:"name"`

	// URL for accessing metadata and content for the remote
	URL string `json:"url"`

	// URL for accessing content. When set, url is used only for
	// metadata. Supports 'mirrorlist=' prefix
	ContentURL string `json:"contenturl,omitempty"`

	// Configured branches for the remote
	Branches []string `json:"branches,omitempty"`

	// GPG keys to verify the commits
	GPGKeys []string `json:"secrets,omitempty"`

	// Paths to ASCII-armored GPG key or directories containing ASCII-armored
	// GPG keys to import
	GPGKeyPaths []string `json:"gpgkeypaths,omitempty"`
}

// A new org.osbuild.ostree.remotes stage to configure remotes
func NewOSTreeRemotesStage(options *OSTreeRemotesStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.remotes",
		Options: options,
	}
}
