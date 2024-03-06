// Disk package contains abstract data-types to define disk-related entities.
//
// The disk package is a collection of interfaces and structs that can be used
// to represent an disk image with its layout. Various concrete types, such as
// PartitionTable, Partition and Filesystem types are defined to model a given
// disk layout. These implement a collection of interfaces that can be used to
// navigate and operate on the various possible combinations of entities in a
// generic way. The entity data model is very generic so that it can represent
// all possible layouts, which can be arbitrarily complex, since technologies
// like logical volume management, LUKS2 container and file systems, that can
// have sub-volumes, allow for complex layouts.
// Entity and Container are the two main interfaces that are used to model the
// tree structure of a disk image layout. The other entity interfaces, such as
// Sizeable and Mountable, then describe various properties and capabilities
// of a given entity.

package disk

import (
	"encoding/hex"
	"io"
	"math/rand"

	"github.com/google/uuid"
)

const (
	// Default sector size in bytes
	DefaultSectorSize = 512

	DefaultGrainBytes = uint64(1048576) // 1 MiB

	// UUIDs
	BIOSBootPartitionGUID = "21686148-6449-6E6F-744E-656564454649"
	BIOSBootPartitionUUID = "FAC7F1FB-3E8D-4137-A512-961DE09A5549"

	FilesystemDataGUID = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
	FilesystemDataUUID = "CB07C243-BC44-4717-853E-28852021225B"

	EFISystemPartitionGUID = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
	EFISystemPartitionUUID = "68B2905B-DF3E-4FB3-80FA-49D1E773AA33"
	EFIFilesystemUUID      = "7B77-95E7"

	LVMPartitionGUID = "E6D6D379-F507-44C2-A23C-238F2A3DF928"
	PRePartitionGUID = "9E1A2D38-C612-4316-AA26-8B49521E5A8B"

	RootPartitionUUID = "6264D520-3FB9-423F-8AB8-7A0A8E3D3562"

	// Extended Boot Loader Partition
	XBootLDRPartitionGUID = "BC13C2FF-59E6-4262-A352-B275FD6F7172"
)

// Entity is the base interface for all disk-related entities.
type Entity interface {
	// IsContainer indicates if the implementing type can
	// contain any other entities.
	IsContainer() bool

	// Clone returns a deep copy of the entity.
	Clone() Entity
}

// Container is the interface for entities that can contain other entities.
// Together with the base Entity interface this allows to model a generic
// entity tree of theoretically arbitrary depth and width.
type Container interface {
	Entity

	// GetItemCount returns the number of actual child entities.
	GetItemCount() uint

	// GetChild returns the child entity at the given index.
	GetChild(n uint) Entity
}

// Sizeable is implemented by entities that carry size information.
type Sizeable interface {
	// EnsureSize will resize the entity to the given size in case
	// it is currently smaller. Returns if the size was changed.
	EnsureSize(size uint64) bool

	// GetSize returns the size of the entity in bytes.
	GetSize() uint64
}

// A Mountable entity is an entity that can be mounted.
type Mountable interface {

	// GetMountPoint returns the path of the mount point.
	GetMountpoint() string

	// GetFSType returns the file system type, e.g. 'xfs'.
	GetFSType() string

	// GetFSSpec returns the file system spec information.
	GetFSSpec() FSSpec

	// GetFSTabOptions returns options for mounting the entity.
	GetFSTabOptions() FSTabOptions
}

// A MountpointCreator is a container that is able to create new volumes.
//
// CreateMountpoint creates a new mountpoint with the given size and
// returns the entity that represents the new mountpoint.
type MountpointCreator interface {
	CreateMountpoint(mountpoint string, size uint64) (Entity, error)

	// AlignUp will align the given bytes according to the
	// requirements of the container type.
	AlignUp(size uint64) uint64
}

// A UniqueEntity is an entity that can be uniquely identified via a UUID.
//
// GenUUID generates a UUID for the entity if it does not yet have one.
type UniqueEntity interface {
	Entity
	GenUUID(rng *rand.Rand)
}

// VolumeContainer is a specific container that contains volume entities
type VolumeContainer interface {

	// MetadataSize returns the size of the container's metadata (in
	// bytes), i.e. the storage space that needs to be reserved for
	// the container itself, in contrast to the data it contains.
	MetadataSize() uint64
}

// FSSpec for a filesystem (UUID and Label); the first field of fstab(5)
type FSSpec struct {
	UUID  string
	Label string
}

type FSTabOptions struct {
	// The fourth field of fstab(5); fs_mntops
	MntOps string
	// The fifth field of fstab(5); fs_freq
	Freq uint64
	// The sixth field of fstab(5); fs_passno
	PassNo uint64
}

// uuid generator helpers

// GeneratesnewRandomUUIDFromReader generates a new random UUID (version
// 4 using) via the given random number generator.
func newRandomUUIDFromReader(r io.Reader) (uuid.UUID, error) {
	var id uuid.UUID
	_, err := io.ReadFull(r, id[:])
	if err != nil {
		return uuid.Nil, err
	}
	id[6] = (id[6] & 0x0f) | 0x40 // Version 4
	id[8] = (id[8] & 0x3f) | 0x80 // Variant is 10
	return id, nil
}

// NewVolIDFromRand creates a random 32 bit hex string to use as a
// volume ID for FAT filesystems
func NewVolIDFromRand(r *rand.Rand) string {
	volid := make([]byte, 4)
	len, _ := r.Read(volid)
	if len != 4 {
		panic("expected four random bytes")
	}
	return hex.EncodeToString(volid)
}
