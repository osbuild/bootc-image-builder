package fsnode

import "os"

type Directory struct {
	baseFsNode
	ensureParentDirs bool
}

func (d *Directory) IsDir() bool {
	return true
}

func (d *Directory) EnsureParentDirs() bool {
	if d == nil {
		return false
	}
	return d.ensureParentDirs
}

// NewDirectory creates a new directory with the given path, mode, user and group.
// user and group can be either a string (user name/group name), an int64 (UID/GID) or nil.
func NewDirectory(path string, mode *os.FileMode, user interface{}, group interface{}, ensureParentDirs bool) (*Directory, error) {
	baseNode, err := newBaseFsNode(path, mode, user, group)

	if err != nil {
		return nil, err
	}

	return &Directory{
		baseFsNode:       *baseNode,
		ensureParentDirs: ensureParentDirs,
	}, nil
}
