package osbuild

import (
	"github.com/google/uuid"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
)

// The GRUB2StageOptions describes the bootloader configuration.
//
// The stage is responsible for installing all bootloader files in
// /boot as well as config files in /etc necessary for regenerating
// the configuration in /boot.
//
// Note that it is the role of an assembler to install any necessary
// bootloaders that are stored in the image outside of any filesystem.
type GRUB2StageOptions struct {
	RootFilesystemUUID uuid.UUID    `json:"root_fs_uuid"`
	BootFilesystemUUID *uuid.UUID   `json:"boot_fs_uuid,omitempty"`
	KernelOptions      string       `json:"kernel_opts,omitempty"`
	Legacy             string       `json:"legacy,omitempty"`
	UEFI               *GRUB2UEFI   `json:"uefi,omitempty"`
	SavedEntry         string       `json:"saved_entry,omitempty"`
	Greenboot          bool         `json:"greenboot,omitempty"`
	WriteCmdLine       *bool        `json:"write_cmdline,omitempty"`
	Config             *GRUB2Config `json:"config,omitempty"`
	Ignition           bool         `json:"ignition,omitempty"`
}

type GRUB2UEFI struct {
	Vendor  string `json:"vendor"`
	Install bool   `json:"install,omitempty"`
	Unified bool   `json:"unified,omitempty"`
}

type GRUB2Config struct {
	Default        string   `json:"default,omitempty"`
	TerminalInput  []string `json:"terminal_input,omitempty"`
	TerminalOutput []string `json:"terminal_output,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	Serial         string   `json:"serial,omitempty"`
}

func (GRUB2StageOptions) isStageOptions() {}

// NewGRUB2Stage creates a new GRUB2 stage object.
func NewGRUB2Stage(options *GRUB2StageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.grub2",
		Options: options,
	}
}

func NewGrub2StageOptions(pt *disk.PartitionTable,
	kernelOptions string,
	kernelVer string,
	uefi bool,
	legacy string,
	vendor string,
	install bool) *GRUB2StageOptions {

	rootFs := pt.FindMountable("/")
	if rootFs == nil {
		panic("root filesystem must be defined for grub2 stage, this is a programming error")
	}

	// NB: We need to set the kernel options regardless of whether we are
	// writing the command line to grubenv or not. This is because the kernel
	// options are also written to /etc/default/grub under the GRUB_CMDLINE_LINUX
	// variable. This is used by the 10_linux script executed by grub2-mkconfig
	// to override the kernel options in /etc/kernel/cmdline if the file has
	// older timestamp than /etc/default/grub.
	stageOptions := GRUB2StageOptions{
		RootFilesystemUUID: uuid.MustParse(rootFs.GetFSSpec().UUID),
		Legacy:             legacy,
		KernelOptions:      kernelOptions,
		WriteCmdLine:       common.ToPtr(false),
	}

	bootFs := pt.FindMountable("/boot")
	if bootFs != nil {
		bootFsUUID := uuid.MustParse(bootFs.GetFSSpec().UUID)
		stageOptions.BootFilesystemUUID = &bootFsUUID
	}

	if uefi {
		stageOptions.UEFI = &GRUB2UEFI{
			Vendor:  vendor,
			Install: install,
			Unified: true,
		}
	}

	if kernelVer != "" {
		stageOptions.SavedEntry = "ffffffffffffffffffffffffffffffff-" + kernelVer
		stageOptions.Config = &GRUB2Config{
			Default: "saved",
		}
	}

	return &stageOptions
}
