package main

import (
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"strings"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/pathpolicy"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/bootc-image-builder/bib/internal/buildconfig"
	"github.com/osbuild/bootc-image-builder/bib/internal/distrodef"
	"github.com/osbuild/bootc-image-builder/bib/internal/imagetypes"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
)

// TODO: Auto-detect this from container image metadata
const DEFAULT_SIZE = uint64(10 * GibiByte)

type ManifestConfig struct {
	// OCI image path (without the transport, that is always docker://)
	Imgref string

	ImageTypes imagetypes.ImageTypes

	// Build config
	Config *buildconfig.BuildConfig

	// CPU architecture of the image
	Architecture arch.Arch

	// TLSVerify specifies whether HTTPS and a valid TLS certificate are required
	TLSVerify bool

	// The minimum size required for the root fs in order to fit the container
	// contents
	RootfsMinsize uint64

	// Paths to the directory with the distro definitions
	DistroDefPaths []string

	// Extracted information about the source container image
	SourceInfo *source.Info

	// Command to run the depsolver
	DepsolverCmd []string

	// RootFSType specifies the filesystem type for the root partition
	RootFSType string
}

func Manifest(c *ManifestConfig) (*manifest.Manifest, error) {
	rng := createRand()

	if c.ImageTypes.BuildsISO() {
		return manifestForISO(c, rng)
	}
	return manifestForDiskImage(c, rng)
}

var (
	// The mountpoint policy for bootc images is more restrictive than the
	// ostree mountpoint policy defined in osbuild/images. It only allows /
	// (for sizing the root partition) and custom mountpoints under /var but
	// not /var itself.

	// Since our policy library doesn't support denying a path while allowing
	// its subpaths (only the opposite), we augment the standard policy check
	// with a simple search through the custom mountpoints to deny /var
	// specifically.
	mountpointPolicy = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
		// allow all existing mountpoints (but no subdirs) to support size customizations
		"/":     {Deny: false, Exact: true},
		"/boot": {Deny: false, Exact: true},

		// /var is not allowed, but we need to allow any subdirectories that
		// are not denied below, so we allow it initially and then check it
		// separately (in checkMountpoints())
		"/var": {Deny: false},

		// /var subdir denials
		"/var/home":     {Deny: true},
		"/var/lock":     {Deny: true}, // symlink to ../run/lock which is on tmpfs
		"/var/mail":     {Deny: true}, // symlink to spool/mail
		"/var/mnt":      {Deny: true},
		"/var/roothome": {Deny: true},
		"/var/run":      {Deny: true}, // symlink to ../run which is on tmpfs
		"/var/srv":      {Deny: true},
		"/var/usrlocal": {Deny: true},
	})

	mountpointMinimalPolicy = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
		// allow all existing mountpoints to support size customizations
		"/":     {Deny: false, Exact: true},
		"/boot": {Deny: false, Exact: true},
	})
)

