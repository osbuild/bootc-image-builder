package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type OSTreeDiskImage struct {
	Base

	Platform       platform.Platform
	Workload       workload.Workload
	PartitionTable *disk.PartitionTable

	Users  []users.User
	Groups []users.Group

	CommitSource    *ostree.SourceSpec
	ContainerSource *container.SourceSpec

	SysrootReadOnly bool

	Remote ostree.Remote
	OSName string
	Ref    string

	KernelOptionsAppend []string
	Keyboard            string
	Locale              string

	Filename string

	IgnitionPlatform string
	Compression      string

	Directories []*fsnode.Directory
	Files       []*fsnode.File

	FIPS bool

	// Lock the root account in the deployment unless the user defined root
	// user options in the build configuration.
	LockRoot bool

	// Container buildable tweaks the buildroot to be container friendly,
	// i.e. to not rely on an installed osbuild-selinux
	ContainerBuildable bool
}

func NewOSTreeDiskImageFromCommit(commit ostree.SourceSpec) *OSTreeDiskImage {
	return &OSTreeDiskImage{
		Base:         NewBase("ostree-raw-image"),
		CommitSource: &commit,
	}
}

func NewOSTreeDiskImageFromContainer(container container.SourceSpec, ref string) *OSTreeDiskImage {
	return &OSTreeDiskImage{
		Base:            NewBase("ostree-raw-image"),
		ContainerSource: &container,
		Ref:             ref,
	}
}

type baseRawOstreeImageOpts struct {
	useBootupd bool
}

func baseRawOstreeImage(img *OSTreeDiskImage, buildPipeline manifest.Build, opts *baseRawOstreeImageOpts) *manifest.RawOSTreeImage {
	if opts == nil {
		opts = &baseRawOstreeImageOpts{}
	}

	var osPipeline *manifest.OSTreeDeployment
	switch {
	case img.CommitSource != nil:
		osPipeline = manifest.NewOSTreeCommitDeployment(buildPipeline, img.CommitSource, img.OSName, img.Platform)
	case img.ContainerSource != nil:
		osPipeline = manifest.NewOSTreeContainerDeployment(buildPipeline, img.ContainerSource, img.Ref, img.OSName, img.Platform)
	default:
		panic("no content source defined for ostree image")
	}

	osPipeline.PartitionTable = img.PartitionTable
	osPipeline.Remote = img.Remote
	osPipeline.KernelOptionsAppend = img.KernelOptionsAppend
	osPipeline.Keyboard = img.Keyboard
	osPipeline.Locale = img.Locale
	osPipeline.Users = img.Users
	osPipeline.Groups = img.Groups
	osPipeline.SysrootReadOnly = img.SysrootReadOnly
	osPipeline.Directories = img.Directories
	osPipeline.Files = img.Files
	osPipeline.FIPS = img.FIPS
	osPipeline.IgnitionPlatform = img.IgnitionPlatform
	osPipeline.LockRoot = img.LockRoot
	osPipeline.UseBootupd = opts.useBootupd

	// other image types (e.g. live) pass the workload to the pipeline.
	if img.Workload != nil {
		osPipeline.EnabledServices = img.Workload.GetServices()
		osPipeline.DisabledServices = img.Workload.GetDisabledServices()
	}
	return manifest.NewRawOStreeImage(buildPipeline, osPipeline, img.Platform)
}

// replaced in testing
var manifestNewBuild = manifest.NewBuild

func (img *OSTreeDiskImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifestNewBuild(m, runner, repos, &manifest.BuildOptions{ContainerBuildable: img.ContainerBuildable})
	buildPipeline.Checkpoint()

	// don't support compressing non-raw images
	imgFormat := img.Platform.GetImageFormat()
	if imgFormat == platform.FORMAT_UNSET {
		// treat unset as raw for this check
		imgFormat = platform.FORMAT_RAW
	}
	if imgFormat != platform.FORMAT_RAW && img.Compression != "" {
		panic(fmt.Sprintf("no compression is allowed with %q format for %q", imgFormat, img.name))
	}

	baseImage := baseRawOstreeImage(img, buildPipeline, nil)
	switch img.Platform.GetImageFormat() {
	case platform.FORMAT_VMDK:
		vmdkPipeline := manifest.NewVMDK(buildPipeline, baseImage)
		vmdkPipeline.SetFilename(img.Filename)
		return vmdkPipeline.Export(), nil
	case platform.FORMAT_QCOW2:
		qcow2Pipeline := manifest.NewQCOW2(buildPipeline, baseImage)
		qcow2Pipeline.Compat = img.Platform.GetQCOW2Compat()
		qcow2Pipeline.SetFilename(img.Filename)
		return qcow2Pipeline.Export(), nil
	default:
		switch img.Compression {
		case "xz":
			compressedImage := manifest.NewXZ(buildPipeline, baseImage)
			compressedImage.SetFilename(img.Filename)
			return compressedImage.Export(), nil
		case "":
			baseImage.SetFilename(img.Filename)
			return baseImage.Export(), nil
		default:
			panic(fmt.Sprintf("unsupported compression type %q on %q", img.Compression, img.name))
		}
	}
}
