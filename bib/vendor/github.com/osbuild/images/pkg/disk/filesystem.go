package disk

import (
	"math/rand"

	"github.com/google/uuid"
)

// Filesystem related functions
type Filesystem struct {
	Type string
	// ID of the filesystem, vfat doesn't use traditional UUIDs, therefore this
	// is just a string.
	UUID       string
	Label      string
	Mountpoint string
	// The fourth field of fstab(5); fs_mntops
	FSTabOptions string
	// The fifth field of fstab(5); fs_freq
	FSTabFreq uint64
	// The sixth field of fstab(5); fs_passno
	FSTabPassNo uint64
}

func (fs *Filesystem) IsContainer() bool {
	return false
}

// Clone the filesystem structure
func (fs *Filesystem) Clone() Entity {
	if fs == nil {
		return nil
	}

	return &Filesystem{
		Type:         fs.Type,
		UUID:         fs.UUID,
		Label:        fs.Label,
		Mountpoint:   fs.Mountpoint,
		FSTabOptions: fs.FSTabOptions,
		FSTabFreq:    fs.FSTabFreq,
		FSTabPassNo:  fs.FSTabPassNo,
	}
}

func (fs *Filesystem) GetMountpoint() string {
	if fs == nil {
		return ""
	}
	return fs.Mountpoint
}

func (fs *Filesystem) GetFSType() string {
	if fs == nil {
		return ""
	}
	return fs.Type
}

func (fs *Filesystem) GetFSSpec() FSSpec {
	if fs == nil {
		return FSSpec{}
	}
	return FSSpec{
		UUID:  fs.UUID,
		Label: fs.Label,
	}
}

func (fs *Filesystem) GetFSTabOptions() FSTabOptions {
	if fs == nil {
		return FSTabOptions{}
	}
	return FSTabOptions{
		MntOps: fs.FSTabOptions,
		Freq:   fs.FSTabFreq,
		PassNo: fs.FSTabPassNo,
	}
}

func (fs *Filesystem) GenUUID(rng *rand.Rand) {
	if fs.UUID == "" {
		fs.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}
}