func checkMountpoints(filesystems []blueprint.FilesystemCustomization, policy *pathpolicy.PathPolicies) error {
	errs := []error{}
	for _, fs := range filesystems {
		if err := policy.Check(fs.Mountpoint); err != nil {
			errs = append(errs, err)
		}
		if fs.Mountpoint == "/var" {
			// this error message is consistent with the errors returned by policy.Check()
			// TODO: remove trailing space inside the quoted path when the function is fixed in osbuild/images.
			errs = append(errs, fmt.Errorf(`path "/var" is not allowed`))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("The following errors occurred while validating custom mountpoints:\n%w", errors.Join(errs...))
	}
	return nil
}

func checkFilesystemCustomizations(fsCustomizations []blueprint.FilesystemCustomization, ptmode disk.PartitioningMode) error {
	var policy *pathpolicy.PathPolicies
	switch ptmode {
	case disk.BtrfsPartitioningMode:
		// btrfs subvolumes are not supported at build time yet, so we only
		// allow / and /boot to be customized when building a btrfs disk (the
		// minimal policy)
		policy = mountpointMinimalPolicy
	default:
		policy = mountpointPolicy
	}
	if err := checkMountpoints(fsCustomizations, policy); err != nil {
		return err
	}
	return nil
}

// updateFilesystemSizes updates the size of the root filesystem customization
// based on the minRootSize. The new min size whichever is larger between the
// existing size and the minRootSize. If the root filesystem is not already
// configured, a new customization is added.
func updateFilesystemSizes(fsCustomizations []blueprint.FilesystemCustomization, minRootSize uint64) []blueprint.FilesystemCustomization {
	updated := make([]blueprint.FilesystemCustomization, len(fsCustomizations), len(fsCustomizations)+1)
	hasRoot := false
	for idx, fsc := range fsCustomizations {
		updated[idx] = fsc
		if updated[idx].Mountpoint == "/" {
			updated[idx].MinSize = max(updated[idx].MinSize, minRootSize)
			hasRoot = true
		}
	}

	if !hasRoot {
		// no root customization found: add it
		updated = append(updated, blueprint.FilesystemCustomization{Mountpoint: "/", MinSize: minRootSize})
	}
	return updated
}

// setFSTypes sets the filesystem types for all mountable entities to match the
// selected rootfs type.
// If rootfs is 'btrfs', the function will keep '/boot' to its default.
func setFSTypes(pt *disk.PartitionTable, rootfs string) error {
	if rootfs == "" {
		return fmt.Errorf("root filesystem type is empty")
	}

	return pt.ForEachMountable(func(mnt disk.Mountable, _ []disk.Entity) error {
		switch mnt.GetMountpoint() {
		case "/boot/efi":
			// never change the efi partition's type
			return nil
		case "/boot":
			// change only if we're not doing btrfs
			if rootfs == "btrfs" {
				return nil
			}
			fallthrough
		default:
			switch elem := mnt.(type) {
			case *disk.Filesystem:
				elem.Type = rootfs
			case *disk.BtrfsSubvolume:
				// nothing to do
			default:
				return fmt.Errorf("the mountable disk entity for %q of the base partition table is not an ordinary filesystem but %T", mnt.GetMountpoint(), mnt)
			}
			return nil
		}
	})
}

func genPartitionTable(c *ManifestConfig, customizations *blueprint.Customizations, rng *rand.Rand) (*disk.PartitionTable, error) {
	basept, ok := partitionTables[c.Architecture.String()]
	if !ok {
		return nil, fmt.Errorf("pipelines: no partition tables defined for %s", c.Architecture)
	}

	partitioningMode := disk.RawPartitioningMode
	if c.RootFSType == "btrfs" {
		partitioningMode = disk.BtrfsPartitioningMode
	}
	if err := checkFilesystemCustomizations(customizations.GetFilesystems(), partitioningMode); err != nil {
		return nil, err
	}
	fsCustomizations := updateFilesystemSizes(customizations.GetFilesystems(), c.RootfsMinsize)

	pt, err := disk.NewPartitionTable(&basept, fsCustomizations, DEFAULT_SIZE, partitioningMode, nil, rng)
	if err != nil {
		return nil, err
	}

	if err := setFSTypes(pt, c.RootFSType); err != nil {
		return nil, fmt.Errorf("error setting root filesystem type: %w", err)
	}
	return pt, nil
}

func manifestForDiskImage(c *ManifestConfig, rng *rand.Rand) (*manifest.Manifest, error) {
	if c.Imgref == "" {
		return nil, fmt.Errorf("pipeline: no base image defined")
	}
	containerSource := container.SourceSpec{
		Source:    c.Imgref,
		Name:      c.Imgref,
		TLSVerify: &c.TLSVerify,
		Local:     true,
	}

	var customizations *blueprint.Customizations
	if c.Config != nil {
		customizations = c.Config.Customizations
	}

	img := image.NewBootcDiskImage(containerSource)
	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())
	// TODO: get from the bootc container instead of hardcoding it
	img.SELinux = "targeted"

	img.KernelOptionsAppend = []string{
		"rw",
		// TODO: Drop this as we expect kargs to come from the container image,
		// xref https://github.com/CentOS/centos-bootc-layered/blob/main/cloud/usr/lib/bootc/install/05-cloud-kargs.toml
		"console=tty0",
		"console=ttyS0",
	}

	switch c.Architecture {
	case arch.ARCH_X86_64:
		img.Platform = &platform.X86{
			BasePlatform: platform.BasePlatform{},
			BIOS:         true,
		}
	case arch.ARCH_AARCH64:
		img.Platform = &platform.Aarch64{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				QCOW2Compat: "1.1",
			},
		}
	case arch.ARCH_S390X:
		img.Platform = &platform.S390X{
			BasePlatform: platform.BasePlatform{
				QCOW2Compat: "1.1",
			},
			Zipl: true,
		}
	case arch.ARCH_PPC64LE:
		img.Platform = &platform.PPC64LE{
			BasePlatform: platform.BasePlatform{
				QCOW2Compat: "1.1",
			},
			BIOS: true,
		}
	}

	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.KernelOptionsAppend = append(img.KernelOptionsAppend, kopts.Append)
	}

	pt, err := genPartitionTable(c, customizations, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	// For the bootc-disk image, the filename is the basename and the extension
	// is added automatically for each disk format
	img.Filename = "disk"

	mf := manifest.New()
	mf.Distro = manifest.DISTRO_FEDORA
	runner := &runner.Linux{}

	// Remove the "NewBootcLegacyDiskImage" if part below and
	// *only* use the "else" part of the code once either of the
	// following is available in centos/rhel
	// https://github.com/containers/bootc/pull/462
	// https://www.mail-archive.com/qemu-devel@nongnu.org/msg1034508.html
	if c.Architecture != arch.Current() {
		legacyImg := image.NewBootcLegacyDiskImage(img)
		err = legacyImg.InstantiateManifestFromContainers(&mf, []container.SourceSpec{containerSource}, runner, rng)
	} else {
		err = img.InstantiateManifestFromContainers(&mf, []container.SourceSpec{containerSource}, runner, rng)
	}

	return &mf, err
}

