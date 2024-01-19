package main

import (
	cryptorand "crypto/rand"
	"fmt"
	"math"
	"math/big"
	"math/rand"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

// TODO: Auto-detect this from container image metadata
const DEFAULT_SIZE = uint64(10 * GibiByte)

type ManifestConfig struct {
	// OCI image path (without the transport, that is always docker://)
	Imgref string

	// Image type to build (currently: qcow2, ami)
	//
	// TODO: Make this an enum.
	ImgType string

	// Build config
	Config *BuildConfig

	// Repositories for a buildroot (or an installer tree in the future)
	Repos []rpmmd.RepoConfig

	// CPU architecture of the image
	Architecture arch.Arch

	// TLSVerify specifies whether HTTPS and a valid TLS certificate are required
	TLSVerify bool
}

func Manifest(c *ManifestConfig) (*manifest.Manifest, error) {
	rng := createRand()

	var img image.ImageKind
	var err error

	switch c.ImgType {
	case "ami", "qcow2", "raw":
		img, err = pipelinesForDiskImage(c, rng)
	case "iso":
		img, err = pipelinesForISO(c, rng)
	default:
		return nil, fmt.Errorf("Manifest(): unsupported image type %q", c.ImgType)
	}

	if err != nil {
		return nil, err
	}

	mf := manifest.New()
	mf.Distro = manifest.DISTRO_FEDORA
	runner := &runner.Fedora{Version: 39}
	_, err = img.InstantiateManifest(&mf, c.Repos, runner, rng)

	return &mf, err
}

func pipelinesForDiskImage(c *ManifestConfig, rng *rand.Rand) (image.ImageKind, error) {
	if c.Imgref == "" {
		return nil, fmt.Errorf("pipeline: no base image defined")
	}
	ref := "ostree/1/1/0"
	containerSource := container.SourceSpec{
		Source:    c.Imgref,
		Name:      c.Imgref,
		TLSVerify: &c.TLSVerify,
	}

	img := image.NewOSTreeDiskImageFromContainer(containerSource, ref)
	img.ContainerBuildable = true

	var customizations *blueprint.Customizations
	if c.Config != nil && c.Config.Blueprint != nil {
		customizations = c.Config.Blueprint.Customizations
	}

	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())

	img.KernelOptionsAppend = []string{
		"rw",
		// TODO: Drop this as we expect kargs to come from the container image,
		// xref https://github.com/CentOS/centos-bootc-layered/blob/main/cloud/usr/lib/bootc/install/05-cloud-kargs.toml
		"console=tty0",
		"console=ttyS0",
	}

	img.SysrootReadOnly = true

	var imageFormat platform.ImageFormat
	var filename string
	switch c.ImgType {
	case "qcow2":
		imageFormat = platform.FORMAT_QCOW2
		filename = "disk.qcow2"
	case "ami", "raw":
		imageFormat = platform.FORMAT_RAW
		filename = "disk.raw"
	}

	switch c.Architecture {
	case arch.ARCH_X86_64:
		img.Platform = &platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: imageFormat,
			},
			BIOS:       true,
			UEFIVendor: "fedora",
		}
	case arch.ARCH_AARCH64:
		img.Platform = &platform.Aarch64{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: imageFormat,
				QCOW2Compat: "1.1",
			},
		}
	}

	img.OSName = "default"

	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.KernelOptionsAppend = append(img.KernelOptionsAppend, kopts.Append)
	}

	img.Workload = &NullWorkload{}

	basept, ok := partitionTables[c.Architecture.String()]
	if !ok {
		return nil, fmt.Errorf("pipelines: no partition tables defined for %s", c.Architecture)
	}
	pt, err := disk.NewPartitionTable(&basept, nil, DEFAULT_SIZE, disk.RawPartitioningMode, nil, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = filename

	return img, nil
}

