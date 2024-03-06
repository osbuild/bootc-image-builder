package rpmmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gobwas/glob"
)

type repository struct {
	Name           string   `json:"name"`
	BaseURL        string   `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKey         string   `json:"gpgkey,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty"`
	CheckGPG       bool     `json:"check_gpg,omitempty"`
	IgnoreSSL      bool     `json:"ignore_ssl,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ImageTypeTags  []string `json:"image_type_tags,omitempty"`
}

type RepoConfig struct {
	// the repo id is not always required and is ignored in some cases.
	// For example, it is not required in dnf-json, but it is a required
	// field for creating a repo file in `/etc/yum.repos.d/`
	Id             string   `json:"id,omitempty"`
	Name           string   `json:"name,omitempty"`
	BaseURLs       []string `json:"baseurls,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty"`
	CheckGPG       *bool    `json:"check_gpg,omitempty"`
	CheckRepoGPG   *bool    `json:"check_repo_gpg,omitempty"`
	Priority       *int     `json:"priority,omitempty"`
	IgnoreSSL      *bool    `json:"ignore_ssl,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	ImageTypeTags  []string `json:"image_type_tags,omitempty"`
	PackageSets    []string `json:"package_sets,omitempty"`
}

// Hash calculates an ID string that uniquely represents a repository
// configuration.  The Name and ImageTypeTags fields are not considered in the
// calculation.
func (r *RepoConfig) Hash() string {
	bts := func(b bool) string {
		return fmt.Sprintf("%T", b)
	}
	bpts := func(b *bool) string {
		if b == nil {
			return ""
		}
		return fmt.Sprintf("%T", b)
	}
	ats := func(s []string) string {
		return strings.Join(s, "")
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(ats(r.BaseURLs)+
		r.Metalink+
		r.MirrorList+
		ats(r.GPGKeys)+
		bpts(r.CheckGPG)+
		bpts(r.CheckRepoGPG)+
		bpts(r.IgnoreSSL)+
		r.MetadataExpire+
		bts(r.RHSM)+
		bpts(r.ModuleHotfixes))))
}

type DistrosRepoConfigs map[string]map[string][]RepoConfig

type PackageList []Package

type Package struct {
	Name        string
	Summary     string
	Description string
	URL         string
	Epoch       uint
	Version     string
	Release     string
	Arch        string
	BuildTime   time.Time
	License     string
}

func (pkg Package) ToPackageBuild() PackageBuild {
	// Convert the time to the API time format
	return PackageBuild{
		Arch:           pkg.Arch,
		BuildTime:      pkg.BuildTime.Format("2006-01-02T15:04:05"),
		Epoch:          pkg.Epoch,
		Release:        pkg.Release,
		Changelog:      "CHANGELOG_NEEDED", // the same value as lorax-composer puts here
		BuildConfigRef: "BUILD_CONFIG_REF", // the same value as lorax-composer puts here
		BuildEnvRef:    "BUILD_ENV_REF",    // the same value as lorax-composer puts here
		Source: PackageSource{
			License:   pkg.License,
			Version:   pkg.Version,
			SourceRef: "SOURCE_REF", // the same value as lorax-composer puts here
		},
	}
}

func (pkg Package) ToPackageInfo() PackageInfo {
	return PackageInfo{
		Name:         pkg.Name,
		Summary:      pkg.Summary,
		Description:  pkg.Description,
		Homepage:     pkg.URL,
		UpstreamVCS:  "UPSTREAM_VCS", // the same value as lorax-composer puts here
		Builds:       []PackageBuild{pkg.ToPackageBuild()},
		Dependencies: nil,
	}
}

// The inputs to depsolve, a set of packages to include and a set of packages
// to exclude. The Repositories are used when depsolving this package set in
// addition to the base repositories.
type PackageSet struct {
	Include         []string
	Exclude         []string
	Repositories    []RepoConfig
	InstallWeakDeps bool
}

// Append the Include and Exclude package list from another PackageSet and
// return the result.
func (ps PackageSet) Append(other PackageSet) PackageSet {
	ps.Include = append(ps.Include, other.Include...)
	ps.Exclude = append(ps.Exclude, other.Exclude...)
	return ps
}

// TODO: the public API of this package should not be reused for serialization.
type PackageSpec struct {
	Name           string `json:"name"`
	Epoch          uint   `json:"epoch"`
	Version        string `json:"version,omitempty"`
	Release        string `json:"release,omitempty"`
	Arch           string `json:"arch,omitempty"`
	RemoteLocation string `json:"remote_location,omitempty"`
	Checksum       string `json:"checksum,omitempty"`
	Secrets        string `json:"secrets,omitempty"`
	CheckGPG       bool   `json:"check_gpg,omitempty"`
	IgnoreSSL      bool   `json:"ignore_ssl,omitempty"`
}

type PackageSource struct {
	License   string   `json:"license"`
	Version   string   `json:"version"`
	SourceRef string   `json:"source_ref"`
	Metadata  struct{} `json:"metadata"` // it's just an empty struct in lorax-composer
}

type PackageBuild struct {
	Arch           string        `json:"arch"`
	BuildTime      string        `json:"build_time"`
	Epoch          uint          `json:"epoch"`
	Release        string        `json:"release"`
	Source         PackageSource `json:"source"`
	Changelog      string        `json:"changelog"`
	BuildConfigRef string        `json:"build_config_ref"`
	BuildEnvRef    string        `json:"build_env_ref"`
	Metadata       struct{}      `json:"metadata"` // it's just an empty struct in lorax-composer
}

type PackageInfo struct {
	Name         string         `json:"name"`
	Summary      string         `json:"summary"`
	Description  string         `json:"description"`
	Homepage     string         `json:"homepage"`
	UpstreamVCS  string         `json:"upstream_vcs"`
	Builds       []PackageBuild `json:"builds"`
	Dependencies []PackageSpec  `json:"dependencies,omitempty"`
}

// GetEVRA returns the package's Epoch:Version-Release.Arch string
func (ps *PackageSpec) GetEVRA() string {
	if ps.Epoch == 0 {
		return fmt.Sprintf("%s-%s.%s", ps.Version, ps.Release, ps.Arch)
	}
	return fmt.Sprintf("%d:%s-%s.%s", ps.Epoch, ps.Version, ps.Release, ps.Arch)
}

// GetNEVRA returns the package's Name-Epoch:Version-Release.Arch string
func (ps *PackageSpec) GetNEVRA() string {
	return fmt.Sprintf("%s-%s", ps.Name, ps.GetEVRA())
}

func GetVerStrFromPackageSpecList(pkgs []PackageSpec, packageName string) (string, error) {
	for _, pkg := range pkgs {
		if pkg.Name == packageName {
			return fmt.Sprintf("%s-%s.%s", pkg.Version, pkg.Release, pkg.Arch), nil
		}
	}
	return "", fmt.Errorf("package %q not found in the PackageSpec list", packageName)
}

func GetVerStrFromPackageSpecListPanic(pkgs []PackageSpec, packageName string) string {
	pkgVerStr, err := GetVerStrFromPackageSpecList(pkgs, packageName)
	if err != nil {
		panic(err)
	}
	return pkgVerStr
}

func LoadRepositoriesFromFile(filename string) (map[string][]RepoConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reposMap map[string][]repository
	repoConfigs := make(map[string][]RepoConfig)

	err = json.NewDecoder(f).Decode(&reposMap)
	if err != nil {
		return nil, err
	}

	for arch, repos := range reposMap {
		for idx := range repos {
			repo := repos[idx]
			var urls []string
			if repo.BaseURL != "" {
				urls = []string{repo.BaseURL}
			}
			var keys []string
			if repo.GPGKey != "" {
				keys = []string{repo.GPGKey}
			}
			if len(repo.GPGKeys) > 0 {
				keys = append(keys, repo.GPGKeys...)
			}
			config := RepoConfig{
				Name:           repo.Name,
				BaseURLs:       urls,
				Metalink:       repo.Metalink,
				MirrorList:     repo.MirrorList,
				GPGKeys:        keys,
				CheckGPG:       &repo.CheckGPG,
				RHSM:           repo.RHSM,
				MetadataExpire: repo.MetadataExpire,
				ModuleHotfixes: repo.ModuleHotfixes,
				ImageTypeTags:  repo.ImageTypeTags,
			}

			repoConfigs[arch] = append(repoConfigs[arch], config)
		}
	}

	return repoConfigs, nil
}

func (packages PackageList) Search(globPatterns ...string) (PackageList, error) {
	var globs []glob.Glob

	for _, globPattern := range globPatterns {
		g, err := glob.Compile(globPattern)
		if err != nil {
			return nil, err
		}

		globs = append(globs, g)
	}

	var foundPackages PackageList

	for _, pkg := range packages {
		for _, g := range globs {
			if g.Match(pkg.Name) {
				foundPackages = append(foundPackages, pkg)
				break
			}
		}
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	return foundPackages, nil
}

func (packages PackageList) ToPackageInfos() []PackageInfo {
	resultsNames := make(map[string]int)
	var results []PackageInfo

	for _, pkg := range packages {
		if index, ok := resultsNames[pkg.Name]; ok {
			foundPkg := &results[index]

			foundPkg.Builds = append(foundPkg.Builds, pkg.ToPackageBuild())
		} else {
			newIndex := len(results)
			resultsNames[pkg.Name] = newIndex

			packageInfo := pkg.ToPackageInfo()

			results = append(results, packageInfo)
		}
	}

	return results
}

// Backwards compatibility for old workers:
// This was added since the custom repository
// PR changes the baseurl field to a list of baseurls.
// This can be removed after 3 releases since the
// old-worker-regression test tests the current
// osbuild-composer with a worker from 3 releases ago
func (r RepoConfig) MarshalJSON() ([]byte, error) {
	type aliasType RepoConfig
	type compatType struct {
		aliasType

		BaseURL string `json:"baseurl,omitempty"`
	}
	compatRepo := compatType{
		aliasType: aliasType(r),
	}

	var baseUrl string
	if len(r.BaseURLs) > 0 {
		baseUrl = strings.Join(r.BaseURLs, ",")
	}

	compatRepo.BaseURL = baseUrl

	return json.Marshal(compatRepo)
}

// Backwards compatibility for old workers:
// This was added since the custom repository
// PR changes the baseurl field to a list of baseurls.
// This can be removed after 3 releases since the
// old-worker-regression test tests the current
// osbuild-composer with a worker from 3 releases ago
func (r *RepoConfig) UnmarshalJSON(data []byte) error {
	type aliasType RepoConfig
	type compatType struct {
		aliasType

		BaseURL string `json:"baseurl,omitempty"`
	}

	var compatRepo compatType
	if err := json.Unmarshal(data, &compatRepo); err != nil {
		return err
	}

	if compatRepo.BaseURL != "" {
		compatRepo.BaseURLs = strings.Split(compatRepo.BaseURL, ",")
	}

	*r = RepoConfig(compatRepo.aliasType)
	return nil
}
