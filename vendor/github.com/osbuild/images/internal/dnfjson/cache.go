package dnfjson

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/osbuild/images/pkg/rpmmd"

	"github.com/gobwas/glob"
)

// CleanupOldCacheDirs will remove cache directories for unsupported distros
// eg. Once support for a fedora release stops and it is removed, this will
// delete its directory under root.
//
// A happy side effect of this is that it will delete old cache directories
// and files from before the switch to per-distro cache directories.
//
// NOTE: This does not return any errors. This is because the most common one
// will be a nonexistant directory which will be created later, during initial
// cache creation. Any other errors like permission issues will be caught by
// later use of the cache. eg. touchRepo
func CleanupOldCacheDirs(root string, distros []string) {
	dirs, _ := os.ReadDir(root)

	for _, e := range dirs {
		if strSliceContains(distros, e.Name()) {
			// known distro
			continue
		}
		if e.IsDir() {
			// Remove the directory and everything under it
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
		} else {
			_ = os.Remove(filepath.Join(root, e.Name()))
		}
	}
}

// strSliceContains returns true if the elem string is in the slc array
func strSliceContains(slc []string, elem string) bool {
	for _, s := range slc {
		if elem == s {
			return true
		}
	}
	return false
}

// global cache locker
var cacheLocks sync.Map

// A collection of directory paths, their total size, and their most recent
// modification time.
type pathInfo struct {
	paths []string
	size  uint64
	mtime time.Time
}

type rpmCache struct {
	// root path for the cache
	root string

	// individual repository cache data
	repoElements map[string]pathInfo

	// list of known repository IDs, sorted by mtime
	repoRecency []string

	// total cache size
	size uint64

	// max cache size
	maxSize uint64

	// locker for this cache directory
	locker *sync.RWMutex
}

func newRPMCache(path string, maxSize uint64) *rpmCache {
	absPath, err := filepath.Abs(path) // convert to abs if it's not already
	if err != nil {
		panic(err) // can only happen if the CWD does not exist and the path isn't already absolute
	}
	path = absPath
	locker := new(sync.RWMutex)
	if l, loaded := cacheLocks.LoadOrStore(path, locker); loaded {
		// value existed and was loaded
		locker = l.(*sync.RWMutex)
	}
	r := &rpmCache{
		root:         path,
		repoElements: make(map[string]pathInfo),
		size:         0,
		maxSize:      maxSize,
		locker:       locker,
	}
	// collect existing cache paths and timestamps
	r.updateInfo()
	return r
}

// updateInfo updates the repoPaths and repoRecency fields of the rpmCache.
//
// NOTE: This does not return any errors. This is because the most common one
// will be a nonexistant directory which will be created later, during initial
// cache creation. Any other errors like permission issues will be caught by
// later use of the cache. eg. touchRepo
func (r *rpmCache) updateInfo() {
	dirs, _ := os.ReadDir(r.root)
	for _, d := range dirs {
		r.updateCacheDirInfo(filepath.Join(r.root, d.Name()))
	}
}

func (r *rpmCache) updateCacheDirInfo(path string) {
	// See updateInfo NOTE on error handling
	cacheEntries, _ := os.ReadDir(path)

	// each repository has multiple cache entries (3 on average), so using the
	// number of cacheEntries to allocate the map and ID slice is a high upper
	// bound, but guarantees we wont need to grow and reallocate either.
	repos := make(map[string]pathInfo, len(cacheEntries))
	repoIDs := make([]string, 0, len(cacheEntries))

	var totalSize uint64

	// Collect the paths grouped by their repo ID
	// We assume the first 64 characters of a file or directory name are the
	// repository ID because we use a sha256 sum of the repository config to
	// create the ID (64 hex chars)
	for _, entry := range cacheEntries {
		eInfo, err := entry.Info()
		if err != nil {
			// skip it
			continue
		}

		fname := entry.Name()
		if len(fname) < 64 {
			// unknown file in cache; ignore
			continue
		}
		repoID := fname[:64]
		repo, ok := repos[repoID]
		if !ok {
			// new repo ID
			repoIDs = append(repoIDs, repoID)
		}
		mtime := eInfo.ModTime()
		ePath := filepath.Join(path, entry.Name())

		// calculate and add entry size
		size, err := dirSize(ePath)
		if err != nil {
			// skip it
			continue
		}
		repo.size += size
		totalSize += size

		// add path
		repo.paths = append(repo.paths, ePath)

		// if for some reason the mtimes of the various entries of a single
		// repository are out of sync, use the most recent one
		if repo.mtime.Before(mtime) {
			repo.mtime = mtime
		}

		// update the collection
		repos[repoID] = repo
	}
	sortFunc := func(idx, jdx int) bool {
		ir := repos[repoIDs[idx]]
		jr := repos[repoIDs[jdx]]
		return ir.mtime.Before(jr.mtime)
	}

	// sort IDs by mtime (oldest first)
	sort.Slice(repoIDs, sortFunc)

	r.size = totalSize
	r.repoElements = repos
	r.repoRecency = repoIDs
}

