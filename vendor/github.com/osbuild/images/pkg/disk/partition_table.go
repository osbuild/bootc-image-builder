package disk

import (
	"fmt"
	"math/rand"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/osbuild/images/pkg/blueprint"
)

type PartitionTable struct {
	Size       uint64 // Size of the disk (in bytes).
	UUID       string // Unique identifier of the partition table (GPT only).
	Type       string // Partition table type, e.g. dos, gpt.
	Partitions []Partition

	SectorSize   uint64 // Sector size in bytes
	ExtraPadding uint64 // Extra space at the end of the partition table (sectors)
	StartOffset  uint64 // Starting offset of the first partition in the table (Mb)
}

type PartitioningMode string

const (
	// AutoLVMPartitioningMode creates a LVM layout if the filesystem
	// contains a mountpoint that's not defined in the base partition table
	// of the specified image type. In the other case, a raw layout is used.
	AutoLVMPartitioningMode PartitioningMode = "auto-lvm"

	// LVMPartitioningMode always creates an LVM layout.
	LVMPartitioningMode PartitioningMode = "lvm"

	// RawPartitioningMode always creates a raw layout.
	RawPartitioningMode PartitioningMode = "raw"

	// DefaultPartitioningMode is AutoLVMPartitioningMode and is the empty state
	DefaultPartitioningMode PartitioningMode = ""
)

func NewPartitionTable(basePT *PartitionTable, mountpoints []blueprint.FilesystemCustomization, imageSize uint64, mode PartitioningMode, requiredSizes map[string]uint64, rng *rand.Rand) (*PartitionTable, error) {
	newPT := basePT.Clone().(*PartitionTable)

	if basePT.features().LVM && mode == RawPartitioningMode {
		return nil, fmt.Errorf("raw partitioning mode set for a base partition table with LVM, this is unsupported")
	}

	// first pass: enlarge existing mountpoints and collect new ones
	newMountpoints, _ := newPT.applyCustomization(mountpoints, false)

	var ensureLVM bool
	switch mode {
	case LVMPartitioningMode:
		ensureLVM = true
	case RawPartitioningMode:
		ensureLVM = false
	case DefaultPartitioningMode, AutoLVMPartitioningMode:
		ensureLVM = len(newMountpoints) > 0
	default:
		return nil, fmt.Errorf("unsupported partitioning mode %q", mode)
	}
	if ensureLVM {
		err := newPT.ensureLVM()
		if err != nil {
			return nil, err
		}
	}

	// second pass: deal with new mountpoints and newly created ones, after switching to
	// the LVM layout, if requested, which might introduce new mount points, i.e. `/boot`
	_, err := newPT.applyCustomization(newMountpoints, true)
	if err != nil {
		return nil, err
	}

	// If no separate requiredSizes are given then we use our defaults
	if requiredSizes == nil {
		requiredSizes = map[string]uint64{
			"/":    1073741824,
			"/usr": 2147483648,
		}
	}

	if len(requiredSizes) != 0 {
		newPT.EnsureDirectorySizes(requiredSizes)
	}

	// Calculate partition table offsets and sizes
	newPT.relayout(imageSize)

	// Generate new UUIDs for filesystems and partitions
	newPT.GenerateUUIDs(rng)

	return newPT, nil
}

func (pt *PartitionTable) IsContainer() bool {
	return true
}

func (pt *PartitionTable) Clone() Entity {
	if pt == nil {
		return nil
	}

	clone := &PartitionTable{
		Size:         pt.Size,
		UUID:         pt.UUID,
		Type:         pt.Type,
		Partitions:   make([]Partition, len(pt.Partitions)),
		SectorSize:   pt.SectorSize,
		ExtraPadding: pt.ExtraPadding,
		StartOffset:  pt.StartOffset,
	}

	for idx, partition := range pt.Partitions {
		ent := partition.Clone()

		// partition.Clone() will return nil only if the partition is nil
		if ent == nil {
			panic(fmt.Sprintf("partition %d in a Partition Table is nil; this is a programming error", idx))
		}

		part, cloneOk := ent.(*Partition)
		if !cloneOk {
			panic("PartitionTable.Clone() returned an Entity that cannot be converted to *PartitionTable; this is a programming error")
		}

		clone.Partitions[idx] = *part
	}
	return clone
}

