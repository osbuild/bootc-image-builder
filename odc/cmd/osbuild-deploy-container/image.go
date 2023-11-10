package main

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func Manifest(imgref string, config *BuildConfig, repos []rpmmd.RepoConfig, architecture arch.Arch, seed int64) (*manifest.Manifest, error) {

	source := rand.NewSource(seed)

	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	img, err := pipelines(imgref, config, architecture, rng)
	if err != nil {
		fail(err.Error())
	}
	mf := manifest.New()
	mf.Distro = manifest.DISTRO_FEDORA
	runner := &runner.Fedora{Version: 39}
	_, err = img.InstantiateManifest(&mf, repos, runner, rng)

	return &mf, err
}

func pipelines(imgref string, config *BuildConfig, architecture arch.Arch, rng *rand.Rand) (image.ImageKind, error) {
	if imgref == "" {
		fail("pipeline: no base image defined")
	}
	ref := "ostree/1/1/0"
	tlsVerify := true
	containerSource := container.SourceSpec{
		Source:    imgref,
		Name:      imgref,
		TLSVerify: &tlsVerify,
	}

	img := image.NewOSTreeDiskImageFromContainer(containerSource, ref)

	var customizations *blueprint.Customizations
	if config != nil && config.Blueprint != nil {
		customizations = config.Blueprint.Customizations
	}

	img.KernelOptionsAppend = []string{
		"rw",
		"console=tty0",
		"console=ttyS0",
	}

	img.SysrootReadOnly = true

	switch architecture {
	case arch.ARCH_X86_64:
		img.Platform = &platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
			BIOS:       true,
			UEFIVendor: "fedora",
		}
	case arch.ARCH_AARCH64:
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

	img.Workload = &NullWorkload{}

	basept, ok := partitionTables[architecture.String()]
	if !ok {
		fail(fmt.Sprintf("pipelines: no partition tables defined for %s", architecture))
	}
	size := uint64(10 * GibiByte)
	pt, err := disk.NewPartitionTable(&basept, nil, size, disk.RawPartitioningMode, nil, rng)
	check(err)
	img.PartitionTable = pt

	img.Filename = "disk.qcow2"

	return img, nil
}
