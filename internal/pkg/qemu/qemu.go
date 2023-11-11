// Copyright 2019 Red Hat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// qemu.go is a Go interface to running `qemu` as a subprocess.
//
// Why not libvirt?
// Two main reasons.  First, we really do want to use qemu, and not
// something else.  We rely on qemu features/APIs and there's a general
// assumption that the qemu process is local (e.g. we expose 9p/virtiofs filesystem
// sharing).  Second, libvirt runs as a daemon, but we want the
// VMs "lifecycle bound" to their creating process (e.g. kola),
// so that e.g. Ctrl-C (SIGINT) kills both reliably.
//
// Other related projects (as a reference to share ideas if not code)
// https://github.com/google/syzkaller/blob/3e84253bf41d63c55f92679b1aab9102f2f4949a/vm/qemu/qemu.go
// https://github.com/intel/govmm

package qemu

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	coreosarch "github.com/coreos/stream-metadata-go/arch"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/cgwalters/osbuildbootc/internal/pkg/nproc"
	"github.com/cgwalters/osbuildbootc/internal/pkg/retry"
)

// HostForwardPort contains details about port-forwarding for the VM.
type HostForwardPort struct {
	Service   string
	HostPort  int
	GuestPort int
}

// Disk holds the details of a virtual disk.
type Disk struct {
	Size          string   // disk image size in bytes, optional suffixes "K", "M", "G", "T" allowed.
	BackingFile   string   // raw disk image to use.
	BackingFormat string   // qcow2, raw, etc.  If unspecified will be autodetected.
	Channel       string   // virtio (default), nvme
	DeviceOpts    []string // extra options to pass to qemu -device. "serial=XXXX" makes disks show up as /dev/disk/by-id/virtio-<serial>
	DriveOpts     []string // extra options to pass to -drive
	SectorSize    int      // if not 0, override disk sector size
	NbdDisk       bool     // if true, the disks should be presented over nbd:unix socket
	MultiPathDisk bool     // if true, present multiple paths

	attachEndPoint string    // qemuPath to attach to
	dstFileName    string    // the prepared file
	nbdServCmd     *exec.Cmd // command to serve the disk
}

// ParseDiskSpec converts a disk specification into a Disk. The format is:
// <size>[:<opt1>,<opt2>,...], like ["5G:channel=nvme"]
func ParseDiskSpec(spec string) (int64, map[string]string, error) {
	diskmap := map[string]string{}
	split := strings.Split(spec, ":")
	if split[0] == "" || (!strings.HasSuffix(split[0], "G")) {
		return 0, nil, fmt.Errorf("invalid size opt %s", spec)
	}
	var disksize string
	if len(split) == 1 {
		disksize = split[0]
	} else if len(split) == 2 {
		disksize = split[0]
		for _, opt := range strings.Split(split[1], ",") {
			kvsplit := strings.SplitN(opt, "=", 2)
			if len(kvsplit) == 0 {
				return 0, nil, fmt.Errorf("invalid empty option found in spec %q", spec)
			} else if len(kvsplit) == 1 {
				diskmap[opt] = ""
			} else {
				diskmap[kvsplit[0]] = kvsplit[1]
			}
		}
	} else {
		return 0, nil, fmt.Errorf("invalid disk spec %s", spec)
	}
	disksize = strings.TrimSuffix(disksize, "G")
	size, err := strconv.ParseInt(disksize, 10, 32)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to convert %q to int64: %w", disksize, err)
	}
	return size, diskmap, nil
}

func ParseDisk(spec string) (*Disk, error) {
	var channel string
	sectorSize := 0
	serialOpt := []string{}
	multipathed := false

	size, diskmap, err := ParseDiskSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to parse disk spec %q: %w", spec, err)
	}

	for key, value := range diskmap {
		switch key {
		case "channel":
			channel = value
		case "4k":
			sectorSize = 4096
		case "mpath":
			multipathed = true
		case "serial":
			value = "serial=" + value
			serialOpt = append(serialOpt, value)
		default:
			return nil, fmt.Errorf("invalid key %q", key)
		}
	}

	return &Disk{
		Size:          fmt.Sprintf("%dG", size),
		Channel:       channel,
		DeviceOpts:    serialOpt,
		SectorSize:    sectorSize,
		MultiPathDisk: multipathed,
	}, nil
}

// bootIso is an internal struct used by AddIso() and setupIso()
type bootIso struct {
	path      string
	bootindex string
}

// QemuInstance holds an instantiated VM through its lifecycle.
type QemuInstance struct {
	qemu         *exec.Cmd
	architecture string
	tempdir      string
	swtpm        *exec.Cmd
	// Helpers are child processes such as nbd or virtiofsd that should be lifecycle bound to qemu
	helpers            []*exec.Cmd
	hostForwardedPorts []HostForwardPort

	journalPipe *os.File

	qmpSocket     *qmp.SocketMonitor
	qmpSocketPath string
}

// Pid returns the PID of QEMU process.
func (inst *QemuInstance) Pid() int {
	return inst.qemu.Process.Pid
}

// Kill kills the VM instance.
func (inst *QemuInstance) Kill() error {
	return inst.qemu.Process.Kill()
}

// SSHAddress returns the IP address with the forwarded port (host-side).
func (inst *QemuInstance) SSHAddress() (string, error) {
	for _, fwdPorts := range inst.hostForwardedPorts {
		if fwdPorts.Service == "ssh" {
			return fmt.Sprintf("127.0.0.1:%d", fwdPorts.HostPort), nil
		}
	}
	return "", fmt.Errorf("didn't find an address")
}

// Wait for the qemu process to exit
func (inst *QemuInstance) Wait() error {
	return inst.qemu.Wait()
}

