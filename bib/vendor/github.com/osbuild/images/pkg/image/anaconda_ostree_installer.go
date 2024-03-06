package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type AnacondaOSTreeInstaller struct {
	Base
	Platform          platform.Platform
	ExtraBasePackages rpmmd.PackageSet
	Users             []users.User
	Groups            []users.Group

	Language *string
	Keyboard *string
	Timezone *string

	// Create a sudoers drop-in file for wheel group with NOPASSWD option
	WheelNoPasswd bool

	// Add kickstart options to make the installation fully unattended
	UnattendedKickstart bool

	SquashfsCompression string

	ISOLabelTempl string
	Product       string
	Variant       string
	OSName        string
	OSVersion     string
	Release       string
	Remote        string

	Commit ostree.SourceSpec

	Filename string

	AdditionalDracutModules   []string
	AdditionalAnacondaModules []string
	AdditionalDrivers         []string
	FIPS                      bool
}

func NewAnacondaOSTreeInstaller(commit ostree.SourceSpec) *AnacondaOSTreeInstaller {
	return &AnacondaOSTreeInstaller{
		Base:   NewBase("ostree-installer"),
		Commit: commit,
	}
}

func (img *AnacondaOSTreeInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	anacondaPipeline := manifest.NewAnacondaInstaller(
		manifest.AnacondaInstallerTypePayload,
		buildPipeline,
		img.Platform,
		repos,
		"kernel",
		img.Product,
		img.OSVersion,
	)
	anacondaPipeline.ExtraPackages = img.ExtraBasePackages.Include
	anacondaPipeline.ExcludePackages = img.ExtraBasePackages.Exclude
	anacondaPipeline.ExtraRepos = img.ExtraBasePackages.Repositories
	anacondaPipeline.Users = img.Users
	anacondaPipeline.Groups = img.Groups
	anacondaPipeline.Variant = img.Variant
	anacondaPipeline.Biosdevname = (img.Platform.GetArch() == arch.ARCH_X86_64)
	anacondaPipeline.Checkpoint()
	anacondaPipeline.AdditionalDracutModules = img.AdditionalDracutModules
	anacondaPipeline.AdditionalAnacondaModules = img.AdditionalAnacondaModules
	if img.FIPS {
		anacondaPipeline.AdditionalAnacondaModules = append(
			anacondaPipeline.AdditionalAnacondaModules,
			"org.fedoraproject.Anaconda.Modules.Security",
		)
	}
	anacondaPipeline.AdditionalDrivers = img.AdditionalDrivers

	// TODO: replace isoLabelTmpl with more high-level properties
	isoLabel := fmt.Sprintf(img.ISOLabelTempl, img.Platform.GetArch())

	rootfsImagePipeline := manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
	rootfsImagePipeline.Size = 4 * common.GibiByte

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.Product, img.OSVersion)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = isoLabel

	kspath := osbuild.KickstartPathOSBuild
	bootTreePipeline.KernelOpts = []string{fmt.Sprintf("inst.stage2=hd:LABEL=%s", isoLabel), fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", isoLabel, kspath)}
	if img.FIPS {
		bootTreePipeline.KernelOpts = append(bootTreePipeline.KernelOpts, "fips=1")
	}

	// enable ISOLinux on x86_64 only
	isoLinuxEnabled := img.Platform.GetArch() == arch.ARCH_X86_64

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.Release
	isoTreePipeline.OSName = img.OSName
	isoTreePipeline.Remote = img.Remote
	isoTreePipeline.Users = img.Users
	isoTreePipeline.Groups = img.Groups
	isoTreePipeline.WheelNoPasswd = img.WheelNoPasswd
	isoTreePipeline.UnattendedKickstart = img.UnattendedKickstart
	isoTreePipeline.SquashfsCompression = img.SquashfsCompression
	isoTreePipeline.Language = img.Language
	isoTreePipeline.Keyboard = img.Keyboard
	isoTreePipeline.Timezone = img.Timezone

	// For ostree installers, always put the kickstart file in the root of the ISO
	isoTreePipeline.KSPath = kspath
	isoTreePipeline.PayloadPath = "/ostree/repo"

	isoTreePipeline.OSTreeCommitSource = &img.Commit
	isoTreePipeline.ISOLinux = isoLinuxEnabled
	if img.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, isoLabel)
	isoPipeline.SetFilename(img.Filename)
	isoPipeline.ISOLinux = isoLinuxEnabled
	artifact := isoPipeline.Export()

	return artifact, nil
}