func (r *rpmCache) shrink() error {
	r.locker.Lock()
	defer r.locker.Unlock()

	// start deleting until we drop below r.maxSize
	nDeleted := 0
	for idx := 0; idx < len(r.repoRecency) && r.size >= r.maxSize; idx++ {
		repoID := r.repoRecency[idx]
		nDeleted++
		repo, ok := r.repoElements[repoID]
		if !ok {
			// cache inconsistency?
			// ignore and let the ID be removed from the recency list
			continue
		}
		for _, gPath := range repo.paths {
			if err := os.RemoveAll(gPath); err != nil {
				return err
			}
		}
		r.size -= repo.size
		delete(r.repoElements, repoID)
	}

	// update recency list
	r.repoRecency = r.repoRecency[nDeleted:]
	return nil
}

// Update file atime and mtime on the filesystem to time t for all files in the
// root of the cache that match the repo ID.  This should be called whenever a
// repository is used.
// This function does not update the internal cache info.  A call to
// updateInfo() should be made after touching one or more repositories.
func (r *rpmCache) touchRepo(repoID string, t time.Time) error {
	repoGlob, err := glob.Compile(fmt.Sprintf("%s*", repoID))
	if err != nil {
		return err
	}

	distroDirs, err := os.ReadDir(r.root)
	if err != nil {
		return err
	}
	for _, d := range distroDirs {
		// we only touch the top-level directories and files of the cache
		cacheEntries, err := os.ReadDir(filepath.Join(r.root, d.Name()))
		if err != nil {
			return err
		}

		for _, cacheEntry := range cacheEntries {
			if repoGlob.Match(cacheEntry.Name()) {
				path := filepath.Join(r.root, d.Name(), cacheEntry.Name())
				if err := os.Chtimes(path, t, t); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func dirSize(path string) (uint64, error) {
	var size uint64
	sizer := func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		size += uint64(info.Size())
		return nil
	}
	err := filepath.Walk(path, sizer)
	return size, err
}

// dnfResults holds the results of a dnfjson request
// expire is the time the request was made, used to expire the entry
type dnfResults struct {
	expire time.Time
	pkgs   rpmmd.PackageList
}

// dnfCache is a cache of results from dnf-json requests
type dnfCache struct {
	results map[string]dnfResults
	timeout time.Duration
	*sync.RWMutex
}

// NewDNFCache returns a pointer to an initialized dnfCache struct
func NewDNFCache(timeout time.Duration) *dnfCache {
	return &dnfCache{
		results: make(map[string]dnfResults),
		timeout: timeout,
		RWMutex: new(sync.RWMutex),
	}
}

// CleanCache deletes unused cache entries
// This prevents the cache from growing for longer than the timeout interval
func (d *dnfCache) CleanCache() {
	d.Lock()
	defer d.Unlock()

	// Delete expired resultCache entries
	for k := range d.results {
		if time.Since(d.results[k].expire) > d.timeout {
			delete(d.results, k)
		}
	}
}

// Get returns the package list and true if cached
// or an empty list and false if not cached or if cache is timed out
func (d *dnfCache) Get(hash string) (rpmmd.PackageList, bool) {
	d.RLock()
	defer d.RUnlock()

	result, ok := d.results[hash]
	if !ok || time.Since(result.expire) >= d.timeout {
		return rpmmd.PackageList{}, false
	}
	return result.pkgs, true
}

// Store saves the package list in the cache
func (d *dnfCache) Store(hash string, pkgs rpmmd.PackageList) {
	d.Lock()
	defer d.Unlock()
	d.results[hash] = dnfResults{expire: time.Now(), pkgs: pkgs}
}