// WaitIgnitionError will only return if the instance
// failed inside the initramfs.  The resulting string will
// be a newline-delimited stream of JSON strings, as returned
// by `journalctl -o json`.
func (inst *QemuInstance) WaitIgnitionError(ctx context.Context) (string, error) {
	b := bufio.NewReaderSize(inst.journalPipe, 64768)
	var r strings.Builder
	iscorrupted := false
	_, err := b.Peek(1)
	if err != nil {
		// It's normal to get EOF if we didn't catch an error and qemu
		// is shutting down.  We also need to handle when the Destroy()
		// function closes the journal FD on us.
		if e, ok := err.(*os.PathError); ok && e.Err == os.ErrClosed {
			return "", nil
		} else if err == io.EOF {
			return "", nil
		}
		return "", errors.Wrapf(err, "Reading from journal")
	}
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		line, prefix, err := b.ReadLine()
		if err != nil {
			return r.String(), errors.Wrapf(err, "Reading from journal channel")
		}
		if prefix {
			iscorrupted = true
		}
		if len(line) == 0 || string(line) == "{}" {
			break
		}
		r.Write(line)
		r.Write([]byte("\n"))
	}
	if iscorrupted {
		return r.String(), fmt.Errorf("journal was truncated due to overly long line")
	}
	return r.String(), nil
}

// WaitAll wraps the process exit as well as WaitIgnitionError,
// returning an error if either fail.
func (inst *QemuInstance) WaitAll(ctx context.Context) error {
	c := make(chan error)
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Machine terminated.
	go func() {
		select {
		case <-waitCtx.Done():
			c <- waitCtx.Err()
		case c <- inst.Wait():
		}

	}()

	return <-c
}

// Destroy kills the instance and associated sidecar processes.
func (inst *QemuInstance) Destroy() {
	if inst.qmpSocket != nil {
		inst.qmpSocket.Disconnect() //nolint // Ignore Errors
		inst.qmpSocket = nil
		os.Remove(inst.qmpSocketPath) //nolint // Ignore Errors
	}
	if inst.journalPipe != nil {
		inst.journalPipe.Close()
		inst.journalPipe = nil
	}
	// kill is safe if already dead
	if err := inst.Kill(); err != nil {
		klog.Errorf("Error killing qemu instance %v: %v", inst.Pid(), err)
	}
	if inst.swtpm.Process != nil {
		inst.swtpm.Process.Kill() //nolint // Ignore errors
		inst.swtpm.Process = nil
	}
	for _, p := range inst.helpers {
		if p.Process != nil {
			p.Process.Kill() //nolint // Ignore errors
		}
	}
	inst.helpers = nil

	if inst.tempdir != "" {
		if err := os.RemoveAll(inst.tempdir); err != nil {
			klog.Errorf("Error removing tempdir: %v", err)
		}
	}
}

// RemovePrimaryBlockDevice deletes the primary device from a qemu instance
// and sets the secondary device as primary. It expects that all block devices
// with device name disk-<N> are mirrors.
func (inst *QemuInstance) RemovePrimaryBlockDevice() (err2 error) {
	var primaryDevice string
	var secondaryDevicePath string

	blkdevs, err := inst.listBlkDevices()
	if err != nil {
		return errors.Wrapf(err, "Could not list block devices through qmp")
	}
	// This tries to identify the primary device by looking into
	// a `BackingFileDepth` parameter of a device and check if
	// it is a removable and part of `virtio-blk-pci` devices.
	for _, dev := range blkdevs.Return {
		if !dev.Removable && strings.HasPrefix(dev.Device, "disk-") {
			if dev.Inserted.BackingFileDepth == 1 {
				primaryDevice = dev.DevicePath
			} else {
				secondaryDevicePath = dev.DevicePath
			}
		}
	}
	if err := inst.setBootIndexForDevice(primaryDevice, -1); err != nil {
		return errors.Wrapf(err, "Could not set bootindex for %v", primaryDevice)
	}
	primaryDevice = primaryDevice[:strings.LastIndex(primaryDevice, "/")]
	if err := inst.deleteBlockDevice(primaryDevice); err != nil {
		return errors.Wrapf(err, "Could not delete primary device %v", primaryDevice)
	}
	if len(secondaryDevicePath) == 0 {
		return errors.Wrapf(err, "Could not find secondary device")
	}
	if err := inst.setBootIndexForDevice(secondaryDevicePath, 1); err != nil {
		return errors.Wrapf(err, "Could not set bootindex for  %v", secondaryDevicePath)
	}

	return nil
}

// A directory mounted from the host into the guest, via 9p or virtiofs
type HostMount struct {
	src      string
	dest     string
	readonly bool
}

// QemuBuilder is a configurator that can then create a qemu instance
type QemuBuilder struct {
	// File to which to redirect the serial console
	ConsoleFile string

	// If set, use QEMU full emulation for the target architecture
	architecture string
	// MemoryMiB defaults to 1024 on most architectures, others it may be 2048
	MemoryMiB int
	// Processors < 0 means to use host count, unset means 1, values > 1 are directly used
	Processors int
	UUID       string
	Firmware   string
	Swtpm      bool
	Pdeathsig  bool
	Argv       []string

	// AppendKernelArgs are appended to the bootloader config
	AppendKernelArgs string

	// AppendFirstbootKernelArgs are written to /boot/ignition
	AppendFirstbootKernelArgs string

	Hostname string

	InheritConsole bool

	iso         *bootIso
	isoAsDisk   bool
	primaryDisk *Disk
	// primaryIsBoot is true if the only boot media should be the primary disk
	primaryIsBoot bool

	// tempdir holds our temporary files
	tempdir string

	UsermodeNetworking        bool
	usermodeNetworkingAddr    string
	RestrictNetworking        bool
	requestedHostForwardPorts []HostForwardPort
	additionalNics            int
	netbootP                  string
	netbootDir                string

	finalized bool
	diskID    uint
	disks     []*Disk
	// virtioSerialID is incremented for each device
	virtioSerialID uint
	// hostMounts is an array of directories mounted (via 9p or virtiofs) from the host
	hostMounts []HostMount
	// fds is file descriptors we own to pass to qemu
	fds []*os.File
}