func pipelinesForISO(c *ManifestConfig, rng *rand.Rand) (image.ImageKind, error) {
	if c.Imgref == "" {
		return nil, fmt.Errorf("pipeline: no base image defined")
	}

	containerSource := container.SourceSpec{
		Source:    c.Imgref,
		Name:      c.Imgref,
		TLSVerify: &c.TLSVerify,
	}

	// The ref is not needed and will be removed from the ctor later
	// in time
	img := image.NewAnacondaContainerInstaller(containerSource, "")
	img.SquashfsCompression = "zstd"

	// TODO: Parametrize me!
	img.Product = "Fedora"

	img.ExtraBasePackages = rpmmd.PackageSet{
		Include: []string{
			"aajohan-comfortaa-fonts",
			"abattis-cantarell-fonts",
			"alsa-firmware",
			"alsa-tools-firmware",
			"anaconda",
			"anaconda-dracut",
			"anaconda-install-env-deps",
			"anaconda-widgets",
			"atheros-firmware",
			"audit",
			"bind-utils",
			"bitmap-fangsongti-fonts",
			"brcmfmac-firmware",
			"bzip2",
			"cryptsetup",
			"curl",
			"dbus-x11",
			"dejavu-sans-fonts",
			"dejavu-sans-mono-fonts",
			"device-mapper-persistent-data",
			"dmidecode",
			"dnf",
			"dracut-config-generic",
			"dracut-network",
			"efibootmgr",
			"ethtool",
			"fcoe-utils",
			"ftp",
			"gdb-gdbserver",
			"gdisk",
			"glibc-all-langpacks",
			"gnome-kiosk",
			"google-noto-sans-cjk-ttc-fonts",
			"grub2-tools",
			"grub2-tools-extra",
			"grub2-tools-minimal",
			"grubby",
			"gsettings-desktop-schemas",
			"hdparm",
			"hexedit",
			"hostname",
			"initscripts",
			"ipmitool",
			"iwlwifi-dvm-firmware",
			"iwlwifi-mvm-firmware",
			"jomolhari-fonts",
			"kbd",
			"kbd-misc",
			"kdump-anaconda-addon",
			"kernel",
			"khmeros-base-fonts",
			"less",
			"libblockdev-lvm-dbus",
			"libibverbs",
			"libreport-plugin-bugzilla",
			"libreport-plugin-reportuploader",
			"librsvg2",
			"linux-firmware",
			"lldpad",
			"lsof",
			"madan-fonts",
			"mt-st",
			"mtr",
			"net-tools",
			"nfs-utils",
			"nm-connection-editor",
			"nmap-ncat",
			"nss-tools",
			"openssh-clients",
			"openssh-server",
			"ostree",
			"pciutils",
			"perl-interpreter",
			"pigz",
			"plymouth",
			"python3-pyatspi",
			"rdma-core",
			"realtek-firmware",
			"rit-meera-new-fonts",
			"rng-tools",
			"rpcbind",
			"rpm-ostree",
			"rsync",
			"rsyslog",
			"selinux-policy-targeted",
			"sg3_utils",
			"sil-abyssinica-fonts",
			"sil-padauk-fonts",
			"smartmontools",
			"spice-vdagent",
			"strace",
			"systemd",
			"tar",
			"tigervnc-server-minimal",
			"tigervnc-server-module",
			"udisks2",
			"udisks2-iscsi",
			"usbutils",
			"vim-minimal",
			"volume_key",
			"wget",
			"xfsdump",
			"xfsprogs",
			"xorg-x11-drivers",
			"xorg-x11-fonts-misc",
			"xorg-x11-server-Xorg",
			"xorg-x11-xauth",
			"xrdb",
			"xz",
		},
	}

	img.ISOLabelTempl = "Container-Installer-%s"

	var customizations *blueprint.Customizations
	if c.Config != nil && c.Config.Blueprint != nil {
		customizations = c.Config.Blueprint.Customizations
	}

	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())

	switch c.Architecture {
	case arch.ARCH_X86_64:
		img.Platform = &platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_ISO,
			},
			BIOS:       true,
			UEFIVendor: "fedora",
		}
	case arch.ARCH_AARCH64:
		img.Platform = &platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_ISO,
			},
			UEFIVendor: "fedora",
		}
	}

	img.OSName = "default"
	img.Filename = "install.iso"

	return img, nil
}

func createRand() *rand.Rand {
	seed, err := cryptorand.Int(cryptorand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		panic("Cannot generate an RNG seed.")
	}

	// math/rand is good enough in this case
	/* #nosec G404 */
	return rand.New(rand.NewSource(seed.Int64()))
}