// AlignUp will align the given bytes to next aligned grain if not already
// aligned
func (pt *PartitionTable) AlignUp(size uint64) uint64 {
	grain := DefaultGrainBytes
	if size%grain == 0 {
		// already aligned: return unchanged
		return size
	}
	return ((size + grain) / grain) * grain
}

// Convert the given bytes to the number of sectors.
func (pt *PartitionTable) BytesToSectors(size uint64) uint64 {
	sectorSize := pt.SectorSize
	if sectorSize == 0 {
		sectorSize = DefaultSectorSize
	}
	return size / sectorSize
}

// Convert the given number of sectors to bytes.
func (pt *PartitionTable) SectorsToBytes(size uint64) uint64 {
	sectorSize := pt.SectorSize
	if sectorSize == 0 {
		sectorSize = DefaultSectorSize
	}
	return size * sectorSize
}

// Returns if the partition table contains a filesystem with the given
// mount point.
func (pt *PartitionTable) ContainsMountpoint(mountpoint string) bool {
	return len(entityPath(pt, mountpoint)) > 0
}

// Generate all needed UUIDs for all the partiton and filesystems
//
// Will not overwrite existing UUIDs and only generate UUIDs for
// partitions if the layout is GPT.
func (pt *PartitionTable) GenerateUUIDs(rng *rand.Rand) {
	setuuid := func(ent Entity, path []Entity) error {
		if ui, ok := ent.(UniqueEntity); ok {
			ui.GenUUID(rng)
		}
		return nil
	}
	_ = pt.ForEachEntity(setuuid)

	// if this is a MBR partition table, there is no need to generate
	// uuids for the partitions themselves
	if pt.Type != "gpt" {
		return
	}

	for idx, part := range pt.Partitions {
		if part.UUID == "" {
			pt.Partitions[idx].UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
		}
	}
}

func (pt *PartitionTable) GetItemCount() uint {
	return uint(len(pt.Partitions))
}

func (pt *PartitionTable) GetChild(n uint) Entity {
	return &pt.Partitions[n]
}

func (pt *PartitionTable) GetSize() uint64 {
	return pt.Size
}

func (pt *PartitionTable) EnsureSize(s uint64) bool {
	if s > pt.Size {
		pt.Size = s
		return true
	}
	return false
}

func (pt *PartitionTable) findDirectoryEntityPath(dir string) []Entity {
	if path := entityPath(pt, dir); path != nil {
		return path
	}

	parent := filepath.Dir(dir)
	if dir == parent {
		// invalid dir or pt has no root
		return nil
	}

	// move up the directory path and check again
	return pt.findDirectoryEntityPath(parent)
}

// EnsureDirectorySizes takes a mapping of directory paths to sizes (in bytes)
// and resizes the appropriate partitions such that they are at least the size
// of the sum of their subdirectories plus their own sizes.
// The function will panic if any of the directory paths are invalid.
func (pt *PartitionTable) EnsureDirectorySizes(dirSizeMap map[string]uint64) {

	type mntSize struct {
		entPath []Entity
		newSize uint64
	}

	// add up the required size for each directory grouped by their mountpoints
	mntSizeMap := make(map[string]*mntSize)
	for dir, size := range dirSizeMap {
		entPath := pt.findDirectoryEntityPath(dir)
		if entPath == nil {
			panic(fmt.Sprintf("EnsureDirectorySizes: invalid dir path %q", dir))
		}
		mnt := entPath[0].(Mountable)
		mountpoint := mnt.GetMountpoint()
		if _, ok := mntSizeMap[mountpoint]; !ok {
			mntSizeMap[mountpoint] = &mntSize{entPath, 0}
		}
		es := mntSizeMap[mountpoint]
		es.newSize += size
	}

	// resize all the entities in the map
	for _, es := range mntSizeMap {
		resizeEntityBranch(es.entPath, es.newSize)
	}
}

