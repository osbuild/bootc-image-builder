package pathpolicy

import (
	"fmt"
	"path"
)

type PathPolicy struct {
	Deny  bool // explicitly do not allow this entry
	Exact bool // require and exact match, no subdirs
}

type PathPolicies = PathTrie

// Create a new PathPolicies trie from a map of path to PathPolicy
func NewPathPolicies(entries map[string]PathPolicy) *PathPolicies {

	noType := make(map[string]interface{}, len(entries))

	for k, v := range entries {
		noType[k] = v
	}

	return NewPathTrieFromMap(noType)
}

// Check a given path against the PathPolicies
func (pol *PathPolicies) Check(fsPath string) error {

	// Quickly check we have a path and it is absolute
	if fsPath == "" || fsPath[0] != '/' {
		return fmt.Errorf("path must be absolute")
	}

	// ensure that only clean paths are valid
	if fsPath != path.Clean(fsPath) {
		return fmt.Errorf("path must be canonical")
	}

	node, left := pol.Lookup(fsPath)
	policy, ok := node.Payload.(PathPolicy)
	if !ok {
		panic("programming error: invalid path trie payload")
	}

	// 1) path is explicitly not allowed or
	// 2) a subpath was match but an explicit match is required
	if policy.Deny || (policy.Exact && len(left) > 0) {
		return fmt.Errorf("path '%s ' is not allowed", fsPath)
	}

	// exact match or recursive path allowed
	return nil
}
