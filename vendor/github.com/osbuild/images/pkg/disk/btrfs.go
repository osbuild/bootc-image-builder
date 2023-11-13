package disk

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/google/uuid"
)

type Btrfs struct {
	UUID       string
	Label      string
	Mountpoint string
	Subvolumes []BtrfsSubvolume
}

func (b *Btrfs) IsContainer() bool {
	return true
}

func (b *Btrfs) Clone() Entity {
	if b == nil {
		return nil
	}

	clone := &Btrfs{
		UUID:       b.UUID,
		Label:      b.Label,
		Mountpoint: b.Mountpoint,
		Subvolumes: make([]BtrfsSubvolume, len(b.Subvolumes)),
	}

	for idx, subvol := range b.Subvolumes {
		entClone := subvol.Clone()
		svClone, cloneOk := entClone.(*BtrfsSubvolume)
		if !cloneOk {
			panic("BtrfsSubvolume.Clone() returned an Entity that cannot be converted to *BtrfsSubvolume; this is a programming error")
		}
		clone.Subvolumes[idx] = *svClone
	}

	return clone
}

func (b *Btrfs) GetItemCount() uint {
	return uint(len(b.Subvolumes))
}

func (b *Btrfs) GetChild(n uint) Entity {
	return &b.Subvolumes[n]
}
func (b *Btrfs) CreateMountpoint(mountpoint string, size uint64) (Entity, error) {
	name := mountpoint
	if name == "/" {
		name = "root"
	}
	subvolume := BtrfsSubvolume{
		Size:       size,
		Mountpoint: mountpoint,
		GroupID:    0,
		UUID:       b.UUID, // subvolumes inherit UUID of main volume
		Name:       name,
	}

	b.Subvolumes = append(b.Subvolumes, subvolume)
	return &b.Subvolumes[len(b.Subvolumes)-1], nil
}

func (b *Btrfs) AlignUp(size uint64) uint64 {
	return size // No extra alignment necessary for subvolumes
}

func (b *Btrfs) GenUUID(rng *rand.Rand) {
	if b.UUID == "" {
		b.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}
}

type BtrfsSubvolume struct {
	Name       string
	Size       uint64
	Mountpoint string
	GroupID    uint64

	MntOps string

	// UUID of the parent volume
	UUID string
}

func (subvol *BtrfsSubvolume) IsContainer() bool {
	return false
}

func (bs *BtrfsSubvolume) Clone() Entity {
	if bs == nil {
		return nil
	}

	return &BtrfsSubvolume{
		Name:       bs.Name,
		Size:       bs.Size,
		Mountpoint: bs.Mountpoint,
		GroupID:    bs.GroupID,
		MntOps:     bs.MntOps,
		UUID:       bs.UUID,
	}
}

func (bs *BtrfsSubvolume) GetSize() uint64 {
	if bs == nil {
		return 0
	}
	return bs.Size
}

func (bs *BtrfsSubvolume) EnsureSize(s uint64) bool {
	if s > bs.Size {
		bs.Size = s
		return true
	}
	return false
}

func (bs *BtrfsSubvolume) GetMountpoint() string {
	if bs == nil {
		return ""
	}
	return bs.Mountpoint
}

func (bs *BtrfsSubvolume) GetFSType() string {
	return "btrfs"
}

func (bs *BtrfsSubvolume) GetFSSpec() FSSpec {
	if bs == nil {
		return FSSpec{}
	}
	return FSSpec{
		UUID:  bs.UUID,
		Label: bs.Name,
	}
}

func (bs *BtrfsSubvolume) GetFSTabOptions() FSTabOptions {
	if bs == nil {
		return FSTabOptions{}
	}

	ops := strings.Join([]string{bs.MntOps, fmt.Sprintf("subvol=%s", bs.Name)}, ",")
	return FSTabOptions{
		MntOps: ops,
		Freq:   0,
		PassNo: 0,
	}
}