func (pt *PartitionTable) CreateMountpoint(mountpoint string, size uint64) (Entity, error) {
	filesystem := Filesystem{
		Type:         "xfs",
		Mountpoint:   mountpoint,
		FSTabOptions: "defaults",
		FSTabFreq:    0,
		FSTabPassNo:  0,
	}

	partition := Partition{
		Size:    size,
		Payload: &filesystem,
	}

	n := len(pt.Partitions)
	var maxNo int

	if pt.Type == "gpt" {
		switch mountpoint {
		case "/boot":
			partition.Type = XBootLDRPartitionGUID
		default:
			partition.Type = FilesystemDataGUID
		}
		maxNo = 128
	} else {
		maxNo = 4
	}

	if n == maxNo {
		return nil, fmt.Errorf("maximum number of partitions reached (%d)", maxNo)
	}

	pt.Partitions = append(pt.Partitions, partition)

	return &pt.Partitions[len(pt.Partitions)-1], nil
}

type EntityCallback func(e Entity, path []Entity) error

func forEachEntity(e Entity, path []Entity, cb EntityCallback) error {

	childPath := append(path, e)
	err := cb(e, childPath)

	if err != nil {
		return err
	}

	c, ok := e.(Container)
	if !ok {
		return nil
	}

	for idx := uint(0); idx < c.GetItemCount(); idx++ {
		child := c.GetChild(idx)
		err = forEachEntity(child, childPath, cb)
		if err != nil {
			return err
		}
	}

	return nil
}

// ForEachEntity runs the provided callback function on each entity in
// the PartitionTable.
func (pt *PartitionTable) ForEachEntity(cb EntityCallback) error {
	return forEachEntity(pt, []Entity{}, cb)
}

func (pt *PartitionTable) HeaderSize() uint64 {
	// always reserve one extra sector for the GPT header
	// this also ensure we have enough space for the MBR
	header := pt.SectorsToBytes(1)

	if pt.Type == "dos" {
		return header
	}

	// calculate the space we need for
	parts := len(pt.Partitions)

	// reserve a minimum of 128 partition entires
	if parts < 128 {
		parts = 128
	}

	// Assume that each partition entry is 128 bytes
	// which might not be the case if the partition
	// name exceeds 72 bytes
	header += uint64(parts * 128)

	return header
}

// Apply filesystem filesystem customization to the partiton table. If create is false
// will only apply customizations to existing partitions and return unhandled, i.e new
// ones. An error can only occur if create is set. Conversely, it will only return non
// empty list of new mountpoints if create is false.
// Does not relayout the table, i.e. a call to relayout might be needed.
func (pt *PartitionTable) applyCustomization(mountpoints []blueprint.FilesystemCustomization, create bool) ([]blueprint.FilesystemCustomization, error) {

	newMountpoints := []blueprint.FilesystemCustomization{}

	for _, mnt := range mountpoints {
		size := clampFSSize(mnt.Mountpoint, mnt.MinSize)
		if path := entityPath(pt, mnt.Mountpoint); len(path) != 0 {
			size = alignEntityBranch(path, size)
			resizeEntityBranch(path, size)
		} else {
			if !create {
				newMountpoints = append(newMountpoints, mnt)
			} else if err := pt.createFilesystem(mnt.Mountpoint, size); err != nil {
				return nil, err
			}
		}
	}

	return newMountpoints, nil
}

// Dynamically calculate and update the start point for each of the existing
// partitions. Adjusts the overall size of image to either the supplied
// value in `size` or to the sum of all partitions if that is lager.
// Will grow the root partition if there is any empty space.
// Returns the updated start point.
func (pt *PartitionTable) relayout(size uint64) uint64 {
	// always reserve one extra sector for the GPT header
	header := pt.HeaderSize()
	footer := uint64(0)

	// The GPT header is also at the end of the partition table
	if pt.Type == "gpt" {
		footer = header
	}

	start := pt.AlignUp(header)
	start += pt.StartOffset
	size = pt.AlignUp(size)

	var rootIdx = -1
	for idx := range pt.Partitions {
		partition := &pt.Partitions[idx]
		if len(entityPath(partition, "/")) != 0 {
			rootIdx = idx
			continue
		}
		partition.Start = start
		partition.Size = pt.AlignUp(partition.Size)
		start += partition.Size
	}

	if rootIdx < 0 {
		panic("no root filesystem found; this is a programming error")
	}

	root := &pt.Partitions[rootIdx]
	root.Start = start

	// add the extra padding specified in the partition table
	footer += pt.ExtraPadding

	// If the sum of all partitions is bigger then the specified size,
	// we use that instead. Grow the partition table size if needed.
	end := pt.AlignUp(root.Start + footer + root.Size)
	if end > size {
		size = end
	}

	if size > pt.Size {
		pt.Size = size
	}

	// If there is space left in the partition table, grow root
	root.Size = pt.Size - root.Start

	// Finally we shrink the last partition, i.e. the root partition,
	// to leave space for the footer, e.g. the secondary GPT header.
	root.Size -= footer

	return start
}

