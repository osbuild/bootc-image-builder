package fsnode

import (
	"os"
)

type File struct {
	baseFsNode
	data []byte
}

func (f *File) IsDir() bool {
	return false
}

func (f *File) Data() []byte {
	if f == nil {
		return nil
	}
	return f.data
}

// NewFile creates a new file with the given path, data, mode, user and group.
// user and group can be either a string (user name/group name), an int64 (UID/GID) or nil.
func NewFile(path string, mode *os.FileMode, user interface{}, group interface{}, data []byte) (*File, error) {
	baseNode, err := newBaseFsNode(path, mode, user, group)

	if err != nil {
		return nil, err
	}

	return &File{
		baseFsNode: *baseNode,
		data:       data,
	}, nil
}
