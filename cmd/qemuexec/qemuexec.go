package qemuexec

import (
	"fmt"
	"strings"

	"github.com/cgwalters/osbuildbootc/internal/pkg/qemu"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	firmware     string
	diskimage    string
	memory       int
	addDisks     []string
	usernet      bool
	cpuCountHost bool
	nvme         bool
	disableKVM   bool

	architecture string

	hostname string

	bindro []string
	bindrw []string

	consoleFile string

	sshCommand string

	additionalNics int

	netboot    string
	netbootDir string

	usernetAddr string

	CmdQemuExec = &cobra.Command{
		Use:   "qemuexec",
		Short: "Execute qemu",
		RunE:  run,
	}
)

func init() {
	CmdQemuExec.Flags().StringVar(&firmware, "firmware", "", "Boot firmware: bios,uefi,uefi-secure (default bios)")
	CmdQemuExec.Flags().StringVar(&diskimage, "image", "", "path to primary disk image")
	CmdQemuExec.Flags().BoolVarP(&usernet, "usernet", "U", false, "Enable usermode networking")
	CmdQemuExec.Flags().BoolVar(&disableKVM, "disable-kvm", false, "Do not use KVM hardware acceleration")
	CmdQemuExec.Flags().StringVarP(&hostname, "hostname", "", "", "Set hostname via DHCP")
	CmdQemuExec.Flags().IntVarP(&memory, "memory", "m", 0, "Memory in MB")
	CmdQemuExec.Flags().StringVar(&architecture, "arch", "", "Use full emulation for target architecture (e.g. aarch64, x86_64, s390x, ppc64le)")
	CmdQemuExec.Flags().StringArrayVarP(&addDisks, "add-disk", "D", []string{}, "Additional disk, human readable size (repeatable)")
	CmdQemuExec.Flags().BoolVar(&cpuCountHost, "auto-cpus", false, "Automatically set number of cpus to host count")
	CmdQemuExec.Flags().BoolVar(&nvme, "nvme", false, "Use NVMe")
	CmdQemuExec.Flags().StringArrayVar(&bindro, "bind-ro", nil, "Mount $hostpath,$guestpath readonly; for example --bind-ro=/path/on/host,/var/mnt/guest)")
	CmdQemuExec.Flags().StringArrayVar(&bindrw, "bind-rw", nil, "Mount $hostpath,$guestpath writable; for example --bind-rw=/path/on/host,/var/mnt/guest)")
	CmdQemuExec.Flags().StringVarP(&consoleFile, "console-to-file", "", "", "Filepath in which to save serial console logs")
	CmdQemuExec.Flags().IntVarP(&additionalNics, "additional-nics", "", 0, "Number of additional NICs to add")
	CmdQemuExec.Flags().StringVarP(&sshCommand, "ssh-command", "x", "", "Command to execute instead of spawning a shell")
	CmdQemuExec.Flags().StringVarP(&netboot, "netboot", "", "", "Filepath to BOOTP program (e.g. PXELINUX/GRUB binary or iPXE script")
	CmdQemuExec.Flags().StringVarP(&netbootDir, "netboot-dir", "", "", "Directory to serve over TFTP (default: BOOTP parent dir). If specified, --netboot is relative to this dir.")
	CmdQemuExec.Flags().StringVarP(&usernetAddr, "usernet-addr", "", "", "Guest IP network (QEMU default is '10.0.2.0/24')")
}

func parseBindOpt(s string) (string, string, error) {
	parts := strings.SplitN(s, ",", 2)
	if len(parts) == 1 {
		return "", "", fmt.Errorf("malformed bind option, required: SRC,DEST")
	}
	return parts[0], parts[1], nil
}

// buildDiskFromOptions generates a disk image template using the process-global
// defaults that were parsed from command line arguments.
func buildDiskFromOptions() *qemu.Disk {
	channel := "virtio"
	if nvme {
		channel = "nvme"
	}
	sectorSize := 0
	options := []string{}
	// Build the disk definition. Note that if kola.QEMUOptions.DiskImage is
	// "" we'll just end up with a blank disk image, which is what we want.
	disk := &qemu.Disk{
		BackingFile: diskimage,
		Channel:     channel,
		SectorSize:  sectorSize,
		DriveOpts:   options,
	}
	return disk
}

func run(cmd *cobra.Command, args []string) error {
	var err error

	/// Qemu allows passing disk images directly, but this bypasses all of our snapshot
	/// infrastructure and it's too easy to accidentally do `cosa run foo.qcow2` instead of
	/// the more verbose (but correct) `cosa run --qemu-image foo.qcow2`.
	/// Anyone who wants persistence can add it as a disk manually.
	removeIdx := -1
	prevIsArg := false
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") {
			prevIsArg = true
		} else {
			if !prevIsArg {
				if strings.HasSuffix(arg, ".qcow2") {
					if diskimage != "" {
						return fmt.Errorf("multiple disk images provided")
					}
					diskimage = arg
					removeIdx = i
					continue
				}
				return fmt.Errorf("unhandled non-option argument passed for qemu: %s", arg)
			}
			prevIsArg = false
		}
	}
	if removeIdx != -1 {
		args = append(args[:removeIdx], args[removeIdx+1:]...)
	}

	builder := qemu.NewQemuBuilder()
	defer builder.Close()

	if architecture != "" {
		if err := builder.SetArchitecture(architecture); err != nil {
			return err
		}
	}

	for _, b := range bindro {
		src, dest, err := parseBindOpt(b)
		if err != nil {
			return err
		}
		builder.MountHost(src, dest, true)
	}
	for _, b := range bindrw {
		src, dest, err := parseBindOpt(b)
		if err != nil {
			return err
		}
		builder.MountHost(src, dest, false)
	}
	if firmware != "" {
		builder.Firmware = firmware
	}
	if diskimage != "" && netboot == "" {
		if err := builder.AddBootDisk(buildDiskFromOptions()); err != nil {
			return err
		}
		if err != nil {
			return err
		}
	}
	builder.Hostname = hostname
	// for historical reasons, both --memory and --qemu-memory are supported
	if memory != 0 {
		builder.MemoryMiB = memory
	}
	if err = builder.AddDisksFromSpecs(addDisks); err != nil {
		return err
	}
	if cpuCountHost {
		builder.Processors = -1
	}
	if usernet || usernetAddr != "" {
		h := []qemu.HostForwardPort{
			{Service: "ssh", HostPort: 0, GuestPort: 22},
		}
		builder.EnableUsermodeNetworking(h, usernetAddr)
	}
	if netboot != "" {
		builder.SetNetbootP(netboot, netbootDir)
	}
	if additionalNics != 0 {
		const maxAdditionalNics = 16
		if additionalNics < 0 || additionalNics > maxAdditionalNics {
			return errors.Wrapf(nil, "additional-nics value cannot be negative or greater than %d", maxAdditionalNics)
		}
		builder.AddAdditionalNics(additionalNics)
	}
	if consoleFile == "" {
		builder.InheritConsole = true
	} else {
		builder.ConsoleFile = consoleFile
	}
	builder.Append(args...)

	inst, err := builder.Exec()
	if err != nil {
		return err
	}
	defer inst.Destroy()

	return inst.Wait()
}
