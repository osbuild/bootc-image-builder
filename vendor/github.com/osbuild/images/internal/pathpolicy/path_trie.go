package pathpolicy

import (
	"sort"
	"strings"
)

// splits the path into its individual components. Retruns the
// empty list if the path is just the absolute root, i.e. "/".
func pathTrieSplitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}

	return strings.Split(path, "/")
}

type PathTrie struct {
	Name    []string
	Paths   []*PathTrie
	Payload interface{}
}

// match checks if the given trie is a prefix of path
func (trie *PathTrie) match(path []string) bool {
	if len(trie.Name) > len(path) {
		return false
	}

	for i := range trie.Name {
		if path[i] != trie.Name[i] {
			return false
		}
	}

	return true
}

func (trie *PathTrie) get(path []string) (*PathTrie, []string) {
	if len(path) < 1 {
		panic("programming error: expected root node")
	}

	var node *PathTrie
	for i := range trie.Paths {
		if trie.Paths[i].match(path) {
			node = trie.Paths[i]
			break
		}
	}

	// no subpath match, we are the best match
	if node == nil {
		return trie, path
	}

	// node, or one of its sub-nodes, is a match
	prefix := len(node.Name)

	// the node is a perfect match, return it
	if len(path) == prefix {
		return node, nil
	}

	// check if any sub-path's of node match
	return node.get(path[prefix:])
}

func (trie *PathTrie) add(path []string) *PathTrie {
	node := &PathTrie{Name: path}

	if trie.Paths == nil {
		trie.Paths = make([]*PathTrie, 0, 1)
	}

	trie.Paths = append(trie.Paths, node)

	return node
}

// Construct a new trie from a map of paths to their payloads.
// Returns the root node of the trie.
func NewPathTrieFromMap(entries map[string]interface{}) *PathTrie {
	root := &PathTrie{Name: []string{}}

	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		node, left := root.Lookup(k)

		if len(left) > 0 {
			node = node.add(left)
		}

		node.Payload = entries[k]
	}

	return root
}

// Lookup returns the node that is the prefix of path and
// the unmatched path segment. Must be called on the root
// trie node.
func (root *PathTrie) Lookup(path string) (*PathTrie, []string) {

	if len(root.Name) != 0 {
		panic("programming error: lookup on non-root trie node")
	}

	elements := pathTrieSplitPath(path)

	if len(elements) == 0 {
		return root, elements
	}

	return root.get(elements)
}
