package main

import (
	cryptorand "crypto/rand"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"strings"

	"github.com/osbuild/bootc-image-builder/bib/internal/buildconfig"
	"github.com/osbuild/bootc-image-builder/bib/internal/distrodef"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
	"github.com/sirupsen/logrus"
)

// TODO: Auto-detect this from container image metadata
const DEFAULT_SIZE = uint64(10 * GibiByte)

type ManifestConfig struct {
	// OCI image path (without the transport, that is always docker://)
	Imgref string

	BuildType BuildType

	// Build config
	Config *buildconfig.BuildConfig

	// CPU architecture of the image
	Architecture arch.Arch

	// TLSVerify specifies whether HTTPS and a valid TLS certificate are required
	TLSVerify bool

	// Only the "/" filesystem size is configured here right now
	Filesystems []blueprint.FilesystemCustomization

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

	switch c.BuildType {
	case BuildTypeDisk:
		return manifestForDiskImage(c, rng)
	case BuildTypeISO:
		return manifestForISO(c, rng)
	default:
		return nil, fmt.Errorf("Manifest(): unknown build type %d", c.BuildType)
	}
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
	}

	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.KernelOptionsAppend = append(img.KernelOptionsAppend, kopts.Append)
	}

	basept, ok := partitionTables[c.Architecture.String()]
	if !ok {
		return nil, fmt.Errorf("pipelines: no partition tables defined for %s", c.Architecture)
	}
	// XXX: extract into helper
	if c.RootFSType != "" {
		rootMountable := basept.FindMountable("/")
		if rootMountable == nil {
			return nil, fmt.Errorf("no root filesystem defined in the partition table")
		}
		rootFS, isFS := rootMountable.(*disk.Filesystem)
		if !isFS {
			return nil, fmt.Errorf("root mountable is not an ordinary filesystem (btrfs is not yet supported)")
		}
		rootFS.Type = c.RootFSType
	}

	pt, err := disk.NewPartitionTable(&basept, c.Filesystems, DEFAULT_SIZE, disk.RawPartitioningMode, nil, rng)
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

	imageDef, err := distrodef.LoadImageDef(c.DistroDefPaths, c.SourceInfo.OSRelease.ID, "anaconda-iso")
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

	img.ExtraBasePackages = rpmmd.PackageSet{
		Include: imageDef.Packages,
	}

	img.ISOLabel = fmt.Sprintf("Container-Installer-%s", c.Architecture)

	var customizations *blueprint.Customizations
	if c.Config != nil {
		customizations = c.Config.Customizations
	}

	img.Kickstart = &kickstart.Options{
		// XXX: this really should be the default in "images"
		Path:   osbuild.KickstartPathOSBuild,
		Users:  users.UsersFromBP(customizations.GetUsers()),
		Groups: users.GroupsFromBP(customizations.GetGroups()),
	}
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.Kickstart.KernelOptionsAppend = append(img.Kickstart.KernelOptionsAppend, kopts.Append)
	}
	img.Kickstart.NetworkOnBoot = true
	// XXX: this should really be done by images, the consumer should not
	// need to know these details. so once images is fixed drop it here
	// again.
	if len(img.Kickstart.Users) > 0 || len(img.Kickstart.Groups) > 0 {
		img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, "org.fedoraproject.Anaconda.Modules.Users")
	}

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

	img.Kickstart.OSTree = &kickstart.OSTree{
		OSName: "default",
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
