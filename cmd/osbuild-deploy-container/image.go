package main

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/users"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func Manifest(imageref string, config *BuildConfig, repos []rpmmd.RepoConfig, arch string, seed int64) (*manifest.Manifest, error) {

	source := rand.NewSource(seed)

	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	baseImage := &ostree.ImageOptions{
		Container: imageref,
		TLSVerify: common.ToPtr(true),
	}

	img, err := pipelines(baseImage, config, arch, rng)
	if err != nil {
		fail(err.Error())
	}
	mf := manifest.New()
	mf.Distro = manifest.DISTRO_FEDORA
	runner := &runner.Fedora{Version: 39}
	_, err = img.InstantiateManifest(&mf, repos, runner, rng)

	return &mf, err
}

func pipelines(baseImage *ostree.ImageOptions, config *BuildConfig, arch string, rng *rand.Rand) (image.ImageKind, error) {
	if baseImage == nil {
		fail("pipeline: no base image defined")
	}
	ref := "ostree/1/1/0"
	containerSource := container.SourceSpec{
		Source:    baseImage.Container,
		Name:      baseImage.Container,
		TLSVerify: baseImage.TLSVerify,
	}

	img := image.NewOSTreeContainerDiskImage(containerSource, ref)

	var customizations *blueprint.Customizations
	if config != nil && config.Blueprint != nil {
		customizations = config.Blueprint.Customizations
	}
	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())

	img.KernelOptionsAppend = []string{
		"rw",
		"console=tty0",
		"console=ttyS0",
	}

	img.SysrootReadOnly = true

	switch arch {
	case platform.ARCH_X86_64.String():
		img.Platform = &platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
			BIOS:       true,
			UEFIVendor: "fedora",
		}
	case platform.ARCH_AARCH64.String():
		img.Platform = &platform.Aarch64{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		}
	}

	img.OSName = "default"

	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.KernelOptionsAppend = append(img.KernelOptionsAppend, kopts.Append)
	}

	img.Workload = &workload.Custom{}

	basept, ok := partitionTables[arch]
	if !ok {
		fail(fmt.Sprintf("pipelines: no partition tables defined for %s", arch))
	}
	size := uint64(10 * common.GibiByte)
	pt, err := disk.NewPartitionTable(&basept, nil, size, disk.RawPartitioningMode, nil, rng)
	check(err)
	img.PartitionTable = pt

	img.Filename = "disk.qcow2"

	return img, nil
}
