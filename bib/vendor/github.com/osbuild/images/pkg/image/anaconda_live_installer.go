package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type AnacondaLiveInstaller struct {
	Base
	Platform    platform.Platform
	Environment environment.Environment
	Workload    workload.Workload

	ExtraBasePackages rpmmd.PackageSet

	ISOLabelTempl string
	Product       string
	Variant       string
	OSName        string
	OSVersion     string
	Release       string

	Filename string

	AdditionalKernelOpts []string
}

func NewAnacondaLiveInstaller() *AnacondaLiveInstaller {
	return &AnacondaLiveInstaller{
		Base: NewBase("live-installer"),
	}
}

func (img *AnacondaLiveInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	livePipeline := manifest.NewAnacondaInstaller(
		manifest.AnacondaInstallerTypeLive,
		buildPipeline,
		img.Platform,
		repos,
		"kernel",
		img.Product,
		img.OSVersion,
	)

	livePipeline.ExtraPackages = img.ExtraBasePackages.Include
	livePipeline.ExcludePackages = img.ExtraBasePackages.Exclude

	livePipeline.Variant = img.Variant
	livePipeline.Biosdevname = (img.Platform.GetArch() == arch.ARCH_X86_64)

	livePipeline.Checkpoint()

	// TODO: replace isoLabelTmpl with more high-level properties
	isoLabel := fmt.Sprintf(img.ISOLabelTempl, img.Platform.GetArch())

	rootfsImagePipeline := manifest.NewISORootfsImg(buildPipeline, livePipeline)
	rootfsImagePipeline.Size = 8 * common.GibiByte

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.Product, img.OSVersion)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = isoLabel

	kernelOpts := []string{
		fmt.Sprintf("root=live:CDLABEL=%s", isoLabel),
		"rd.live.image",
		"quiet",
		"rhgb",
	}

	kernelOpts = append(kernelOpts, img.AdditionalKernelOpts...)

	bootTreePipeline.KernelOpts = kernelOpts

	// enable ISOLinux on x86_64 only
	isoLinuxEnabled := img.Platform.GetArch() == arch.ARCH_X86_64

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, livePipeline, rootfsImagePipeline, bootTreePipeline)
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.Release
	isoTreePipeline.OSName = img.OSName

	isoTreePipeline.KernelOpts = kernelOpts
	isoTreePipeline.ISOLinux = isoLinuxEnabled

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, isoLabel)
	isoPipeline.SetFilename(img.Filename)
	isoPipeline.ISOLinux = isoLinuxEnabled

	artifact := isoPipeline.Export()

	return artifact, nil
}
