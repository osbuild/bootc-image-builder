package blueprint

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/pathpolicy"
	"github.com/osbuild/images/pkg/customizations/fsnode"
)

// validateModeString checks that the given string is a valid mode octal number
func validateModeString(mode string) error {
	// Check that the mode string matches the octal format regular expression.
	// The leading is optional.
	if regexp.MustCompile(`^[0]{0,1}[0-7]{3}$`).MatchString(mode) {
		return nil
	}
	return fmt.Errorf("invalid mode %s: must be an octal number", mode)
}

// DirectoryCustomization represents a directory to be created in the image
type DirectoryCustomization struct {
	// Absolute path to the directory
	Path string `json:"path" toml:"path"`
	// Owner of the directory specified as a string (user name), int64 (UID) or nil
	User interface{} `json:"user,omitempty" toml:"user,omitempty"`
	// Owner of the directory specified as a string (group name), int64 (UID) or nil
	Group interface{} `json:"group,omitempty" toml:"group,omitempty"`
	// Permissions of the directory specified as an octal number
	Mode string `json:"mode,omitempty" toml:"mode,omitempty"`
	// EnsureParents ensures that all parent directories of the directory exist
	EnsureParents bool `json:"ensure_parents,omitempty" toml:"ensure_parents,omitempty"`
}

// Custom TOML unmarshalling for DirectoryCustomization with validation
func (d *DirectoryCustomization) UnmarshalTOML(data interface{}) error {
	var dir DirectoryCustomization

	dataMap, _ := data.(map[string]interface{})

	switch path := dataMap["path"].(type) {
	case string:
		dir.Path = path
	default:
		return fmt.Errorf("UnmarshalTOML: path must be a string")
	}

	switch user := dataMap["user"].(type) {
	case string:
		dir.User = user
	case int64:
		dir.User = user
	case nil:
		break
	default:
		return fmt.Errorf("UnmarshalTOML: user must be a string or an integer, got %T", user)
	}

	switch group := dataMap["group"].(type) {
	case string:
		dir.Group = group
	case int64:
		dir.Group = group
	case nil:
		break
	default:
		return fmt.Errorf("UnmarshalTOML: group must be a string or an integer")
	}

	switch mode := dataMap["mode"].(type) {
	case string:
		dir.Mode = mode
	case nil:
		break
	default:
		return fmt.Errorf("UnmarshalTOML: mode must be a string")
	}

	switch ensureParents := dataMap["ensure_parents"].(type) {
	case bool:
		dir.EnsureParents = ensureParents
	case nil:
		break
	default:
		return fmt.Errorf("UnmarshalTOML: ensure_parents must be a bool")
	}

	// try converting to fsnode.Directory to validate all values
	_, err := dir.ToFsNodeDirectory()
	if err != nil {
		return err
	}

	*d = dir
	return nil
}

// Custom JSON unmarshalling for DirectoryCustomization with validation
func (d *DirectoryCustomization) UnmarshalJSON(data []byte) error {
	type directoryCustomization DirectoryCustomization

	var dirPrivate directoryCustomization
	if err := json.Unmarshal(data, &dirPrivate); err != nil {
		return err
	}

	dir := DirectoryCustomization(dirPrivate)
	if uid, ok := dir.User.(float64); ok {
		// check if uid can be converted to int64
		if uid != float64(int64(uid)) {
			return fmt.Errorf("invalid user %f: must be an integer", uid)
		}
		dir.User = int64(uid)
	}
	if gid, ok := dir.Group.(float64); ok {
		// check if gid can be converted to int64
		if gid != float64(int64(gid)) {
			return fmt.Errorf("invalid group %f: must be an integer", gid)
		}
		dir.Group = int64(gid)
	}
	// try converting to fsnode.Directory to validate all values
	_, err := dir.ToFsNodeDirectory()
	if err != nil {
		return err
	}

	*d = dir
	return nil
}

// ToFsNodeDirectory converts the DirectoryCustomization to an fsnode.Directory
func (d DirectoryCustomization) ToFsNodeDirectory() (*fsnode.Directory, error) {
	var mode *os.FileMode
	if d.Mode != "" {
		err := validateModeString(d.Mode)
		if err != nil {
			return nil, err
		}
		modeNum, err := strconv.ParseUint(d.Mode, 8, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid mode %s: %v", d.Mode, err)
		}
		mode = common.ToPtr(os.FileMode(modeNum))
	}

	return fsnode.NewDirectory(d.Path, mode, d.User, d.Group, d.EnsureParents)
}