func (pt *PartitionTable) createFilesystem(mountpoint string, size uint64) error {
	rootPath := entityPath(pt, "/")
	if rootPath == nil {
		panic("no root mountpoint for PartitionTable")
	}

	var vc MountpointCreator
	var entity Entity
	var idx int
	for idx, entity = range rootPath {
		var ok bool
		if vc, ok = entity.(MountpointCreator); ok {
			break
		}
	}

	if vc == nil {
		panic("could not find root volume container")
	}

	newVol, err := vc.CreateMountpoint(mountpoint, 0)
	if err != nil {
		return fmt.Errorf("failed creating volume: " + err.Error())
	}
	vcPath := append([]Entity{newVol}, rootPath[idx:]...)
	size = alignEntityBranch(vcPath, size)
	resizeEntityBranch(vcPath, size)
	return nil
}

// entityPath stats at ent and searches for an Entity with a Mountpoint equal
// to the target. Returns a slice of all the Entities leading to the Mountable
// in reverse order. If no Entity has the target as a Mountpoint, returns nil.
// If a slice is returned, the last element is always the starting Entity ent
// and the first element is always a Mountable with a Mountpoint equal to the
// target.
func entityPath(ent Entity, target string) []Entity {
	switch e := ent.(type) {
	case Mountable:
		if target == e.GetMountpoint() {
			return []Entity{ent}
		}
	case Container:
		for idx := uint(0); idx < e.GetItemCount(); idx++ {
			child := e.GetChild(idx)
			path := entityPath(child, target)
			if path != nil {
				path = append(path, e)
				return path
			}
		}
	}
	return nil
}

type MountableCallback func(mnt Mountable, path []Entity) error