// NewQemuBuilder creates a new build for QEMU with default settings.
func NewQemuBuilder() *QemuBuilder {
	var defaultFirmware string
	switch coreosarch.CurrentRpmArch() {
	case "x86_64":
		defaultFirmware = "bios"
	case "aarch64":
		defaultFirmware = "uefi"
	default:
		defaultFirmware = ""
	}
	ret := QemuBuilder{
		Firmware:     defaultFirmware,
		Swtpm:        true,
		Pdeathsig:    true,
		Argv:         []string{},
		architecture: coreosarch.CurrentRpmArch(),
	}
	return &ret
}

func (builder *QemuBuilder) ensureTempdir() error {
	if builder.tempdir != "" {
		return nil
	}
	tempdir, err := os.MkdirTemp("/var/tmp", "mantle-qemu")
	if err != nil {
		return err
	}
	builder.tempdir = tempdir
	return nil
}

// Small wrapper around os.CreateTemp() to avoid leaking our tempdir to
// others.
func (builder *QemuBuilder) TempFile(pattern string) (*os.File, error) {
	if err := builder.ensureTempdir(); err != nil {
		return nil, err
	}
	return os.CreateTemp(builder.tempdir, pattern)
}

// AddFd appends a file descriptor that will be passed to qemu,
// returning a "/dev/fdset/<num>" argument that one can use with e.g.
// -drive file=/dev/fdset/<num>.
func (builder *QemuBuilder) AddFd(fd *os.File) string {
	set := len(builder.fds) + 1
	builder.fds = append(builder.fds, fd)
	return fmt.Sprintf("/dev/fdset/%d", set)
}

// virtio returns a virtio device argument for qemu, which is architecture dependent
func virtio(arch, device, args string) string {
	var suffix string
	switch arch {
	case "x86_64", "ppc64le", "aarch64":
		suffix = "pci"
	case "s390x":
		suffix = "ccw"
	default:
		panic(fmt.Sprintf("RpmArch %s unhandled in virtio()", arch))
	}
	return fmt.Sprintf("virtio-%s-%s,%s", device, suffix, args)
}

// EnableUsermodeNetworking configure forwarding for all requested ports,
// via usermode network helpers.
func (builder *QemuBuilder) EnableUsermodeNetworking(h []HostForwardPort, usernetAddr string) {
	builder.UsermodeNetworking = true
	builder.requestedHostForwardPorts = h
	builder.usermodeNetworkingAddr = usernetAddr
}

func (builder *QemuBuilder) SetNetbootP(filename, dir string) {
	builder.UsermodeNetworking = true
	builder.netbootP = filename
	builder.netbootDir = dir
}

func (builder *QemuBuilder) AddAdditionalNics(additionalNics int) {
	builder.additionalNics = additionalNics
}

func (builder *QemuBuilder) setupNetworking() error {
	netdev := "user,id=eth0"
	for i := range builder.requestedHostForwardPorts {
		address := fmt.Sprintf(":%d", builder.requestedHostForwardPorts[i].HostPort)
		// Possible race condition between getting the port here and using it
		// with qemu -- trade off for simpler port management
		l, err := net.Listen("tcp", address)
		if err != nil {
			return err
		}
		l.Close()
		builder.requestedHostForwardPorts[i].HostPort = l.Addr().(*net.TCPAddr).Port
		netdev += fmt.Sprintf(",hostfwd=tcp:127.0.0.1:%d-:%d",
			builder.requestedHostForwardPorts[i].HostPort,
			builder.requestedHostForwardPorts[i].GuestPort)
	}

	if builder.Hostname != "" {
		netdev += fmt.Sprintf(",hostname=%s", builder.Hostname)
	}
	if builder.RestrictNetworking {
		netdev += ",restrict=on"
	}
	if builder.usermodeNetworkingAddr != "" {
		netdev += ",net=" + builder.usermodeNetworkingAddr
	}
	if builder.netbootP != "" {
		// do an early stat so we fail with a nicer error now instead of in the VM
		if _, err := os.Stat(filepath.Join(builder.netbootDir, builder.netbootP)); err != nil {
			return err
		}
		tftpDir := ""
		relpath := ""
		if builder.netbootDir == "" {
			absPath, err := filepath.Abs(builder.netbootP)
			if err != nil {
				return err
			}
			tftpDir = filepath.Dir(absPath)
			relpath = filepath.Base(absPath)
		} else {
			absPath, err := filepath.Abs(builder.netbootDir)
			if err != nil {
				return err
			}
			tftpDir = absPath
			relpath = builder.netbootP
		}
		netdev += fmt.Sprintf(",tftp=%s,bootfile=/%s", tftpDir, relpath)
		builder.Append("-boot", "order=n")
	}

	builder.Append("-netdev", netdev, "-device", virtio(builder.architecture, "net", "netdev=eth0"))
	return nil
}

func (builder *QemuBuilder) setupAdditionalNetworking() error {
	macCounter := 0
	netOffset := 30
	for i := 1; i <= builder.additionalNics; i++ {
		idSuffix := fmt.Sprintf("%d", i)
		netSuffix := fmt.Sprintf("%d", netOffset+i)
		macSuffix := fmt.Sprintf("%02x", macCounter)

		netdev := fmt.Sprintf("user,id=eth%s,dhcpstart=10.0.2.%s", idSuffix, netSuffix)
		device := virtio(builder.architecture, "net", fmt.Sprintf("netdev=eth%s,mac=52:55:00:d1:56:%s", idSuffix, macSuffix))
		builder.Append("-netdev", netdev, "-device", device)
		macCounter++
	}

	return nil
}