// DirectoryCustomizationsToFsNodeDirectories converts a slice of DirectoryCustomizations
// to a slice of fsnode.Directories
func DirectoryCustomizationsToFsNodeDirectories(dirs []DirectoryCustomization) ([]*fsnode.Directory, error) {
	if len(dirs) == 0 {
		return nil, nil
	}

	var fsDirs []*fsnode.Directory
	var errors []error
	for _, dir := range dirs {
		fsDir, err := dir.ToFsNodeDirectory()
		if err != nil {
			errors = append(errors, err)
		}
		fsDirs = append(fsDirs, fsDir)
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("invalid directory customizations: %v", errors)
	}

	return fsDirs, nil
}

// FileCustomization represents a file to be created in the image
type FileCustomization struct {
	// Absolute path to the file
	Path string `json:"path" toml:"path"`
	// Owner of the directory specified as a string (user name), int64 (UID) or nil
	User interface{} `json:"user,omitempty" toml:"user,omitempty"`
	// Owner of the directory specified as a string (group name), int64 (UID) or nil
	Group interface{} `json:"group,omitempty" toml:"group,omitempty"`
	// Permissions of the file specified as an octal number
	Mode string `json:"mode,omitempty" toml:"mode,omitempty"`
	// Data is the file content in plain text
	Data string `json:"data,omitempty" toml:"data,omitempty"`
}

// Custom TOML unmarshalling for FileCustomization with validation
func (f *FileCustomization) UnmarshalTOML(data interface{}) error {
	var file FileCustomization

	dataMap, _ := data.(map[string]interface{})

	switch path := dataMap["path"].(type) {
	case string:
		file.Path = path
	default:
		return fmt.Errorf("UnmarshalTOML: path must be a string")
	}

	switch user := dataMap["user"].(type) {
	case string:
		file.User = user
	case int64:
		file.User = user
	case nil:
		break
	default:
		return fmt.Errorf("UnmarshalTOML: user must be a string or an integer")
	}

	switch group := dataMap["group"].(type) {
	case string:
		file.Group = group
	case int64:
		file.Group = group
	case nil:
		break
	default:
		return fmt.Errorf("UnmarshalTOML: group must be a string or an integer")
	}

	switch mode := dataMap["mode"].(type) {
	case string:
		file.Mode = mode
	case nil:
		break
	default:
		return fmt.Errorf("UnmarshalTOML: mode must be a string")
	}

	switch data := dataMap["data"].(type) {
	case string:
		file.Data = data
	case nil:
		break
	default:
		return fmt.Errorf("UnmarshalTOML: data must be a string")
	}

	// try converting to fsnode.File to validate all values
	_, err := file.ToFsNodeFile()
	if err != nil {
		return err
	}

	*f = file
	return nil
}

// Custom JSON unmarshalling for FileCustomization with validation
func (f *FileCustomization) UnmarshalJSON(data []byte) error {
	type fileCustomization FileCustomization

	var filePrivate fileCustomization
	if err := json.Unmarshal(data, &filePrivate); err != nil {
		return err
	}

	file := FileCustomization(filePrivate)
	if uid, ok := file.User.(float64); ok {
		// check if uid can be converted to int64
		if uid != float64(int64(uid)) {
			return fmt.Errorf("invalid user %f: must be an integer", uid)
		}
		file.User = int64(uid)
	}
	if gid, ok := file.Group.(float64); ok {
		// check if gid can be converted to int64
		if gid != float64(int64(gid)) {
			return fmt.Errorf("invalid group %f: must be an integer", gid)
		}
		file.Group = int64(gid)
	}
	// try converting to fsnode.File to validate all values
	_, err := file.ToFsNodeFile()
	if err != nil {
		return err
	}

	*f = file
	return nil
}

// ToFsNodeFile converts the FileCustomization to an fsnode.File
func (f FileCustomization) ToFsNodeFile() (*fsnode.File, error) {
	var data []byte
	if f.Data != "" {
		data = []byte(f.Data)
	}

	var mode *os.FileMode
	if f.Mode != "" {
		err := validateModeString(f.Mode)
		if err != nil {
			return nil, err
		}
		modeNum, err := strconv.ParseUint(f.Mode, 8, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid mode %s: %v", f.Mode, err)
		}
		mode = common.ToPtr(os.FileMode(modeNum))
	}

	return fsnode.NewFile(f.Path, mode, f.User, f.Group, data)
}

// FileCustomizationsToFsNodeFiles converts a slice of FileCustomization to a slice of *fsnode.File
func FileCustomizationsToFsNodeFiles(files []FileCustomization) ([]*fsnode.File, error) {
	if len(files) == 0 {
		return nil, nil
	}

	var fsFiles []*fsnode.File
	var errors []error
	for _, file := range files {
		fsFile, err := file.ToFsNodeFile()
		if err != nil {
			errors = append(errors, err)
		}
		fsFiles = append(fsFiles, fsFile)
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("invalid file customizations: %v", errors)
	}

	return fsFiles, nil
}

