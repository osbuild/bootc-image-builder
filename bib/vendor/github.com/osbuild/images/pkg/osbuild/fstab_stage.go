package osbuild

import (
	"fmt"
	"sort"

	"github.com/osbuild/images/pkg/disk"
)

// The FSTabStageOptions describe the content of the /etc/fstab file.
//
// The structure of the options follows the format of /etc/fstab, except
// that filesystem must be identified by their UUID and ommitted fields
// are set to their defaults (if possible).
type FSTabStageOptions struct {
	FileSystems []*FSTabEntry `json:"filesystems"`

	OSTree *OSTreeFstab `json:"ostree,omitempty"`
}

func (FSTabStageOptions) isStageOptions() {}

type OSTreeFstab struct {
	Deployment OSTreeDeployment `json:"deployment"`
}

// NewFSTabStage creates a now FSTabStage object
func NewFSTabStage(options *FSTabStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.fstab",
		Options: options,
	}
}

// An FSTabEntry represents one line in /etc/fstab. With the one exception
// that the the spec field must be represented as an UUID.
type FSTabEntry struct {
	UUID    string `json:"uuid,omitempty"`
	Label   string `json:"label,omitempty"`
	VFSType string `json:"vfs_type"`
	Path    string `json:"path,omitempty"`
	Options string `json:"options,omitempty"`
	Freq    uint64 `json:"freq,omitempty"`
	PassNo  uint64 `json:"passno,omitempty"`
}

// AddFilesystem adds one entry to and FSTabStageOptions object.
func (options *FSTabStageOptions) AddFilesystem(id string, vfsType string, path string, opts string, freq uint64, passNo uint64) {
	options.FileSystems = append(options.FileSystems, &FSTabEntry{
		UUID:    id,
		VFSType: vfsType,
		Path:    path,
		Options: opts,
		Freq:    freq,
		PassNo:  passNo,
	})
}

func NewFSTabStageOptions(pt *disk.PartitionTable) *FSTabStageOptions {
	var options FSTabStageOptions
	genOption := func(mnt disk.Mountable, path []disk.Entity) error {
		fsSpec := mnt.GetFSSpec()
		fsOptions := mnt.GetFSTabOptions()
		options.AddFilesystem(fsSpec.UUID, mnt.GetFSType(), mnt.GetMountpoint(), fsOptions.MntOps, fsOptions.Freq, fsOptions.PassNo)
		return nil
	}

	key := func(fs *FSTabEntry) string {
		return fmt.Sprintf("%d%s", fs.PassNo, fs.Path)
	}

	_ = pt.ForEachMountable(genOption) // genOption always returns nil
	// sort the entries by PassNo to maintain backward compatibility
	sort.Slice(options.FileSystems, func(i, j int) bool {
		return key(options.FileSystems[i]) < key(options.FileSystems[j])
	})
	return &options
}