// SetArchitecture enables qemu full emulation for the target architecture.
func (builder *QemuBuilder) SetArchitecture(arch string) error {
	switch arch {
	case "x86_64", "aarch64", "s390x", "ppc64le":
		builder.architecture = arch
		return nil
	}
	return fmt.Errorf("architecture %s not supported by coreos-assembler qemu", arch)
}

// MountHost sets up a mount point from the host to guest.
// Note that virtiofs does not currently support read-only mounts (which is really surprising!).
// We do mount it read-only by default in the guest, however.
func (builder *QemuBuilder) MountHost(source, dest string, readonly bool) {
	builder.hostMounts = append(builder.hostMounts, HostMount{src: source, dest: dest, readonly: readonly})
}

// supportsSwtpm if the target system supports a virtual TPM device
func (builder *QemuBuilder) supportsSwtpm() bool {
	switch builder.architecture {
	case "s390x":
		// s390x does not support a backend for TPM
		return false
	}
	return true
}

func resolveBackingFile(backingFile string) (string, error) {
	backingFile, err := filepath.Abs(backingFile)
	if err != nil {
		return "", err
	}
	// Keep the COW image from breaking if the "latest" symlink changes.
	// Ignore /proc/*/fd/* paths, since they look like symlinks but
	// really aren't.
	if !strings.HasPrefix(backingFile, "/proc/") {
		backingFile, err = filepath.EvalSymlinks(backingFile)
		if err != nil {
			return "", err
		}
	}
	return backingFile, nil
}

// prepare creates the target disk and sets all the runtime attributes
// for use by the QemuBuilder.
func (disk *Disk) prepare(builder *QemuBuilder) error {
	if err := builder.ensureTempdir(); err != nil {
		return err
	}
	tmpf, err := os.CreateTemp(builder.tempdir, "disk")
	if err != nil {
		return err
	}
	disk.dstFileName = tmpf.Name()

	imgOpts := []string{"create", "-f", "qcow2", disk.dstFileName}
	// On filesystems like btrfs, qcow2 files can become much more fragmented
	// if copy-on-write is enabled.  We don't need that, our disks are ephemeral.
	// https://gitlab.gnome.org/GNOME/gnome-boxes/-/issues/88
	// https://btrfs.wiki.kernel.org/index.php/Gotchas#Fragmentation
	// https://www.redhat.com/archives/libvir-list/2014-July/msg00361.html
	qcow2Opts := "nocow=on"
	if disk.BackingFile != "" {
		backingFile, err := resolveBackingFile(disk.BackingFile)
		if err != nil {
			return err
		}
		qcow2Opts += fmt.Sprintf(",backing_file=%s,lazy_refcounts=on", backingFile)
		format := disk.BackingFormat
		if format == "" {
			// QEMU 5 warns if format is omitted, let's do detection for the common case
			// on our own.
			if strings.HasSuffix(backingFile, "qcow2") {
				format = "qcow2"
			}
		}
		if format != "" {
			qcow2Opts += fmt.Sprintf(",backing_fmt=%s", format)
		}
	}
	imgOpts = append(imgOpts, "-o", qcow2Opts)

	if disk.Size != "" {
		imgOpts = append(imgOpts, disk.Size)
	}
	qemuImg := exec.Command("qemu-img", imgOpts...)
	qemuImg.Stderr = os.Stderr

	if err := qemuImg.Run(); err != nil {
		return err
	}

	fdSet := builder.AddFd(tmpf)
	disk.attachEndPoint = fdSet

	// MultiPathDisks must be NBD remote mounted
	if disk.MultiPathDisk || disk.NbdDisk {
		socketName := fmt.Sprintf("%s.socket", disk.dstFileName)
		shareCount := "1"
		if disk.MultiPathDisk {
			shareCount = "2"
		}
		disk.nbdServCmd = exec.Command("qemu-nbd",
			"--format", "qcow2",
			"--cache", "unsafe",
			"--discard", "unmap",
			"--socket", socketName,
			"--share", shareCount,
			disk.dstFileName)
		disk.attachEndPoint = fmt.Sprintf("nbd:unix:%s", socketName)
	}

	builder.diskID++
	builder.disks = append(builder.disks, disk)
	return nil
}

