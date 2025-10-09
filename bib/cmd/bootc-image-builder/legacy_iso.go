package main

import (
	"fmt"
	"math/rand"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/arch"
	podman_container "github.com/osbuild/images/pkg/bib/container"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro/bootc"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"

	"github.com/osbuild/bootc-image-builder/bib/internal/distrodef"
)

// all possible locations for the bib's distro definitions
// ./data/defs and ./bib/data/defs are for development
// /usr/share/bootc-image-builder/defs is for the production, containerized version
var distroDefPaths = []string{
	"./data/defs",
	"./bib/data/defs",
	"/usr/share/bootc-image-builder/defs",
}

type ManifestConfig struct {
	// OCI image path (without the transport, that is always docker://)
	Imgref      string
	BuildImgref string

	// Build config
	Config *blueprint.Blueprint

	// CPU architecture of the image
	Architecture arch.Arch

	// Paths to the directory with the distro definitions
	DistroDefPaths []string

	// Extracted information about the source container image
	SourceInfo      *osinfo.Info
	BuildSourceInfo *osinfo.Info

	// RootFSType specifies the filesystem type for the root partition
	RootFSType string

	// use librepo ad the rpm downlaod backend
	UseLibrepo bool
}

func manifestFromCobraForLegacyISO(imgref, buildImgref, imgTypeStr, rootFs, rpmCacheRoot string, config *blueprint.Blueprint, useLibrepo bool, cntArch arch.Arch) ([]byte, *mTLSConfig, error) {
	container, err := podman_container.New(imgref)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := container.Stop(); err != nil {
			logrus.Warnf("error stopping container: %v", err)
		}
	}()

	var rootfsType string
	if rootFs != "" {
		rootfsType = rootFs
	} else {
		rootfsType, err = container.DefaultRootfsType()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot get rootfs type for container: %w", err)
		}
		if rootfsType == "" {
			return nil, nil, fmt.Errorf(`no default root filesystem type specified in container, please use "--rootfs" to set manually`)
		}
	}

	// Gather some data from the containers distro
	sourceinfo, err := osinfo.Load(container.Root())
	if err != nil {
		return nil, nil, err
	}

	buildContainer := container
	buildSourceinfo := sourceinfo
	startedBuildContainer := false
	defer func() {
		if startedBuildContainer {
			if err := buildContainer.Stop(); err != nil {
				logrus.Warnf("error stopping container: %v", err)
			}
		}
	}()

	if buildImgref != "" {
		buildContainer, err = podman_container.New(buildImgref)
		if err != nil {
			return nil, nil, err
		}
		startedBuildContainer = true

		// Gather some data from the containers distro
		buildSourceinfo, err = osinfo.Load(buildContainer.Root())
		if err != nil {
			return nil, nil, err
		}
	} else {
		buildImgref = imgref
	}

	// This is needed just for RHEL and RHSM in most cases, but let's run it every time in case
	// the image has some non-standard dnf plugins.
	if err := buildContainer.InitDNF(); err != nil {
		return nil, nil, err
	}
	solver, err := buildContainer.NewContainerSolver(rpmCacheRoot, cntArch, sourceinfo)
	if err != nil {
		return nil, nil, err
	}

	manifestConfig := &ManifestConfig{
		Architecture:    cntArch,
		Config:          config,
		Imgref:          imgref,
		BuildImgref:     buildImgref,
		DistroDefPaths:  distroDefPaths,
		SourceInfo:      sourceinfo,
		BuildSourceInfo: buildSourceinfo,
		RootFSType:      rootfsType,
		UseLibrepo:      useLibrepo,
	}

	manifest, repos, err := makeISOManifest(manifestConfig, solver, rpmCacheRoot)
	if err != nil {
		return nil, nil, err
	}

	mTLS, err := extractTLSKeys(repos)
	if err != nil {
		return nil, nil, err
	}

	return manifest, mTLS, nil
}

