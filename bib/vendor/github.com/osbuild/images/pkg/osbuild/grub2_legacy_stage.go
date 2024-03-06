package osbuild

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
)

type GRUB2FSDesc struct {
	Device string     `json:"device,omitempty"`
	Label  string     `json:"label,omitempty"`
	UUID   *uuid.UUID `json:"uuid,omitempty"`
}

func (d GRUB2FSDesc) validate() error {

	have := make([]string, 0, 3)
	if d.Device != "" {
		have = append(have, "`device`")
	}

	if d.Label != "" {
		have = append(have, "`label`")
	}

	if d.UUID != nil {
		have = append(have, "`uuid`")
	}

	count := len(have)
	if count == 0 {
		return fmt.Errorf("need `device`, `label`, or `uuid`")
	} else if count > 1 {
		return fmt.Errorf("must only specify one of %s", strings.Join(have, ", "))
	}

	return nil
}

type GRUB2Product struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Nick    string `json:"nick,omitempty"`
}

func (p GRUB2Product) validate() error {
	if p.Name == "" {
		return fmt.Errorf("need `Name`")
	}
	if p.Version == "" {
		return fmt.Errorf("need `Version`")
	}
	return nil
}

type GRUB2MenuEntry struct {
	Default *bool        `json:"default,omitempty"`
	Id      string       `json:"id,omitempty"`
	Kernel  string       `json:"kernel,omitempty"`
	Product GRUB2Product `json:"product,omitempty"`
}

func (e GRUB2MenuEntry) validate() (err error) {
	if e.Id == "" {
		return fmt.Errorf("need `Id`")
	}
	if e.Kernel == "" {
		return fmt.Errorf("need `Kernel`")
	}
	if err = e.Product.validate(); err != nil {
		return fmt.Errorf("`Product` error: %w", err)
	}
	return nil
}

type GRUB2BIOS struct {
	Platform string `json:"platform,"`
}

type GRUB2LegacyConfig struct {
	GRUB2Config
	CmdLine     string `json:"cmdline,omitempty"`
	Distributor string `json:"distributor,omitempty"`
}

type GRUB2LegacyStageOptions struct {
	// Required
	RootFS  GRUB2FSDesc      `json:"rootfs"`
	Entries []GRUB2MenuEntry `json:"entries"`

	// One of
	BIOS *GRUB2BIOS `json:"bios,omitempty"`
	UEFI *GRUB2UEFI `json:"uefi,omitempty"`

	// Optional
	BootFS        *GRUB2FSDesc       `json:"bootfs,omitempty"`
	WriteDefaults *bool              `json:"write_defaults,omitempty"`
	Config        *GRUB2LegacyConfig `json:"config,omitempty"`
}

func (GRUB2LegacyStageOptions) isStageOptions() {}

func MakeGrub2MenuEntries(id string, kernelVer string, product GRUB2Product, rescue bool) []GRUB2MenuEntry {
	entries := []GRUB2MenuEntry{
		{
			Default: common.ToPtr(true),
			Id:      id,
			Product: product,
			Kernel:  kernelVer,
		},
	}

	if rescue {
		entry := GRUB2MenuEntry{
			Id:      id,
			Product: product,
			Kernel:  "0-rescue-ffffffffffffffffffffffffffffffff",
		}
		entries = append(entries, entry)
	}

	return entries
}

func NewGrub2LegacyStageOptions(cfg *GRUB2Config,
	pt *disk.PartitionTable,
	kernelOptions []string,
	legacy string,
	uefi string,
	entries []GRUB2MenuEntry) *GRUB2LegacyStageOptions {

	rootFs := pt.FindMountable("/")
	if rootFs == nil {
		panic("root filesystem must be defined for grub2 stage, this is a programming error")
	}

	kopts := strings.Join(kernelOptions, " ")

	rootFsUUID := uuid.MustParse(rootFs.GetFSSpec().UUID)
	stageOptions := GRUB2LegacyStageOptions{
		RootFS:  GRUB2FSDesc{UUID: &rootFsUUID},
		Entries: entries,
		Config: &GRUB2LegacyConfig{
			CmdLine:     kopts,
			Distributor: "$(sed 's, release .*$,,g' /etc/system-release)",
		},
	}

	if cfg != nil {
		stageOptions.Config.GRUB2Config = *cfg
	}

	bootFs := pt.FindMountable("/boot")
	if bootFs != nil {
		bootFsUUID := uuid.MustParse(bootFs.GetFSSpec().UUID)
		stageOptions.BootFS = &GRUB2FSDesc{UUID: &bootFsUUID}
	}

	if legacy != "" {
		stageOptions.BIOS = &GRUB2BIOS{
			Platform: legacy,
		}
	}

	if uefi != "" {
		stageOptions.UEFI = &GRUB2UEFI{
			Vendor: uefi,
		}
	}

	return &stageOptions
}

func (o GRUB2LegacyStageOptions) validate() error {
	// Check we have the required options
	err := o.RootFS.validate()
	if err != nil {
		return fmt.Errorf("`rootfs` error: %w", err)
	}

	if o.BIOS == nil && o.UEFI == nil {
		return fmt.Errorf("need `BIOS` or `UEFI`")
	}

	if o.BIOS != nil && o.BIOS.Platform == "" {
		return fmt.Errorf("need `BIOS.Platform`")
	}

	if o.UEFI != nil && o.UEFI.Vendor == "" {
		return fmt.Errorf("need `UEFI.Vendor`")
	}

	if len(o.Entries) == 0 {
		return fmt.Errorf("at least one entry is required")
	}

	for i, entry := range o.Entries {
		if err = entry.validate(); err != nil {
			return fmt.Errorf("menu entry %d: %w", i, err)
		}
	}

	// check optional arguments
	if o.BootFS != nil {
		err = o.BootFS.validate()
		if err != nil {
			return fmt.Errorf("`bootfs` error: %w", err)
		}
	}

	return nil
}

func NewGrub2LegacyStage(options *GRUB2LegacyStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(fmt.Errorf("grub2.legacy validation failed: %w", err))
	}

	return &Stage{
		Type:    "org.osbuild.grub2.legacy",
		Options: options,
	}
}