// ValidateDirFileCustomizations validates the given Directory and File customizations.
// If the customizations are invalid, an error is returned. Otherwise, nil is returned.
//
// It currently ensures that:
// - No file path is a prefix of another file or directory path
// - There are no duplicate file or directory paths in the customizations
func ValidateDirFileCustomizations(dirs []DirectoryCustomization, files []FileCustomization) error {
	fsNodesMap := make(map[string]interface{}, len(dirs)+len(files))
	nodesPaths := make([]string, 0, len(dirs)+len(files))

	// First check for duplicate paths
	duplicatePaths := make([]string, 0)
	for _, dir := range dirs {
		if _, ok := fsNodesMap[dir.Path]; ok {
			duplicatePaths = append(duplicatePaths, dir.Path)
		}
		fsNodesMap[dir.Path] = dir
		nodesPaths = append(nodesPaths, dir.Path)
	}

	for _, file := range files {
		if _, ok := fsNodesMap[file.Path]; ok {
			duplicatePaths = append(duplicatePaths, file.Path)
		}
		fsNodesMap[file.Path] = file
		nodesPaths = append(nodesPaths, file.Path)
	}

	// There is no point in continuing if there are duplicate paths,
	// since the fsNodesMap will not be valid.
	if len(duplicatePaths) > 0 {
		return fmt.Errorf("duplicate files / directory customization paths: %v", duplicatePaths)
	}

	invalidFSNodes := make([]string, 0)
	checkedPaths := make(map[string]bool)
	// Sort the paths so that we always check the longest paths first. This
	// ensures that we don't check a parent path before we check the child
	// path. Reverse sort the slice based on directory depth.
	sort.Slice(nodesPaths, func(i, j int) bool {
		return strings.Count(nodesPaths[i], "/") > strings.Count(nodesPaths[j], "/")
	})

	for _, nodePath := range nodesPaths {
		// Skip paths that we have already checked
		if checkedPaths[nodePath] {
			continue
		}

		// Check all parent paths of the current path. If any of them have
		// already been checked, then we do not need to check them again.
		// This is because we always check the longest paths first. If a parent
		// path exists in the filesystem nodes map and it is a File,
		// then it is an error because it is a parent of a Directory or File.
		// Parent paths can be only Directories.
		parentPath := nodePath
		for {
			parentPath = path.Dir(parentPath)

			// "." is returned only when the path is relative and we reached
			// the root directory. This should never happen because File
			// and Directory customization paths are validated as part of
			// the unmarshalling process from JSON and TOML.
			if parentPath == "." {
				panic("filesystem node has relative path set.")
			}

			if parentPath == "/" {
				break
			}

			if checkedPaths[parentPath] {
				break
			}

			// If the node is not a Directory, then it is an error because
			// it is a parent of a Directory or File.
			if node, ok := fsNodesMap[parentPath]; ok {
				switch node.(type) {
				case DirectoryCustomization:
					break
				case FileCustomization:
					invalidFSNodes = append(invalidFSNodes, nodePath)
				default:
					panic(fmt.Sprintf("unexpected filesystem node customization type: %T", node))
				}
			}

			checkedPaths[parentPath] = true
		}

		checkedPaths[nodePath] = true
	}

	if len(invalidFSNodes) > 0 {
		return fmt.Errorf("the following filesystem nodes are parents of another node and are not directories: %s", invalidFSNodes)
	}

	return nil
}

// CheckFileCustomizationsPolicy checks if the given File customizations are allowed by the path policy.
// If any of the customizations are not allowed by the path policy, an error is returned. Otherwise, nil is returned.
func CheckFileCustomizationsPolicy(files []FileCustomization, pathPolicy *pathpolicy.PathPolicies) error {
	var invalidPaths []string
	for _, file := range files {
		if err := pathPolicy.Check(file.Path); err != nil {
			invalidPaths = append(invalidPaths, file.Path)
		}
	}

	if len(invalidPaths) > 0 {
		return fmt.Errorf("the following custom files are not allowed: %+q", invalidPaths)
	}

	return nil
}

// CheckDirectoryCustomizationsPolicy checks if the given Directory customizations are allowed by the path policy.
// If any of the customizations are not allowed by the path policy, an error is returned. Otherwise, nil is returned.
func CheckDirectoryCustomizationsPolicy(dirs []DirectoryCustomization, pathPolicy *pathpolicy.PathPolicies) error {
	var invalidPaths []string
	for _, dir := range dirs {
		if err := pathPolicy.Check(dir.Path); err != nil {
			invalidPaths = append(invalidPaths, dir.Path)
		}
	}

	if len(invalidPaths) > 0 {
		return fmt.Errorf("the following custom directories are not allowed: %+q", invalidPaths)
	}

	return nil
}