func forEachMountable(c Container, path []Entity, cb MountableCallback) error {
	for idx := uint(0); idx < c.GetItemCount(); idx++ {
		child := c.GetChild(idx)
		childPath := append(path, child)
		var err error
		switch ent := child.(type) {
		case Mountable:
			err = cb(ent, childPath)
		case Container:
			err = forEachMountable(ent, childPath, cb)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ForEachMountable runs the provided callback function on each Mountable in
// the PartitionTable.
func (pt *PartitionTable) ForEachMountable(cb MountableCallback) error {
	return forEachMountable(pt, []Entity{pt}, cb)
}

// FindMountable returns the Mountable entity with the given mountpoint in the
// PartitionTable. Returns nil if no Entity has the target as a Mountpoint.
func (pt *PartitionTable) FindMountable(mountpoint string) Mountable {
	path := entityPath(pt, mountpoint)

	if len(path) == 0 {
		return nil
	}
	// first path element is guaranteed to be Mountable
	return path[0].(Mountable)
}

func clampFSSize(mountpoint string, size uint64) uint64 {
	// set a minimum size of 1GB for all mountpoints
	// with the exception for '/boot' (= 500 MB)
	var minSize uint64 = 1073741824

	if mountpoint == "/boot" {
		minSize = 524288000
	}

	if minSize > size {
		return minSize
	}
	return size
}

func alignEntityBranch(path []Entity, size uint64) uint64 {
	if len(path) == 0 {
		return size
	}

	element := path[0]

	if c, ok := element.(MountpointCreator); ok {
		size = c.AlignUp(size)
	}

	return alignEntityBranch(path[1:], size)
}

// resizeEntityBranch resizes the first entity in the specified path to be at
// least the specified size and then grows every entity up the path to the
// PartitionTable accordingly.
func resizeEntityBranch(path []Entity, size uint64) {
	if len(path) == 0 {
		return
	}

	element := path[0]

	if c, ok := element.(Container); ok {
		containerSize := uint64(0)
		for idx := uint(0); idx < c.GetItemCount(); idx++ {
			if s, ok := c.GetChild(idx).(Sizeable); ok {
				containerSize += s.GetSize()
			} else {
				break
			}
		}
		if vc, ok := element.(VolumeContainer); ok {
			containerSize += vc.MetadataSize()
		}
		if containerSize > size {
			size = containerSize
		}
	}
	if sz, ok := element.(Sizeable); ok {
		if !sz.EnsureSize(size) {
			return
		}
	}
	resizeEntityBranch(path[1:], size)
}

// GenUUID generates and sets UUIDs for all Partitions in the PartitionTable if
// the layout is GPT.
func (pt *PartitionTable) GenUUID(rng *rand.Rand) {
	if pt.UUID == "" {
		pt.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}
}

// ensureLVM will ensure that the root partition is on an LVM volume, i.e. if
// it currently is not, it will wrap it in one
func (pt *PartitionTable) ensureLVM() error {

	rootPath := entityPath(pt, "/")
	if rootPath == nil {
		panic("no root mountpoint for PartitionTable")
	}

	// we need a /boot partition to boot LVM, ensure one exists
	bootPath := entityPath(pt, "/boot")
	if bootPath == nil {
		_, err := pt.CreateMountpoint("/boot", 512*1024*1024)

		if err != nil {
			return err
		}

		rootPath = entityPath(pt, "/")
	}

	parent := rootPath[1] // NB: entityPath has reversed order

	if _, ok := parent.(*LVMLogicalVolume); ok {
		return nil
	} else if part, ok := parent.(*Partition); ok {
		filesystem := part.Payload

		vg := &LVMVolumeGroup{
			Name:        "rootvg",
			Description: "created via lvm2 and osbuild",
		}

		_, err := vg.CreateLogicalVolume("root", part.Size, filesystem)
		if err != nil {
			panic(fmt.Sprintf("Could not create LV: %v", err))
		}

		part.Payload = vg

		// reset it so it will be grown later
		part.Size = 0

		if pt.Type == "gpt" {
			part.Type = LVMPartitionGUID
		} else {
			part.Type = "8e"
		}

	} else {
		return fmt.Errorf("Unsupported parent for LVM")
	}

	return nil
}

type partitionTableFeatures struct {
	LVM   bool
	Btrfs bool
	XFS   bool
	FAT   bool
	EXT4  bool
	LUKS  bool
}

// features examines all of the PartitionTable entities
// and returns a struct with flags set for each feature used
func (pt *PartitionTable) features() partitionTableFeatures {
	var ptFeatures partitionTableFeatures

	introspectPT := func(e Entity, path []Entity) error {
		switch ent := e.(type) {
		case *LVMLogicalVolume:
			ptFeatures.LVM = true
		case *Btrfs:
			ptFeatures.Btrfs = true
		case *Filesystem:
			switch ent.GetFSType() {
			case "vfat":
				ptFeatures.FAT = true
			case "btrfs":
				ptFeatures.Btrfs = true
			case "xfs":
				ptFeatures.XFS = true
			case "ext4":
				ptFeatures.EXT4 = true
			}
		case *LUKSContainer:
			ptFeatures.LUKS = true
		}
		return nil
	}
	_ = pt.ForEachEntity(introspectPT)

	return ptFeatures
}

// GetBuildPackages returns an array of packages needed to support the features used in the PartitionTable.
func (pt *PartitionTable) GetBuildPackages() []string {
	packages := []string{}

	features := pt.features()

	if features.LVM {
		packages = append(packages, "lvm2")
	}
	if features.Btrfs {
		packages = append(packages, "btrfs-progs")
	}
	if features.XFS {
		packages = append(packages, "xfsprogs")
	}
	if features.FAT {
		packages = append(packages, "dosfstools")
	}
	if features.EXT4 {
		packages = append(packages, "e2fsprogs")
	}
	if features.LUKS {
		packages = append(packages,
			"clevis",
			"clevis-luks",
			"cryptsetup",
		)
	}

	return packages
}
