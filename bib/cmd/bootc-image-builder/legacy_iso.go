package main

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/bootc"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/sirupsen/logrus"

	podman_container "github.com/osbuild/images/pkg/bib/container"
)

// newDistroYAMLFrom() returns the distroYAML for the given sourceInfo,
// if no direct match can be found it will it will use the ID_LIKE.
// This should ensure we work on every bootc image that puts a correct
// ID_LIKE= in /etc/os-release
func newDistroYAMLFrom(sourceInfo *osinfo.Info) (*defs.DistroYAML, *distro.ID, error) {
	for _, distroID := range append([]string{sourceInfo.OSRelease.ID}, sourceInfo.OSRelease.IDLike...) {
		nameVer := fmt.Sprintf("%s-%s", distroID, sourceInfo.OSRelease.VersionID)
		id, err := distro.ParseID(nameVer)
		if err != nil {
			return nil, nil, err
		}
		distroYAML, err := defs.NewDistroYAML(nameVer)
		if err != nil {
			return nil, nil, err
		}
		if distroYAML != nil {
			return distroYAML, id, nil
		}
	}
	return nil, nil, fmt.Errorf("cannot load distro definitions for %s-%s or any of %v", sourceInfo.OSRelease.ID, sourceInfo.OSRelease.VersionID, sourceInfo.OSRelease.IDLike)
}

func manifestFromCobraForLegacyISO(imgref, buildImgref, imgTypeStr, rootFs, rpmCacheRoot string, config *blueprint.Blueprint, useLibrepo bool, cntArch arch.Arch) ([]byte, *mTLSConfig, error) {
	cnt, err := podman_container.New(imgref)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := cnt.Stop(); err != nil {
			logrus.Warnf("error stopping container: %v", err)
		}
	}()

	var rootfsType string
	if rootFs != "" {
		rootfsType = rootFs
	} else {
		rootfsType, err = cnt.DefaultRootfsType()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot get rootfs type for container: %w", err)
		}
		if rootfsType == "" {
			return nil, nil, fmt.Errorf(`no default root filesystem type specified in container, please use "--rootfs" to set manually`)
		}
	}

	// Gather some data from the containers distro
	sourceinfo, err := osinfo.Load(cnt.Root())
	if err != nil {
		return nil, nil, err
	}

	buildContainer := cnt
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

	rng := createRand()
	mani, err := imgTypeManifestForLegacyISO(imgref, rootFs, cntArch.String(), buildSourceinfo, config, rng)
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
	resolver := container.NewResolver(cntArch.String())

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
			if spec.Arch != cntArch {
				return nil, nil, fmt.Errorf("image found is for unexpected architecture %q (expected %q), if that is intentional, please make sure --target-arch matches", spec.Arch, cntArch)
			}
		}
		containerSpecs[plName] = specs
	}

	var opts manifest.SerializeOptions
	if useLibrepo {
		opts.RpmDownloader = osbuild.RpmDownloaderLibrepo
	}
	mf, err := mani.Serialize(depsolvedSets, containerSpecs, nil, &opts)
	if err != nil {
		return nil, nil, fmt.Errorf("[ERROR] manifest serialization failed: %s", err.Error())
	}

	mTLS, err := extractTLSKeys(depsolvedRepos)
	if err != nil {
		return nil, nil, err
	}

	return mf, mTLS, nil
}

// XXX: ideally this would be an imageType.Manifest() function but
// we have no legacyISO image type (yet)
func imgTypeManifestForLegacyISO(imgref, rootFSType, archStr string, sourceInfo *osinfo.Info, bp *blueprint.Blueprint, rng *rand.Rand) (*manifest.Manifest, error) {
	if imgref == "" {
		return nil, fmt.Errorf("pipeline: no base image defined")
	}
	distroYAML, id, err := newDistroYAMLFrom(sourceInfo)
	if err != nil {
		return nil, err
	}

	// XXX: or "bootc-legacy-installer"?
	installerImgTypeName := "bootc-rpm-installer"
	imgType, ok := distroYAML.ImageTypes()[installerImgTypeName]
	if !ok {
		return nil, fmt.Errorf("cannot find image definition for %v", installerImgTypeName)
	}
	installerPkgSet, ok := imgType.PackageSets(*id, archStr)["installer"]
	if !ok {
		return nil, fmt.Errorf("cannot find installer package set for %v", installerImgTypeName)
	}
	installerConfig := imgType.InstallerConfig(*id, archStr)
	if installerConfig == nil {
		return nil, fmt.Errorf("empty installer config for %s", installerImgTypeName)
	}

	containerSource := container.SourceSpec{
		Source: imgref,
		Name:   imgref,
		Local:  true,
	}

	platformi := bootc.PlatformFor(archStr, sourceInfo.UEFIVendor)
	platformi.ImageFormat = platform.FORMAT_ISO

	// The ref is not needed and will be removed from the ctor later
	// in time
	img := image.NewAnacondaContainerInstallerLegacy(platformi, imgType.Filename, containerSource, "")
	img.ContainerRemoveSignatures = true
	img.RootfsCompression = "zstd"

	if archStr == arch.ARCH_X86_64.String() {
		img.InstallerCustomizations.ISOBoot = manifest.Grub2ISOBoot
	}

	img.InstallerCustomizations.Product = sourceInfo.OSRelease.Name
	img.InstallerCustomizations.OSVersion = sourceInfo.OSRelease.VersionID
	img.InstallerCustomizations.ISOLabel = bootc.LabelForISO(&sourceInfo.OSRelease, archStr)
	img.ExtraBasePackages = installerPkgSet

	var customizations *blueprint.Customizations
	if bp != nil {
		customizations = bp.Customizations
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
	img.InstallerCustomizations.LoraxTemplates = installerConfig.LoraxTemplates
	if installerConfig.LoraxTemplatePackage != nil {
		img.InstallerCustomizations.LoraxTemplatePackage = *installerConfig.LoraxTemplatePackage
	}

	// see https://github.com/osbuild/bootc-image-builder/issues/733
	img.InstallerCustomizations.ISORootfsType = manifest.SquashfsRootfs

	installRootfsType, err := disk.NewFSType(rootFSType)
	if err != nil {
		return nil, err
	}
	img.InstallRootfsType = installRootfsType

	mf := manifest.New()

	foundDistro, foundRunner, err := bootc.GetDistroAndRunner(sourceInfo.OSRelease)
	if err != nil {
		return nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifest(&mf, nil, foundRunner, rng)
	return &mf, err
}
