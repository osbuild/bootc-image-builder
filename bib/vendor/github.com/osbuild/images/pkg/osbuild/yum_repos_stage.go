package osbuild

import (
	"fmt"
	"regexp"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/rpmmd"
)

const repoFilenameRegex = "^[\\w.-]{1,250}\\.repo$"
const repoIDRegex = "^[\\w.\\-:]+$"

// YumRepository represents a single DNF / YUM repository.
type YumRepository struct {
	Id             string   `json:"id"`
	BaseURLs       []string `json:"baseurl,omitempty"`
	Cost           *int     `json:"cost,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	Priority       *int     `json:"priority,omitempty"`
	GPGKey         []string `json:"gpgkey,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	Mirrorlist     string   `json:"mirrorlist,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
	Name           string   `json:"name,omitempty"`
	GPGCheck       *bool    `json:"gpgcheck,omitempty"`
	RepoGPGCheck   *bool    `json:"repo_gpgcheck,omitempty"`
	SSLVerify      *bool    `json:"sslverify,omitempty"`
}

func (r YumRepository) validate() error {
	// Plain string values which can not be empty strings as mandated by
	// the stage schema are not validated. The reason is that if they
	// are empty, they will be omitted from the resulting JSON, therefore
	// they will be never passed to osbuild in the stage options.
	// The same logic is applied to slices of strings, which must have
	// at least one item if defined. These won't appear in the resulting
	// JSON if the slice it empty.

	idRegex := regexp.MustCompile(repoIDRegex)
	if !idRegex.MatchString(r.Id) {
		return fmt.Errorf("repo ID %q doesn't conform to schema (%s)", r.Id, repoIDRegex)
	}

	// at least one of baseurl, metalink or mirrorlist must be provided
	if len(r.BaseURLs) == 0 && r.Metalink == "" && r.Mirrorlist == "" {
		return fmt.Errorf("at least one of baseurl, metalink or mirrorlist values must be provided")
	}

	for idx, url := range r.BaseURLs {
		if url == "" {
			return fmt.Errorf("baseurl must not be an empty string (idx %d)", idx)
		}
	}

	for idx, gpgkey := range r.GPGKey {
		if gpgkey == "" {
			return fmt.Errorf("gpgkey must not be an empty string (idx %d)", idx)
		}
	}

	return nil
}

// YumReposStageOptions represents a single DNF / YUM repo configuration file.
type YumReposStageOptions struct {
	// Filename of the configuration file to be created. Must end with '.repo'.
	Filename string `json:"filename"`
	// List of repositories. The list must contain at least one item.
	Repos []YumRepository `json:"repos"`
}

func (YumReposStageOptions) isStageOptions() {}

// NewYumReposStageOptions creates a new YumRepos Stage options object.
func NewYumReposStageOptions(filename string, repos []rpmmd.RepoConfig) *YumReposStageOptions {
	var yumRepos []YumRepository
	for _, repo := range repos {
		yumRepos = append(yumRepos, repoConfigToYumRepository(repo))
	}

	return &YumReposStageOptions{
		Filename: filename,
		Repos:    yumRepos,
	}
}

func repoConfigToYumRepository(repo rpmmd.RepoConfig) YumRepository {
	urls := make([]string, len(repo.BaseURLs))
	copy(urls, repo.BaseURLs)

	keys := make([]string, len(repo.GPGKeys))
	copy(keys, repo.GPGKeys)

	var sslVerify *bool
	if repo.IgnoreSSL != nil {
		ignoreSSL := *repo.IgnoreSSL
		sslVerify = common.ToPtr(!ignoreSSL)
	}

	yumRepo := YumRepository{
		Id:             repo.Id,
		Name:           repo.Name,
		Mirrorlist:     repo.MirrorList,
		Metalink:       repo.Metalink,
		BaseURLs:       urls,
		GPGKey:         keys,
		GPGCheck:       repo.CheckGPG,
		RepoGPGCheck:   repo.CheckRepoGPG,
		Enabled:        repo.Enabled,
		Priority:       repo.Priority,
		SSLVerify:      sslVerify,
		ModuleHotfixes: repo.ModuleHotfixes,
	}

	return yumRepo
}

func (o YumReposStageOptions) validate() error {
	filenameRegex := regexp.MustCompile(repoFilenameRegex)
	if !filenameRegex.MatchString(o.Filename) {
		return fmt.Errorf("filename %q doesn't conform to schema (%s)", o.Filename, repoFilenameRegex)
	}

	if len(o.Repos) == 0 {
		return fmt.Errorf("at least one repository must be defined")
	}

	for idx, repo := range o.Repos {
		if err := repo.validate(); err != nil {
			return fmt.Errorf("validation of repository #%d failed: %s", idx, err)
		}
	}

	return nil
}

func NewYumReposStage(options *YumReposStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.yum.repos",
		Options: options,
	}
}
