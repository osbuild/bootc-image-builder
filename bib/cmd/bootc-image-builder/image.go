package main

import (
	cryptorand "crypto/rand"
	"fmt"
	"math"
	"math/big"
	"math/rand"

	"github.com/osbuild/bootc-image-builder/bib/internal/distrodef"
	"github.com/osbuild/bootc-image-builder/bib/internal/source"
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

	BuildType BuildType

	// Build config
	Config *BuildConfig

	// CPU architecture of the image
	Architecture arch.Arch

	// TLSVerify specifies whether HTTPS and a valid TLS certificate are required
	TLSVerify bool

	// Only the "/" filesystem size is configured here right now
	Filesystems []blueprint.FilesystemCustomization

	// Paths to the directory with the distro definitions
	DistroDefPaths []string

	// Extracted information about the source container image
	Info *source.Info

	// Command to run the depsolver
	DepsolverCmd []string
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
	if c.Config != nil && c.Config.Blueprint != nil {
		customizations = c.Config.Blueprint.Customizations
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

	imageDef, err := distrodef.LoadImageDef(c.DistroDefPaths, c.Info.ID, "anaconda-iso")
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

	img.Product = c.Info.Name

	img.ExtraBasePackages = rpmmd.PackageSet{
		Include: imageDef.Packages,
	}

	img.ISOLabel = fmt.Sprintf("Container-Installer-%s", c.Architecture)

	var customizations *blueprint.Customizations
	if c.Config != nil && c.Config.Blueprint != nil {
		customizations = c.Config.Blueprint.Customizations
	}

	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.KickstartKernelOptionsAppend = append(img.KickstartKernelOptionsAppend, kopts.Append)
	}
	img.KickstartNetworkOnBoot = true
	// XXX: this should really be done by images, the consumer should not
	// need to know these details. so once images is fixed drop it here
	// again.
	if len(img.Users) > 0 || len(img.Groups) > 0 {
		img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, "org.fedoraproject.Anaconda.Modules.Users")
	}

	switch c.Architecture {
	case arch.ARCH_X86_64:
		img.Platform = &platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_ISO,
			},
			BIOS:       true,
			UEFIVendor: c.Info.UEFIVendor,
		}
	case arch.ARCH_AARCH64:
		// aarch64 always uses UEFI, so let's enforce the vendor
		if c.Info.UEFIVendor == "" {
			return nil, fmt.Errorf("UEFI vendor must be set for aarch64 ISO")
		}
		img.Platform = &platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_ISO,
			},
			UEFIVendor: c.Info.UEFIVendor,
		}
	}

	img.OSName = "default"
	img.Filename = "install.iso"

	mf := manifest.New()
	// The following two lines are slightly hacky, but converting os-release
	// into these "enums" cannot be done generically, so let's use use the generic
	// options, and rely on tests to catch any issues.
	mf.Distro = manifest.DISTRO_NULL
	runner := &runner.Linux{}
	_, err = img.InstantiateManifest(&mf, nil, runner, rng)
	return &mf, err
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