func (builder *QemuBuilder) addDiskImpl(disk *Disk, primary bool) error {
	if err := disk.prepare(builder); err != nil {
		return err
	}
	diskOpts := disk.DeviceOpts
	if primary {
		diskOpts = append(diskOpts, "serial=primary-disk")
	} else {
		foundserial := false
		for _, opt := range diskOpts {
			if strings.HasPrefix(opt, "serial=") {
				foundserial = true
			}
		}
		if !foundserial {
			diskOpts = append(diskOpts, "serial="+fmt.Sprintf("disk%d", builder.diskID))
		}
	}
	channel := disk.Channel
	if channel == "" {
		channel = "virtio"
	}
	if disk.SectorSize != 0 {
		diskOpts = append(diskOpts, fmt.Sprintf("physical_block_size=%[1]d,logical_block_size=%[1]d", disk.SectorSize))
	}
	// Primary disk gets bootindex 1, all other disks have unspecified
	// bootindex, which means lower priority.
	if primary {
		diskOpts = append(diskOpts, "bootindex=1")
	}

	opts := ""
	if len(diskOpts) > 0 {
		opts = "," + strings.Join(diskOpts, ",")
	}

	id := fmt.Sprintf("disk-%d", builder.diskID)

	// Avoid file locking detection, and the disks we create
	// here are always currently ephemeral.
	defaultDiskOpts := "auto-read-only=off,cache=unsafe"
	if len(disk.DriveOpts) > 0 {
		defaultDiskOpts += "," + strings.Join(disk.DriveOpts, ",")
	}

	if disk.MultiPathDisk {
		// Fake a NVME device with a fake WWN. All these attributes are needed in order
		// to trick multipath-tools that this is a "real" multipath device.
		// Each disk is presented on its own controller.

		// The WWN needs to be a unique uint64 number
		wwn := rand.Uint64()

		var bus string
		switch builder.architecture {
		case "x86_64", "ppc64le", "aarch64":
			bus = "pci"
		case "s390x":
			bus = "ccw"
		default:
			panic(fmt.Sprintf("Mantle doesn't know which bus type to use on %s", builder.architecture))
		}

		for i := 0; i < 2; i++ {
			if i == 1 {
				opts = strings.Replace(opts, "bootindex=1", "bootindex=2", -1)
			}
			pID := fmt.Sprintf("mpath%d%d", builder.diskID, i)
			scsiID := fmt.Sprintf("scsi_%s", pID)
			builder.Append("-device", fmt.Sprintf("virtio-scsi-%s,id=%s", bus, scsiID))
			builder.Append("-device",
				fmt.Sprintf("scsi-hd,bus=%s.0,drive=%s,vendor=NVME,product=VirtualMultipath,wwn=%d%s",
					scsiID, pID, wwn, opts))
			builder.Append("-drive", fmt.Sprintf("if=none,id=%s,format=raw,file=%s,media=disk,%s",
				pID, disk.attachEndPoint, defaultDiskOpts))
		}
	} else {
		if !disk.NbdDisk {
			// In the non-multipath/nbd case we can just unlink the disk now
			// and avoid leaking space if we get Ctrl-C'd (though it's best if
			// higher level code catches SIGINT and cleans up the directory)
			os.Remove(disk.dstFileName)
		}
		disk.dstFileName = ""
		switch channel {
		case "virtio":
			builder.Append("-device", virtio(builder.architecture, "blk", fmt.Sprintf("drive=%s%s", id, opts)))
		case "nvme":
			builder.Append("-device", fmt.Sprintf("nvme,drive=%s%s", id, opts))
		default:
			panic(fmt.Sprintf("Unhandled channel: %s", channel))
		}

		// Default to cache=unsafe
		builder.Append("-drive", fmt.Sprintf("if=none,id=%s,file=%s,%s",
			id, disk.attachEndPoint, defaultDiskOpts))
	}
	return nil
}

// AddPrimaryDisk sets up the primary disk for the instance.
func (builder *QemuBuilder) AddPrimaryDisk(disk *Disk) error {
	if builder.primaryDisk != nil {
		return errors.New("Multiple primary disks specified")
	}
	// We do this one lazily in order to break an ordering requirement
	// for SetConfig() and AddPrimaryDisk() in the case where the
	// config needs to be injected into the disk.
	builder.primaryDisk = disk
	return nil
}

// AddBootDisk sets the instance to boot only from the target disk
func (builder *QemuBuilder) AddBootDisk(disk *Disk) error {
	if err := builder.AddPrimaryDisk(disk); err != nil {
		return err
	}
	builder.primaryIsBoot = true
	return nil
}

// AddDisk adds a secondary disk for the instance.
func (builder *QemuBuilder) AddDisk(disk *Disk) error {
	return builder.addDiskImpl(disk, false)
}

// AddDisksFromSpecs adds multiple secondary disks from their specs.
func (builder *QemuBuilder) AddDisksFromSpecs(specs []string) error {
	for _, spec := range specs {
		if disk, err := ParseDisk(spec); err != nil {
			return errors.Wrapf(err, "parsing additional disk spec '%s'", spec)
		} else if err = builder.AddDisk(disk); err != nil {
			return errors.Wrapf(err, "adding additional disk '%s'", spec)
		}
	}
	return nil
}

// AddIso adds an ISO image, optionally configuring its boot index
// If asDisk is set, attach the ISO as a disk drive (as though it was copied
// to a USB stick) and overwrite the El Torito signature in the image
// (to force QEMU's UEFI firmware to boot via the hybrid ESP).
func (builder *QemuBuilder) AddIso(path string, bootindexStr string, asDisk bool) error {
	builder.iso = &bootIso{
		path:      path,
		bootindex: bootindexStr,
	}
	builder.isoAsDisk = asDisk
	return nil
}

func (builder *QemuBuilder) finalize() {
	if builder.finalized {
		return
	}
	if builder.MemoryMiB == 0 {
		// FIXME; Required memory should really be a property of the tests, and
		// let's try to drop these arch-specific overrides.  ARM was bumped via
		// commit 09391907c0b25726374004669fa6c2b161e3892f
		// Commit:     Geoff Levand <geoff@infradead.org>
		// CommitDate: Mon Aug 21 12:39:34 2017 -0700
		//
		// kola: More memory for arm64 qemu guest machines
		//
		// arm64 guest machines seem to run out of memory with 1024 MiB of
		// RAM, so increase to 2048 MiB.

		// Then later, other non-x86_64 seemed to just copy that.
		memory := 1024
		switch builder.architecture {
		case "aarch64", "s390x", "ppc64le":
			memory = 2048
		}
		builder.MemoryMiB = memory
	}
	builder.finalized = true
}

// Append appends additional arguments for QEMU.
func (builder *QemuBuilder) Append(args ...string) {
	builder.Argv = append(builder.Argv, args...)
}