func manifestForISO(c *ManifestConfig, rng *rand.Rand) (*manifest.Manifest, error) {
	if c.Imgref == "" {
		return nil, fmt.Errorf("pipeline: no base image defined")
	}

	imageDef, err := distrodef.LoadImageDef(c.DistroDefPaths, c.SourceInfo.OSRelease.ID, c.SourceInfo.OSRelease.VersionID, "anaconda-iso")
	if err != nil {
		return nil, err
	}

	containerSource := container.SourceSpec{
		Source:    c.Imgref,
		Name:      c.Imgref,
		TLSVerify: &c.TLSVerify,
		Local:     true,
	}

	// The ref is not needed and will be removed from the ctor later
	// in time
	img := image.NewAnacondaContainerInstaller(containerSource, "")
	img.SquashfsCompression = "zstd"

	img.Product = c.SourceInfo.OSRelease.Name
	img.OSVersion = c.SourceInfo.OSRelease.VersionID

	img.ExtraBasePackages = rpmmd.PackageSet{
		Include: imageDef.Packages,
	}

	img.ISOLabel = fmt.Sprintf("Container-Installer-%s", c.Architecture)

	var customizations *blueprint.Customizations
	if c.Config != nil {
		customizations = c.Config.Customizations
	}

	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return nil, err
	}
	img.Kickstart.Path = osbuild.KickstartPathOSBuild
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.Kickstart.KernelOptionsAppend = append(img.Kickstart.KernelOptionsAppend, kopts.Append)
	}
	img.Kickstart.NetworkOnBoot = true

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return nil, err
	}
	if instCust != nil && instCust.Modules != nil {
		img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, instCust.Modules.Enable...)
		img.DisabledAnacondaModules = append(img.DisabledAnacondaModules, instCust.Modules.Disable...)
	}
	img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules,
		anaconda.ModuleUsers,
		anaconda.ModuleServices,
		anaconda.ModuleSecurity,
	)

	img.Kickstart.OSTree = &kickstart.OSTree{
		OSName: "default",
	}
	// use lorax-templates-rhel if the source distro is not Fedora with the exception of Fedora ELN
	img.UseRHELLoraxTemplates =
		c.SourceInfo.OSRelease.ID != "fedora" || c.SourceInfo.OSRelease.VersionID == "eln"

	switch c.Architecture {
	case arch.ARCH_X86_64:
		img.Platform = &platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_ISO,
			},
			BIOS:       true,
			UEFIVendor: c.SourceInfo.UEFIVendor,
		}
	case arch.ARCH_AARCH64:
		// aarch64 always uses UEFI, so let's enforce the vendor
		if c.SourceInfo.UEFIVendor == "" {
			return nil, fmt.Errorf("UEFI vendor must be set for aarch64 ISO")
		}
		img.Platform = &platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_ISO,
			},
			UEFIVendor: c.SourceInfo.UEFIVendor,
		}
	}

	img.Filename = "install.iso"

	mf := manifest.New()

	foundDistro, foundRunner, err := getDistroAndRunner(c.SourceInfo.OSRelease)
	if err != nil {
		return nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifest(&mf, nil, foundRunner, rng)
	return &mf, err
}

func getDistroAndRunner(osRelease source.OSRelease) (manifest.Distro, runner.Runner, error) {
	switch osRelease.ID {
	case "fedora":
		version, err := strconv.ParseUint(osRelease.VersionID, 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse Fedora version (%s): %w", osRelease.VersionID, err)
		}

		return manifest.DISTRO_FEDORA, &runner.Fedora{
			Version: version,
		}, nil
	case "centos":
		version, err := strconv.ParseUint(osRelease.VersionID, 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse CentOS version (%s): %w", osRelease.VersionID, err)
		}
		r := &runner.CentOS{
			Version: version,
		}
		switch version {
		case 9:
			return manifest.DISTRO_EL9, r, nil
		case 10:
			return manifest.DISTRO_EL10, r, nil
		default:
			logrus.Warnf("Unknown CentOS version %d, using default distro for manifest generation", version)
			return manifest.DISTRO_NULL, r, nil
		}

	case "rhel":
		versionParts := strings.Split(osRelease.VersionID, ".")
		if len(versionParts) != 2 {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("invalid RHEL version format: %s", osRelease.VersionID)
		}
		major, err := strconv.ParseUint(versionParts[0], 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse RHEL major version (%s): %w", versionParts[0], err)
		}
		minor, err := strconv.ParseUint(versionParts[1], 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse RHEL minor version (%s): %w", versionParts[1], err)
		}
		r := &runner.RHEL{
			Major: major,
			Minor: minor,
		}
		switch major {
		case 9:
			return manifest.DISTRO_EL9, r, nil
		case 10:
			return manifest.DISTRO_EL10, r, nil
		default:
			logrus.Warnf("Unknown RHEL version %d, using default distro for manifest generation", major)
			return manifest.DISTRO_NULL, r, nil
		}
	}

	logrus.Warnf("Unknown distro %s, using default runner", osRelease.ID)
	return manifest.DISTRO_NULL, &runner.Linux{}, nil
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
