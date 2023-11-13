package fsnode

import (
	"fmt"
	"os"
	"path"
	"regexp"
)

const usernameRegex = `^[A-Za-z0-9_.][A-Za-z0-9_.-]{0,31}$`
const groupnameRegex = `^[A-Za-z0-9_][A-Za-z0-9_-]{0,31}$`

type FsNode interface {
	Path() string
	Mode() *os.FileMode
	// User can return either a string (user name/group name), an int64 (UID/GID) or nil
	User() interface{}
	// Group can return either a string (user name/group name), an int64 (UID/GID) or nil
	Group() interface{}
	IsDir() bool
}

type baseFsNode struct {
	path  string
	mode  *os.FileMode
	user  interface{}
	group interface{}
}

func (f *baseFsNode) Path() string {
	if f == nil {
		return ""
	}
	return f.path
}

func (f *baseFsNode) Mode() *os.FileMode {
	if f == nil {
		return nil
	}
	return f.mode
}

// User can return either a string (user name) or an int64 (UID)
func (f *baseFsNode) User() interface{} {
	if f == nil {
		return nil
	}
	return f.user
}

// Group can return either a string (group name) or an int64 (GID)
func (f *baseFsNode) Group() interface{} {
	if f == nil {
		return nil
	}
	return f.group
}

func newBaseFsNode(path string, mode *os.FileMode, user interface{}, group interface{}) (*baseFsNode, error) {
	node := &baseFsNode{
		path:  path,
		mode:  mode,
		user:  user,
		group: group,
	}

	err := node.validate()
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (f *baseFsNode) validate() error {
	// Check that the path is valid
	if f.path == "" {
		return fmt.Errorf("path must not be empty")
	}
	if f.path[0] != '/' {
		return fmt.Errorf("path must be absolute")
	}
	if f.path[len(f.path)-1] == '/' {
		return fmt.Errorf("path must not end with a slash")
	}
	if f.path != path.Clean(f.path) {
		return fmt.Errorf("path must be canonical")
	}

	// Check that the mode is valid
	if f.mode != nil && *f.mode&os.ModeType != 0 {
		return fmt.Errorf("mode must not contain file type bits")
	}

	// Check that the user and group are valid
	switch user := f.user.(type) {
	case string:
		nameRegex := regexp.MustCompile(usernameRegex)
		if !nameRegex.MatchString(user) {
			return fmt.Errorf("user name %q doesn't conform to validating regex (%s)", user, nameRegex.String())
		}
	case int64:
		if user < 0 {
			return fmt.Errorf("user ID must be non-negative")
		}
	case nil:
		// user is not set
	default:
		return fmt.Errorf("user must be either a string or an int64, got %T", user)
	}

	switch group := f.group.(type) {
	case string:
		nameRegex := regexp.MustCompile(groupnameRegex)
		if !nameRegex.MatchString(group) {
			return fmt.Errorf("group name %q doesn't conform to validating regex (%s)", group, nameRegex.String())
		}
	case int64:
		if group < 0 {
			return fmt.Errorf("group ID must be non-negative")
		}
	case nil:
		// group is not set
	default:
		return fmt.Errorf("group must be either a string or an int64, got %T", group)
	}

	return nil
}

func (f *baseFsNode) IsDir() bool {
	panic("IsDir() called on baseFsNode")
}