// baseQemuArgs takes a board and returns the basic qemu
// arguments needed for the current architecture.
func baseQemuArgs(arch string, memoryMiB int) ([]string, error) {
	// memoryDevice is the object identifier we use for the backing RAM
	const memoryDevice = "mem"

	kvm := true
	hostArch := coreosarch.CurrentRpmArch()
	// The machine argument needs to reference our memory device; see below
	machineArg := "memory-backend=" + memoryDevice
	accel := "accel=kvm"
	if _, ok := os.LookupEnv("COSA_NO_KVM"); ok || hostArch != arch {
		accel = "accel=tcg"
		kvm = false
	}
	machineArg += "," + accel
	var ret []string
	switch arch {
	case "x86_64":
		ret = []string{
			"qemu-system-x86_64",
			"-machine", machineArg,
		}
	case "aarch64":
		ret = []string{
			"qemu-system-aarch64",
			"-machine", "virt,gic-version=max," + machineArg,
		}
	case "s390x":
		ret = []string{
			"qemu-system-s390x",
			"-machine", "s390-ccw-virtio," + machineArg,
		}
	case "ppc64le":
		ret = []string{
			"qemu-system-ppc64",
			// kvm-type=HV ensures we use bare metal KVM and not "user mode"
			// https://qemu.readthedocs.io/en/latest/system/ppc/pseries.html#switching-between-the-kvm-pr-and-kvm-hv-kernel-module
			"-machine", "pseries,kvm-type=HV," + machineArg,
		}
	default:
		return nil, fmt.Errorf("architecture %s not supported for qemu", arch)
	}
	if kvm {
		ret = append(ret, "-cpu", "host")
	} else {
		if arch == "x86_64" {
			// the default qemu64 CPU model does not support x86_64_v2
			// causing crashes on EL9+ kernels
			// see https://bugzilla.redhat.com/show_bug.cgi?id=2060839
			ret = append(ret, "-cpu", "Nehalem")
		}
	}
	// And define memory using a memfd (in shared mode), which is needed for virtiofs
	ret = append(ret, "-object", fmt.Sprintf("memory-backend-memfd,id=%s,size=%dM,share=on", memoryDevice, memoryMiB))
	ret = append(ret, "-m", fmt.Sprintf("%d", memoryMiB))
	return ret, nil
}

func (builder *QemuBuilder) setupUefi(secureBoot bool) error {
	switch coreosarch.CurrentRpmArch() {
	case "x86_64":
		varsVariant := ""
		if secureBoot {
			varsVariant = ".secboot"
		}
		varsSrc, err := os.Open(fmt.Sprintf("/usr/share/edk2/ovmf/OVMF_VARS%s.fd", varsVariant))
		if err != nil {
			return err
		}
		defer varsSrc.Close()
		vars, err := os.CreateTemp("", "mantle-qemu")
		if err != nil {
			return err
		}
		if _, err := io.Copy(vars, varsSrc); err != nil {
			return err
		}
		_, err = vars.Seek(0, 0)
		if err != nil {
			return err
		}

		fdset := builder.AddFd(vars)
		builder.Append("-drive", fmt.Sprintf("file=/usr/share/edk2/ovmf/OVMF_CODE%s.fd,if=pflash,format=raw,unit=0,readonly=on,auto-read-only=off", varsVariant))
		builder.Append("-drive", fmt.Sprintf("file=%s,if=pflash,format=raw,unit=1,readonly=off,auto-read-only=off", fdset))
		builder.Append("-machine", "q35")
	case "aarch64":
		if secureBoot {
			return fmt.Errorf("architecture %s doesn't have support for secure boot in kola", coreosarch.CurrentRpmArch())
		}
		vars, err := os.CreateTemp("", "mantle-qemu")
		if err != nil {
			return err
		}
		//67108864 bytes is expected size of the "VARS" by qemu
		err = vars.Truncate(67108864)
		if err != nil {
			return err
		}

		_, err = vars.Seek(0, 0)
		if err != nil {
			return err
		}

		fdset := builder.AddFd(vars)
		builder.Append("-drive", "file=/usr/share/edk2/aarch64/QEMU_EFI-silent-pflash.raw,if=pflash,format=raw,unit=0,readonly=on,auto-read-only=off")
		builder.Append("-drive", fmt.Sprintf("file=%s,if=pflash,format=raw,unit=1,readonly=off,auto-read-only=off", fdset))
	default:
		panic(fmt.Sprintf("Architecture %s doesn't have support for UEFI in qemu.", coreosarch.CurrentRpmArch()))
	}

	return nil
}


// VirtioChannelRead allocates a virtio-serial channel that will appear in
// the guest as /dev/virtio-ports/<name>.  The guest can write to it, and
// the host can read.
func (builder *QemuBuilder) VirtioChannelRead(name string) (*os.File, error) {
	// Set up the virtio channel to get Ignition failures by default
	r, w, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrapf(err, "virtioChannelRead creating pipe")
	}
	if builder.virtioSerialID == 0 {
		builder.Append("-device", "virtio-serial")
	}
	builder.virtioSerialID++
	id := fmt.Sprintf("virtioserial%d", builder.virtioSerialID)
	// https://www.redhat.com/archives/libvir-list/2015-December/msg00305.html
	builder.Append("-chardev", fmt.Sprintf("file,id=%s,path=%s,append=on", id, builder.AddFd(w)))
	builder.Append("-device", fmt.Sprintf("virtserialport,chardev=%s,name=%s", id, name))

	return r, nil
}

// SerialPipe reads the serial console output into a pipe
func (builder *QemuBuilder) SerialPipe() (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrapf(err, "virtioChannelRead creating pipe")
	}
	id := "serialpipe"
	builder.Append("-chardev", fmt.Sprintf("file,id=%s,path=%s,append=on", id, builder.AddFd(w)))
	builder.Append("-serial", fmt.Sprintf("chardev:%s", id))

	return r, nil
}

