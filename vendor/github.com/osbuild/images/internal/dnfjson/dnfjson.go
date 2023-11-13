// Package dnfjson is an interface to the dnf-json Python script that is
// packaged with the osbuild-composer project. The core component of this
// package is the Solver type. The Solver can be configured with
// distribution-specific values (platform ID, architecture, and version
// information) and provides methods for dependency resolution (Depsolve) and
// retrieving a full list of repository package metadata (FetchMetadata).
//
// Alternatively, a BaseSolver can be created which represents an un-configured
// Solver. This type can't be used for depsolving, but can be used to create
// configured Solver instances sharing the same cache directory.
//
// This package relies on the types defined in rpmmd to describe RPM package
// metadata.
package dnfjson

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/osbuild/images/pkg/rhsm"
	"github.com/osbuild/images/pkg/rpmmd"
)

// BaseSolver defines the basic solver configuration without platform
// information. It can be used to create configured Solver instances with the
// NewWithConfig() method. A BaseSolver maintains the global repository cache
// directory.
type BaseSolver struct {
	// Cache information
	cache *rpmCache

	// Path to the dnf-json binary and optional args (default: "/usr/libexec/osbuild-composer/dnf-json")
	dnfJsonCmd []string

	resultCache *dnfCache
}

// Create a new unconfigured BaseSolver (without platform information). It can
// be used to create configured Solver instances with the NewWithConfig()
// method.
func NewBaseSolver(cacheDir string) *BaseSolver {
	return &BaseSolver{
		cache:       newRPMCache(cacheDir, 1024*1024*1024), // 1 GiB
		dnfJsonCmd:  []string{"/usr/libexec/osbuild-composer/dnf-json"},
		resultCache: NewDNFCache(60 * time.Second),
	}
}

// SetMaxCacheSize sets the maximum size for the global repository metadata
// cache. This is the maximum size of the cache after a CleanCache()
// call. Cache cleanup is never performed automatically.
func (s *BaseSolver) SetMaxCacheSize(size uint64) {
	s.cache.maxSize = size
}

// SetDNFJSONPath sets the path to the dnf-json binary and optionally any command line arguments.
func (s *BaseSolver) SetDNFJSONPath(cmd string, args ...string) {
	s.dnfJsonCmd = append([]string{cmd}, args...)
}

// NewWithConfig initialises a Solver with the platform information and the
// BaseSolver's subscription info, cache directory, and dnf-json path.
// Also loads system subscription information.
func (bs *BaseSolver) NewWithConfig(modulePlatformID, releaseVer, arch, distro string) *Solver {
	s := new(Solver)
	s.BaseSolver = *bs
	s.modulePlatformID = modulePlatformID
	s.arch = arch
	s.releaseVer = releaseVer
	s.distro = distro
	subs, _ := rhsm.LoadSystemSubscriptions()
	s.subscriptions = subs
	return s
}

// CleanCache deletes the least recently used repository metadata caches until
// the total size of the cache falls below the configured maximum size (see
// SetMaxCacheSize()).
func (bs *BaseSolver) CleanCache() error {
	bs.resultCache.CleanCache()
	return bs.cache.shrink()
}

// Solver is configured with system information in order to resolve
// dependencies for RPM packages using DNF.
type Solver struct {
	BaseSolver

	// Platform ID, e.g., "platform:el8"
	modulePlatformID string

	// System architecture
	arch string

	// Release version of the distro. This is used in repo files on the host
	// system and required for subscription support.
	releaseVer string

	// Full distribution string, eg. fedora-38, used to create separate dnf cache directories
	// for each distribution.
	distro string

	subscriptions *rhsm.Subscriptions
}

// Create a new Solver with the given configuration. Initialising a Solver also loads system subscription information.
func NewSolver(modulePlatformID, releaseVer, arch, distro, cacheDir string) *Solver {
	s := NewBaseSolver(cacheDir)
	return s.NewWithConfig(modulePlatformID, releaseVer, arch, distro)
}

// GetCacheDir returns a distro specific rpm cache directory
// It ensures that the distro name is below the root cache directory, and if there is
// a problem it returns the root cache intead of an error.
func (s *Solver) GetCacheDir() string {
	b := filepath.Base(s.distro)
	if b == "." || b == "/" {
		return s.cache.root
	}

	return filepath.Join(s.cache.root, s.distro)
}

