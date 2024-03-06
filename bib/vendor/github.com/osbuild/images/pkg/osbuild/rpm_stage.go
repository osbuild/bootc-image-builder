package osbuild

import (
	"github.com/osbuild/images/pkg/rpmmd"
)

type RPMStageOptions struct {
	// Use the given path as RPM database
	DBPath string `json:"dbpath,omitempty"`

	// Array of GPG key contents to import
	GPGKeys []string `json:"gpgkeys,omitempty"`

	// Array of files in the tree containing GPG keys to import
	GPGKeysFromTree []string `json:"gpgkeys.fromtree,omitempty"`

	// Prevent dracut from running
	DisableDracut bool `json:"disable_dracut,omitempty"`

	Exclude *Exclude `json:"exclude,omitempty"`

	// Create the '/run/ostree-booted' marker
	OSTreeBooted *bool `json:"ostree_booted,omitempty"`
}

type Exclude struct {
	// Do not install documentation.
	Docs bool `json:"docs,omitempty"`
}

// RPMPackage represents one RPM, as referenced by its content hash
// (checksum). The files source must indicate where to fetch the given
// RPM. If CheckGPG is `true` the RPM must be signed with one of the
// GPGKeys given in the RPMStageOptions.
type RPMPackage struct {
	Checksum string `json:"checksum"`
	CheckGPG bool   `json:"check_gpg,omitempty"`
}

func (RPMStageOptions) isStageOptions() {}

// RPMStageInputs defines a collection of packages to be installed by the RPM
// stage.
type RPMStageInputs struct {
	// Packages to install
	Packages *FilesInput `json:"packages"`
}

func (RPMStageInputs) isStageInputs() {}

// RPM package reference metadata
type RPMStageReferenceMetadata struct {
	// This option defaults to `false`, therefore it does not need to be
	// defined as pointer to bool and can be omitted.
	CheckGPG bool `json:"rpm.check_gpg,omitempty"`
}

func (*RPMStageReferenceMetadata) isFilesInputRefMetadata() {}

// NewRPMStage creates a new RPM stage.
func NewRPMStage(options *RPMStageOptions, inputs *RPMStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.rpm",
		Inputs:  inputs,
		Options: options,
	}
}

// RPMStageMetadata gives the set of packages installed by the RPM stage
type RPMStageMetadata struct {
	Packages []RPMPackageMetadata `json:"packages"`
}

// RPMPackageMetadata contains the metadata extracted from one RPM header
type RPMPackageMetadata struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Release string  `json:"release"`
	Epoch   *string `json:"epoch"`
	Arch    string  `json:"arch"`
	SigMD5  string  `json:"sigmd5"`
	SigPGP  string  `json:"sigpgp"`
	SigGPG  string  `json:"siggpg"`
}

func (RPMStageMetadata) isStageMetadata() {}

func OSBuildMetadataToRPMs(stagesMetadata map[string]StageMetadata) []rpmmd.RPM {
	rpms := make([]rpmmd.RPM, 0)
	for _, md := range stagesMetadata {
		switch metadata := md.(type) {
		case *RPMStageMetadata:
			for _, pkg := range metadata.Packages {
				rpms = append(rpms, rpmmd.RPM{
					Type:      "rpm",
					Name:      pkg.Name,
					Epoch:     pkg.Epoch,
					Version:   pkg.Version,
					Release:   pkg.Release,
					Arch:      pkg.Arch,
					Sigmd5:    pkg.SigMD5,
					Signature: RPMPackageMetadataToSignature(pkg),
				})
			}
		default:
			continue
		}
	}
	return rpms
}

func RPMPackageMetadataToSignature(pkg RPMPackageMetadata) *string {
	if pkg.SigGPG != "" {
		return &pkg.SigGPG
	} else if pkg.SigPGP != "" {
		return &pkg.SigPGP
	}
	return nil
}

func NewRpmStageSourceFilesInputs(specs []rpmmd.PackageSpec) *RPMStageInputs {
	input := NewFilesInput(pkgRefs(specs))
	return &RPMStageInputs{Packages: input}
}

func pkgRefs(specs []rpmmd.PackageSpec) FilesInputRef {
	refs := make([]FilesInputSourceArrayRefEntry, len(specs))
	for idx, pkg := range specs {
		var pkgMetadata FilesInputRefMetadata
		if pkg.CheckGPG {
			pkgMetadata = &RPMStageReferenceMetadata{
				CheckGPG: pkg.CheckGPG,
			}
		}
		refs[idx] = NewFilesInputSourceArrayRefEntry(pkg.Checksum, pkgMetadata)
	}
	return NewFilesInputSourceArrayRef(refs)
}

func NewRPMStageOptions(repos []rpmmd.RepoConfig) *RPMStageOptions {
	var gpgKeys []string
	for _, repo := range repos {
		if len(repo.GPGKeys) == 0 {
			continue
		}
		gpgKeys = append(gpgKeys, repo.GPGKeys...)
	}

	return &RPMStageOptions{
		GPGKeys: gpgKeys,
	}
}