func makeISOManifest(c *ManifestConfig, solver *depsolvednf.Solver, cacheRoot string) (manifest.OSBuildManifest, map[string][]rpmmd.RepoConfig, error) {
	rng := createRand()
	mani, err := manifestForISO(c, rng)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get manifest: %w", err)
	}

	// depsolve packages
	depsolvedSets := make(map[string]depsolvednf.DepsolveResult)
	depsolvedRepos := make(map[string][]rpmmd.RepoConfig)
	pkgSetChains, err := mani.GetPackageSetChains()
	if err != nil {
		return nil, nil, err
	}
	for name, pkgSet := range pkgSetChains {
		res, err := solver.Depsolve(pkgSet, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot depsolve: %w", err)
		}
		depsolvedSets[name] = *res
		depsolvedRepos[name] = res.Repos
	}

	// Resolve container - the normal case is that host and target
	// architecture are the same. However it is possible to build
	// cross-arch images by using qemu-user. This will run everything
	// (including the build-root) with the target arch then, it
	// is fast enough (given that it's mostly I/O and all I/O is
	// run naively via syscall translation)

	// XXX: should NewResolver() take "arch.Arch"?
	resolver := container.NewResolver(c.Architecture.String())

	containerSpecs := make(map[string][]container.Spec)
	for plName, sourceSpecs := range mani.GetContainerSourceSpecs() {
		for _, c := range sourceSpecs {
			resolver.Add(c)
		}
		specs, err := resolver.Finish()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot resolve containers: %w", err)
		}
		for _, spec := range specs {
			if spec.Arch != c.Architecture {
				return nil, nil, fmt.Errorf("image found is for unexpected architecture %q (expected %q), if that is intentional, please make sure --target-arch matches", spec.Arch, c.Architecture)
			}
		}
		containerSpecs[plName] = specs
	}

	var opts manifest.SerializeOptions
	if c.UseLibrepo {
		opts.RpmDownloader = osbuild.RpmDownloaderLibrepo
	}
	mf, err := mani.Serialize(depsolvedSets, containerSpecs, nil, &opts)
	if err != nil {
		return nil, nil, fmt.Errorf("[ERROR] manifest serialization failed: %s", err.Error())
	}
	return mf, depsolvedRepos, nil
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
		Source: c.Imgref,
		Name:   c.Imgref,
		Local:  true,
	}

	if c.Architecture == arch.ARCH_AARCH64 {
		// aarch64 always uses UEFI, so let's enforce the vendor
		if c.SourceInfo.UEFIVendor == "" {
			return nil, fmt.Errorf("UEFI vendor must be set for aarch64 ISO")
		}
	}
	platformi := bootc.PlatformFor(c.Architecture.String(), c.SourceInfo.UEFIVendor)
	platformi.ImageFormat = platform.FORMAT_ISO
	filename := "install.iso"

	// The ref is not needed and will be removed from the ctor later
	// in time
	img := image.NewAnacondaContainerInstallerLegacy(platformi, filename, containerSource, "")
	img.ContainerRemoveSignatures = true
	img.RootfsCompression = "zstd"

	if c.Architecture == arch.ARCH_X86_64 {
		img.InstallerCustomizations.ISOBoot = manifest.Grub2ISOBoot
	}

	img.InstallerCustomizations.Product = c.SourceInfo.OSRelease.Name
	img.InstallerCustomizations.OSVersion = c.SourceInfo.OSRelease.VersionID
	img.InstallerCustomizations.ISOLabel = bootc.LabelForISO(&c.SourceInfo.OSRelease, c.Architecture.String())

	img.ExtraBasePackages = rpmmd.PackageSet{
		Include: imageDef.Packages,
	}

	var customizations *blueprint.Customizations
	if c.Config != nil {
		customizations = c.Config.Customizations
	}
	img.InstallerCustomizations.FIPS = customizations.GetFIPS()
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
		img.InstallerCustomizations.EnabledAnacondaModules = append(img.InstallerCustomizations.EnabledAnacondaModules, instCust.Modules.Enable...)
		img.InstallerCustomizations.DisabledAnacondaModules = append(img.InstallerCustomizations.DisabledAnacondaModules, instCust.Modules.Disable...)
	}
	img.InstallerCustomizations.EnabledAnacondaModules = append(img.InstallerCustomizations.EnabledAnacondaModules,
		anaconda.ModuleUsers,
		anaconda.ModuleServices,
		anaconda.ModuleSecurity,
		// XXX: get from the imagedefs
		anaconda.ModuleNetwork,
		anaconda.ModulePayloads,
		anaconda.ModuleRuntime,
		anaconda.ModuleStorage,
	)

	img.Kickstart.OSTree = &kickstart.OSTree{
		OSName: "default",
	}
	img.InstallerCustomizations.LoraxTemplates = bootc.LoraxTemplates(c.SourceInfo.OSRelease)
	img.InstallerCustomizations.LoraxTemplatePackage = bootc.LoraxTemplatePackage(c.SourceInfo.OSRelease)

	// see https://github.com/osbuild/bootc-image-builder/issues/733
	img.InstallerCustomizations.ISORootfsType = manifest.SquashfsRootfs

	installRootfsType, err := disk.NewFSType(c.RootFSType)
	if err != nil {
		return nil, err
	}
	img.InstallRootfsType = installRootfsType

	mf := manifest.New()

	foundDistro, foundRunner, err := bootc.GetDistroAndRunner(c.SourceInfo.OSRelease)
	if err != nil {
		return nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifest(&mf, nil, foundRunner, rng)
	return &mf, err
}