// Depsolve the list of required package sets with explicit excludes using
// their associated repositories.  Each package set is depsolved as a separate
// transactions in a chain.  It returns a list of all packages (with solved
// dependencies) that will be installed into the system.
func (s *Solver) Depsolve(pkgSets []rpmmd.PackageSet) ([]rpmmd.PackageSpec, error) {
	req, repoMap, err := s.makeDepsolveRequest(pkgSets)
	if err != nil {
		return nil, err
	}

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	output, err := run(s.dnfJsonCmd, req)
	if err != nil {
		return nil, err
	}
	// touch repos to now
	now := time.Now().Local()
	for _, r := range repoMap {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	var result packageSpecs
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	return result.toRPMMD(repoMap), nil
}

// FetchMetadata returns the list of all the available packages in repos and
// their info.
func (s *Solver) FetchMetadata(repos []rpmmd.RepoConfig) (rpmmd.PackageList, error) {
	req, err := s.makeDumpRequest(repos)
	if err != nil {
		return nil, err
	}

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	// Is this cached?
	if pkgs, ok := s.resultCache.Get(req.Hash()); ok {
		return pkgs, nil
	}

	result, err := run(s.dnfJsonCmd, req)
	if err != nil {
		return nil, err
	}

	// touch repos to now
	now := time.Now().Local()
	for _, r := range repos {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	var pkgs rpmmd.PackageList
	if err := json.Unmarshal(result, &pkgs); err != nil {
		return nil, err
	}

	sortID := func(pkg rpmmd.Package) string {
		return fmt.Sprintf("%s-%s-%s", pkg.Name, pkg.Version, pkg.Release)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return sortID(pkgs[i]) < sortID(pkgs[j])
	})

	// Cache the results
	s.resultCache.Store(req.Hash(), pkgs)
	return pkgs, nil
}

// SearchMetadata searches for packages and returns a list of the info for matches.
func (s *Solver) SearchMetadata(repos []rpmmd.RepoConfig, packages []string) (rpmmd.PackageList, error) {
	req, err := s.makeSearchRequest(repos, packages)
	if err != nil {
		return nil, err
	}

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	// Is this cached?
	if pkgs, ok := s.resultCache.Get(req.Hash()); ok {
		return pkgs, nil
	}

	result, err := run(s.dnfJsonCmd, req)
	if err != nil {
		return nil, err
	}

	// touch repos to now
	now := time.Now().Local()
	for _, r := range repos {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	var pkgs rpmmd.PackageList
	if err := json.Unmarshal(result, &pkgs); err != nil {
		return nil, err
	}

	sortID := func(pkg rpmmd.Package) string {
		return fmt.Sprintf("%s-%s-%s", pkg.Name, pkg.Version, pkg.Release)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return sortID(pkgs[i]) < sortID(pkgs[j])
	})

	// Cache the results
	s.resultCache.Store(req.Hash(), pkgs)
	return pkgs, nil
}

func (s *Solver) reposFromRPMMD(rpmRepos []rpmmd.RepoConfig) ([]repoConfig, error) {
	dnfRepos := make([]repoConfig, len(rpmRepos))
	for idx, rr := range rpmRepos {
		dr := repoConfig{
			ID:             rr.Hash(),
			Name:           rr.Name,
			BaseURLs:       rr.BaseURLs,
			Metalink:       rr.Metalink,
			MirrorList:     rr.MirrorList,
			GPGKeys:        rr.GPGKeys,
			MetadataExpire: rr.MetadataExpire,
			repoHash:       rr.Hash(),
		}

		if rr.CheckGPG != nil {
			dr.CheckGPG = *rr.CheckGPG
		}

		if rr.CheckRepoGPG != nil {
			dr.CheckRepoGPG = *rr.CheckRepoGPG
		}

		if rr.IgnoreSSL != nil {
			dr.IgnoreSSL = *rr.IgnoreSSL
		}

		if rr.RHSM {
			if s.subscriptions == nil {
				return nil, fmt.Errorf("This system does not have any valid subscriptions. Subscribe it before specifying rhsm: true in sources.")
			}
			secrets, err := s.subscriptions.GetSecretsForBaseurl(rr.BaseURLs, s.arch, s.releaseVer)
			if err != nil {
				return nil, fmt.Errorf("RHSM secrets not found on the host for this baseurl: %s", rr.BaseURLs)
			}
			dr.SSLCACert = secrets.SSLCACert
			dr.SSLClientKey = secrets.SSLClientKey
			dr.SSLClientCert = secrets.SSLClientCert
		}
		dnfRepos[idx] = dr
	}
	return dnfRepos, nil
}

// Repository configuration for resolving dependencies for a set of packages. A
// Solver needs at least one RPM repository configured to be able to depsolve.
type repoConfig struct {
	ID             string   `json:"id"`
	Name           string   `json:"name,omitempty"`
	BaseURLs       []string `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty"`
	CheckGPG       bool     `json:"gpgcheck"`
	CheckRepoGPG   bool     `json:"check_repogpg"`
	IgnoreSSL      bool     `json:"ignoressl"`
	SSLCACert      string   `json:"sslcacert,omitempty"`
	SSLClientKey   string   `json:"sslclientkey,omitempty"`
	SSLClientCert  string   `json:"sslclientcert,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	// set the repo hass from `rpmmd.RepoConfig.Hash()` function
	// rather than re-calculating it
	repoHash string
}

// use the hash calculated by the `rpmmd.RepoConfig.Hash()`
// function rather than re-implementing the same code
func (r *repoConfig) Hash() string {
	return r.repoHash
}

// Helper function for creating a depsolve request payload.
// The request defines a sequence of transactions, each depsolving one of the
// elements of `pkgSets` in the order they appear.  The `repoConfigs` are used
// as the base repositories for all transactions.  The extra repository configs
// in `pkgsetsRepos` are used for each of the `pkgSets` with matching index.
// The length of `pkgsetsRepos` must match the length of `pkgSets` or be empty
// (nil or empty slice).
//
// NOTE: Due to implementation limitations of DNF and dnf-json, each package set
// in the chain must use all of the repositories used by its predecessor.
// An error is returned if this requirement is not met.
func (s *Solver) makeDepsolveRequest(pkgSets []rpmmd.PackageSet) (*Request, map[string]rpmmd.RepoConfig, error) {

	// dedupe repository configurations but maintain order
	// the order in which repositories are added to the request affects the
	// order of the dependencies in the result
	repos := make([]rpmmd.RepoConfig, 0)
	rpmRepoMap := make(map[string]rpmmd.RepoConfig)

	for _, ps := range pkgSets {
		for _, repo := range ps.Repositories {
			id := repo.Hash()
			if _, ok := rpmRepoMap[id]; !ok {
				rpmRepoMap[id] = repo
				repos = append(repos, repo)
			}
		}
	}

	transactions := make([]transactionArgs, len(pkgSets))
	for dsIdx, pkgSet := range pkgSets {
		transactions[dsIdx] = transactionArgs{
			PackageSpecs:    pkgSet.Include,
			ExcludeSpecs:    pkgSet.Exclude,
			InstallWeakDeps: pkgSet.InstallWeakDeps,
		}

		for _, jobRepo := range pkgSet.Repositories {
			transactions[dsIdx].RepoIDs = append(transactions[dsIdx].RepoIDs, jobRepo.Hash())
		}

		// If more than one transaction, ensure that the transaction uses
		// all of the repos from its predecessor
		if dsIdx > 0 {
			prevRepoIDs := transactions[dsIdx-1].RepoIDs
			if len(transactions[dsIdx].RepoIDs) < len(prevRepoIDs) {
				return nil, nil, fmt.Errorf("chained packageSet %d does not use all of the repos used by its predecessor", dsIdx)
			}

			for idx, repoID := range prevRepoIDs {
				if repoID != transactions[dsIdx].RepoIDs[idx] {
					return nil, nil, fmt.Errorf("chained packageSet %d does not use all of the repos used by its predecessor", dsIdx)
				}
			}
		}
	}

	dnfRepoMap, err := s.reposFromRPMMD(repos)
	if err != nil {
		return nil, nil, err
	}
	args := arguments{
		Repos:        dnfRepoMap,
		Transactions: transactions,
	}

	req := Request{
		Command:          "depsolve",
		ModulePlatformID: s.modulePlatformID,
		Arch:             s.arch,
		CacheDir:         s.GetCacheDir(),
		Arguments:        args,
	}

	return &req, rpmRepoMap, nil
}

// Helper function for creating a dump request payload
func (s *Solver) makeDumpRequest(repos []rpmmd.RepoConfig) (*Request, error) {
	dnfRepos, err := s.reposFromRPMMD(repos)
	if err != nil {
		return nil, err
	}
	req := Request{
		Command:          "dump",
		ModulePlatformID: s.modulePlatformID,
		Arch:             s.arch,
		CacheDir:         s.GetCacheDir(),
		Arguments: arguments{
			Repos: dnfRepos,
		},
	}
	return &req, nil
}

// Helper function for creating a search request payload
func (s *Solver) makeSearchRequest(repos []rpmmd.RepoConfig, packages []string) (*Request, error) {
	dnfRepos, err := s.reposFromRPMMD(repos)
	if err != nil {
		return nil, err
	}
	req := Request{
		Command:          "search",
		ModulePlatformID: s.modulePlatformID,
		Arch:             s.arch,
		CacheDir:         s.GetCacheDir(),
		Arguments: arguments{
			Repos: dnfRepos,
			Search: searchArgs{
				Packages: packages,
			},
		},
	}
	return &req, nil
}

// convert internal a list of PackageSpecs to the rpmmd equivalent and attach
// key and subscription information based on the repository configs.
func (pkgs packageSpecs) toRPMMD(repos map[string]rpmmd.RepoConfig) []rpmmd.PackageSpec {
	rpmDependencies := make([]rpmmd.PackageSpec, len(pkgs))
	for i, dep := range pkgs {
		repo, ok := repos[dep.RepoID]
		if !ok {
			panic("dependency repo ID not found in repositories")
		}
		dep := pkgs[i]
		rpmDependencies[i].Name = dep.Name
		rpmDependencies[i].Epoch = dep.Epoch
		rpmDependencies[i].Version = dep.Version
		rpmDependencies[i].Release = dep.Release
		rpmDependencies[i].Arch = dep.Arch
		rpmDependencies[i].RemoteLocation = dep.RemoteLocation
		rpmDependencies[i].Checksum = dep.Checksum
		if repo.CheckGPG != nil {
			rpmDependencies[i].CheckGPG = *repo.CheckGPG
		}
		if repo.IgnoreSSL != nil {
			rpmDependencies[i].IgnoreSSL = *repo.IgnoreSSL
		}
		if repo.RHSM {
			rpmDependencies[i].Secrets = "org.osbuild.rhsm"
		}
	}
	return rpmDependencies
}

// Request command and arguments for dnf-json
type Request struct {
	// Command should be either "depsolve" or "dump"
	Command string `json:"command"`

	// Platform ID, e.g., "platform:el8"
	ModulePlatformID string `json:"module_platform_id"`

	// System architecture
	Arch string `json:"arch"`

	// Cache directory for the DNF metadata
	CacheDir string `json:"cachedir"`

	// Arguments for the action defined by Command
	Arguments arguments `json:"arguments"`
}

// Hash returns a hash of the unique aspects of the Request
//
//nolint:errcheck
func (r *Request) Hash() string {
	h := sha256.New()

	h.Write([]byte(r.Command))
	h.Write([]byte(r.ModulePlatformID))
	h.Write([]byte(r.Arch))
	for _, repo := range r.Arguments.Repos {
		h.Write([]byte(repo.Hash()))
	}
	h.Write([]byte(fmt.Sprintf("%T", r.Arguments.Search.Latest)))
	h.Write([]byte(strings.Join(r.Arguments.Search.Packages, "")))

	return fmt.Sprintf("%x", h.Sum(nil))
}

// arguments for a dnf-json request
type arguments struct {
	// Repositories to use for depsolving
	Repos []repoConfig `json:"repos"`

	// Search terms to use with search command
	Search searchArgs `json:"search"`

	// Depsolve package sets and repository mappings for this request
	Transactions []transactionArgs `json:"transactions"`
}

type searchArgs struct {
	// Only include latest NEVRA when true
	Latest bool `json:"latest"`

	// List of package name globs to search for
	// If it has '*' it is passed to dnf glob search, if it has *name* it is passed
	// to substr matching, and if it has neither an exact match is expected.
	Packages []string `json:"packages"`
}

type transactionArgs struct {
	// Packages to depsolve
	PackageSpecs []string `json:"package-specs"`

	// Packages to exclude from results
	ExcludeSpecs []string `json:"exclude-specs"`

	// IDs of repositories to use for this depsolve
	RepoIDs []string `json:"repo-ids"`

	// If we want weak deps for this depsolve
	InstallWeakDeps bool `json:"install_weak_deps"`
}

type packageSpecs []PackageSpec

// Package specification
type PackageSpec struct {
	Name           string `json:"name"`
	Epoch          uint   `json:"epoch"`
	Version        string `json:"version,omitempty"`
	Release        string `json:"release,omitempty"`
	Arch           string `json:"arch,omitempty"`
	RepoID         string `json:"repo_id,omitempty"`
	Path           string `json:"path,omitempty"`
	RemoteLocation string `json:"remote_location,omitempty"`
	Checksum       string `json:"checksum,omitempty"`
	Secrets        string `json:"secrets,omitempty"`
}

// dnf-json error structure
type Error struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

func (err Error) Error() string {
	return fmt.Sprintf("DNF error occurred: %s: %s", err.Kind, err.Reason)
}

// parseError parses the response from dnf-json into the Error type and appends
// the name and URL of a repository to all detected repository IDs in the
// message.
func parseError(data []byte, repos []repoConfig) Error {
	var e Error
	if err := json.Unmarshal(data, &e); err != nil {
		// dumping the error into the Reason can get noisy, but it's good for troubleshooting
		return Error{
			Kind:   "InternalError",
			Reason: fmt.Sprintf("Failed to unmarshal dnf-json error output %q: %s", string(data), err.Error()),
		}
	}

	// append to any instance of a repository ID the URL (or metalink, mirrorlist, etc)
	for _, repo := range repos {
		idstr := fmt.Sprintf("'%s'", repo.ID)
		var nameURL string
		if len(repo.BaseURLs) > 0 {
			nameURL = strings.Join(repo.BaseURLs, ",")
		} else if len(repo.Metalink) > 0 {
			nameURL = repo.Metalink
		} else if len(repo.MirrorList) > 0 {
			nameURL = repo.MirrorList
		}

		if len(repo.Name) > 0 {
			nameURL = fmt.Sprintf("%s: %s", repo.Name, nameURL)
		}
		e.Reason = strings.Replace(e.Reason, idstr, fmt.Sprintf("%s [%s]", idstr, nameURL), -1)
	}

	return e
}
func ParseError(data []byte) Error {
	var e Error
	if err := json.Unmarshal(data, &e); err != nil {
		// dumping the error into the Reason can get noisy, but it's good for troubleshooting
		return Error{
			Kind:   "InternalError",
			Reason: fmt.Sprintf("Failed to unmarshal dnf-json error output %q: %s", string(data), err.Error()),
		}
	}
	return e
}

func run(dnfJsonCmd []string, req *Request) ([]byte, error) {
	if len(dnfJsonCmd) == 0 {
		return nil, fmt.Errorf("dnf-json command undefined")
	}
	ex := dnfJsonCmd[0]
	args := make([]string, len(dnfJsonCmd)-1)
	if len(dnfJsonCmd) > 1 {
		args = dnfJsonCmd[1:]
	}
	cmd := exec.Command(ex, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	cmd.Stderr = os.Stderr
	stdout := new(bytes.Buffer)
	cmd.Stdout = stdout

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	err = json.NewEncoder(stdin).Encode(req)
	if err != nil {
		return nil, err
	}
	stdin.Close()

	err = cmd.Wait()
	output := stdout.Bytes()
	if runError, ok := err.(*exec.ExitError); ok && runError.ExitCode() != 0 {
		return nil, parseError(output, req.Arguments.Repos)
	}

	return output, nil
}