// createVirtiofsCmd returns a new command instance configured to launch virtiofsd.
func createVirtiofsCmd(directory, socketPath string) *exec.Cmd {
	args := []string{"--sandbox", "none", "--socket-path", socketPath, "--shared-dir", "."}
	// Work around https://gitlab.com/virtio-fs/virtiofsd/-/merge_requests/197
	if os.Getuid() == 0 {
		args = append(args, "--modcaps=-mknod:-setfcap")
	}
	// We don't need seccomp filtering; we trust our workloads. This incidentally
	// works around issues like https://gitlab.com/virtio-fs/virtiofsd/-/merge_requests/200.
	args = append(args, "--seccomp=none")
	cmd := exec.Command("/usr/libexec/virtiofsd", args...)
	// This sets things up so that the `.` we passed in the arguments is the target directory
	cmd.Dir = directory
	// Quiet the daemon by default
	cmd.Env = append(cmd.Env, "RUST_LOG=ERROR")
	// But we do want to see errors
	cmd.Stderr = os.Stderr
	// Like other processes, "lifecycle bind" it to us
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
	return cmd
}

// Exec tries to run a QEMU instance with the given settings.
func (builder *QemuBuilder) Exec() (*QemuInstance, error) {
	builder.finalize()
	var err error

	inst := QemuInstance{}
	cleanupInst := false
	defer func() {
		if cleanupInst {
			inst.Destroy()
		}
	}()

	argv, err := baseQemuArgs(builder.architecture, builder.MemoryMiB)
	if err != nil {
		return nil, err
	}

	if builder.Processors < 0 {
		nproc, err := nproc.GetProcessors()
		if err != nil {
			return nil, errors.Wrapf(err, "qemu estimating processors")
		}
		// cap qemu smp at some reasonable level; sometimes our tooling runs
		// on 32-core servers (64 hyperthreads) and there's no reason to
		// try to match that.
		if nproc > 16 {
			nproc = 16
		}

		builder.Processors = int(nproc)
	} else if builder.Processors == 0 {
		builder.Processors = 1
	}
	argv = append(argv, "-smp", fmt.Sprintf("%d", builder.Processors))

	switch builder.Firmware {
	case "":
		// Nothing to do, use qemu default
	case "uefi":
		if err := builder.setupUefi(false); err != nil {
			return nil, err
		}
	case "uefi-secure":
		if err := builder.setupUefi(true); err != nil {
			return nil, err
		}
	case "bios":
		if coreosarch.CurrentRpmArch() != "x86_64" {
			return nil, fmt.Errorf("unknown firmware: %s", builder.Firmware)
		}
	default:
		return nil, fmt.Errorf("unknown firmware: %s", builder.Firmware)
	}

	// We always provide a random source
	argv = append(argv, "-object", "rng-random,filename=/dev/urandom,id=rng0",
		"-device", virtio(builder.architecture, "rng", "rng=rng0"))
	if builder.UUID != "" {
		argv = append(argv, "-uuid", builder.UUID)
	}

	// We never want a popup window
	argv = append(argv, "-nographic")

	// We want to customize everything from scratch, so avoid defaults
	argv = append(argv, "-nodefaults")

	if builder.primaryDisk != nil {
		if err := builder.addDiskImpl(builder.primaryDisk, true); err != nil {
			return nil, err
		}
		if builder.primaryIsBoot {
			argv = append(argv, "-boot", "order=c,strict=on")
		}
	}

	// Start up the disks. Since the disk may be served via NBD,
	// we can't use builder.AddFd (no support for fdsets), so we at the disk to the tmpFiles.
	for _, disk := range builder.disks {
		if disk.nbdServCmd != nil {
			if err := disk.nbdServCmd.Start(); err != nil {
				return nil, errors.Wrapf(err, "spawing nbd server")
			}
			inst.helpers = append(inst.helpers, disk.nbdServCmd)
		}
	}

	// Handle Usermode Networking
	if builder.UsermodeNetworking {
		if err := builder.setupNetworking(); err != nil {
			return nil, err
		}
		inst.hostForwardedPorts = builder.requestedHostForwardPorts
	}

	// Handle Additional NICs networking
	if builder.additionalNics > 0 {
		if err := builder.setupAdditionalNetworking(); err != nil {
			return nil, err
		}
	}

	// Handle Software TPM
	if builder.Swtpm && builder.supportsSwtpm() {
		err = builder.ensureTempdir()
		if err != nil {
			return nil, err
		}
		swtpmSock := filepath.Join(builder.tempdir, "swtpm-sock")
		swtpmdir := filepath.Join(builder.tempdir, "swtpm")
		if err := os.Mkdir(swtpmdir, 0755); err != nil {
			return nil, err
		}

		inst.swtpm = exec.Command("swtpm", "socket", "--tpm2",
			"--ctrl", fmt.Sprintf("type=unixio,path=%s", swtpmSock),
			"--terminate", "--tpmstate", fmt.Sprintf("dir=%s", swtpmdir))
		// For now silence the swtpm stderr as it prints errors when
		// disconnected, but that's normal.
		if builder.Pdeathsig {
			inst.swtpm.SysProcAttr = &syscall.SysProcAttr{
				Pdeathsig: syscall.SIGTERM,
			}
		}
		if err = inst.swtpm.Start(); err != nil {
			return nil, err
		}
		// We need to wait until the swtpm starts up
		err = retry.Retry(10, 500*time.Millisecond, func() error {
			_, err := os.Stat(swtpmSock)
			return err
		})
		if err != nil {
			return nil, err
		}
		argv = append(argv, "-chardev", fmt.Sprintf("socket,id=chrtpm,path=%s", swtpmSock), "-tpmdev", "emulator,id=tpm0,chardev=chrtpm")
		// There are different device backends on each architecture
		switch builder.architecture {
		case "x86_64":
			argv = append(argv, "-device", "tpm-tis,tpmdev=tpm0")
		case "aarch64":
			argv = append(argv, "-device", "tpm-tis-device,tpmdev=tpm0")
		case "ppc64le":
			argv = append(argv, "-device", "tpm-spapr,tpmdev=tpm0")
		}

	}

	// Set up QMP (currently used to switch boot order after first boot on aarch64.
	// The qmp socket path must be unique to the instance.
	inst.qmpSocketPath = filepath.Join(builder.tempdir, fmt.Sprintf("qmp-%d.sock", time.Now().UnixNano()))
	qmpID := "qemu-qmp"
	builder.Append("-chardev", fmt.Sprintf("socket,id=%s,path=%s,server=on,wait=off", qmpID, inst.qmpSocketPath))
	builder.Append("-mon", fmt.Sprintf("chardev=%s,mode=control", qmpID))

	// Set up the virtio channel to get Ignition failures by default
	journalPipeR, err := builder.VirtioChannelRead("com.coreos.ignition.journal")
	inst.journalPipe = journalPipeR
	if err != nil {
		return nil, err
	}

	// Process virtiofs mounts
	if len(builder.hostMounts) > 0 {
		if err := builder.ensureTempdir(); err != nil {
			return nil, err
		}

		// Spawn off a virtiofsd helper per mounted path
		virtiofsHelpers := make(map[string]*exec.Cmd)
		for i, hostmnt := range builder.hostMounts {
			// By far the most common failure to spawn virtiofsd will be a typo'd source directory,
			// so let's synchronously check that ourselves here.
			if _, err := os.Stat(hostmnt.src); err != nil {
				return nil, fmt.Errorf("failed to access virtiofs source directory %s", hostmnt.src)
			}
			virtiofsChar := fmt.Sprintf("virtiofschar%d", i)
			virtiofsdSocket := filepath.Join(builder.tempdir, fmt.Sprintf("virtiofsd-%d.sock", i))
			builder.Append("-chardev", fmt.Sprintf("socket,id=%s,path=%s", virtiofsChar, virtiofsdSocket))
			builder.Append("-device", fmt.Sprintf("vhost-user-fs-pci,queue-size=1024,chardev=%s,tag=%s", virtiofsChar, hostmnt.dest))
			// TODO: Honor hostmnt.readonly somehow here (add an option to virtiofsd)
			p := createVirtiofsCmd(hostmnt.src, virtiofsdSocket)
			if err := p.Start(); err != nil {
				return nil, fmt.Errorf("failed to start virtiofsd")
			}
			virtiofsHelpers[virtiofsdSocket] = p
		}
		// Loop waiting for the sockets to appear
		err := retry.RetryUntilTimeout(10*time.Minute, 1*time.Second, func() error {
			found := []string{}
			for sockpath := range virtiofsHelpers {
				if _, err := os.Stat(sockpath); err == nil {
					found = append(found, sockpath)
				}
			}
			for _, sockpath := range found {
				helper := virtiofsHelpers[sockpath]
				inst.helpers = append(inst.helpers, helper)
				delete(virtiofsHelpers, sockpath)
			}
			if len(virtiofsHelpers) == 0 {
				return nil
			}
			waitingFor := []string{}
			for socket := range virtiofsHelpers {
				waitingFor = append(waitingFor, socket)
			}
			return fmt.Errorf("waiting for virtiofsd sockets: %s", strings.Join(waitingFor, " "))
		})
		if err != nil {
			return nil, err
		}
	}

	fdnum := 3 // first additional file starts at position 3
	for i := range builder.fds {
		fdset := i + 1 // Start at 1
		argv = append(argv, "-add-fd", fmt.Sprintf("fd=%d,set=%d", fdnum, fdset))
		fdnum++
	}

	if builder.ConsoleFile != "" {
		builder.Append("-display", "none", "-chardev", "file,id=log,path="+builder.ConsoleFile, "-serial", "chardev:log")
	} else {
		builder.Append("-serial", "mon:stdio")
	}

	// And the custom arguments
	argv = append(argv, builder.Argv...)

	inst.qemu = exec.Command(argv[0], argv[1:]...)
	inst.architecture = builder.architecture
	cmd := inst.qemu
	cmd.Stderr = os.Stderr

	if builder.Pdeathsig {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGTERM,
		}
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, builder.fds...)

	if builder.InheritConsole {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err = inst.qemu.Start(); err != nil {
		return nil, err
	}

	klog.Infof("Started qemu (%v) with args: %v", inst.qemu.Process.Pid, argv)

	// Transfer ownership of the tempdir
	inst.tempdir = builder.tempdir
	builder.tempdir = ""
	cleanupInst = false

	// Connect to the QMP socket which allows us to control qemu.  We wait up to 30s
	// to avoid flakes on loaded CI systems.  But, probably rather than bumping this
	// any higher it'd be better to try to reduce parallelism.
	if err := retry.Retry(30, 1*time.Second,
		func() error {
			sockMonitor, err := qmp.NewSocketMonitor("unix", inst.qmpSocketPath, 2*time.Second)
			if err != nil {
				return err
			}
			inst.qmpSocket = sockMonitor
			return nil
		}); err != nil {
		return nil, fmt.Errorf("failed to establish qmp connection: %w", err)
	}
	if err := inst.qmpSocket.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect over qmp to qemu instance")
	}

	return &inst, nil
}

// Close drops all resources owned by the builder.
func (builder *QemuBuilder) Close() {
	if builder.fds == nil {
		return
	}
	for _, f := range builder.fds {
		f.Close()
	}
	builder.fds = nil

	if builder.tempdir != "" {
		os.RemoveAll(builder.tempdir)
	}
}
