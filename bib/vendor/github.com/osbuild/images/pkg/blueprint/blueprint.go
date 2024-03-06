// Package blueprint contains primitives for representing weldr blueprints
package blueprint

import "fmt"

const (
	dockerTransport            = "docker"
	containersStorageTransport = "containers-storage"
)

// A Blueprint is a high-level description of an image.
type Blueprint struct {
	Name           string          `json:"name" toml:"name"`
	Description    string          `json:"description" toml:"description"`
	Version        string          `json:"version,omitempty" toml:"version,omitempty"`
	Packages       []Package       `json:"packages" toml:"packages"`
	Modules        []Package       `json:"modules" toml:"modules"`
	Groups         []Group         `json:"groups" toml:"groups"`
	Containers     []Container     `json:"containers,omitempty" toml:"containers,omitempty"`
	Customizations *Customizations `json:"customizations,omitempty" toml:"customizations"`
	Distro         string          `json:"distro" toml:"distro"`

	// EXPERIMENTAL
	Minimal bool `json:"minimal" toml:"minimal"`
}

// A Package specifies an RPM package.
type Package struct {
	Name    string `json:"name" toml:"name"`
	Version string `json:"version,omitempty" toml:"version,omitempty"`
}

// A group specifies an package group.
type Group struct {
	Name string `json:"name" toml:"name"`
}

type Container struct {
	Source string `json:"source,omitempty" toml:"source"`
	Name   string `json:"name,omitempty" toml:"name,omitempty"`

	TLSVerify           *bool   `json:"tls-verify,omitempty" toml:"tls-verify,omitempty"`
	ContainersTransport *string `json:"containers-transport,omitempty" toml:"containers-transport,omitempty"`
	StoragePath         *string `json:"source-path,omitempty" toml:"source-path,omitempty"`
}

// packages, modules, and groups all resolve to rpm packages right now. This
// function returns a combined list of "name-version" strings.
func (b *Blueprint) GetPackages() []string {
	return b.GetPackagesEx(true)
}

func (b *Blueprint) GetPackagesEx(bootable bool) []string {
	packages := []string{}
	for _, pkg := range b.Packages {
		packages = append(packages, pkg.ToNameVersion())
	}
	for _, pkg := range b.Modules {
		packages = append(packages, pkg.ToNameVersion())
	}
	for _, group := range b.Groups {
		packages = append(packages, "@"+group.Name)
	}

	if bootable {
		kc := b.Customizations.GetKernel()
		kpkg := Package{Name: kc.Name}
		packages = append(packages, kpkg.ToNameVersion())
	}

	return packages
}

func (p Package) ToNameVersion() string {
	// Omit version to prevent all packages with prefix of name to be installed
	if p.Version == "*" || p.Version == "" {
		return p.Name
	}

	return p.Name + "-" + p.Version
}

func (c Container) Validate() error {
	if c.StoragePath != nil {
		if c.ContainersTransport == nil {
			// error out here, but realistically we could also just
			// set the transport instead
			return fmt.Errorf("Cannot specify storage location %s without a transport", *c.StoragePath)
		}

		if *c.ContainersTransport != containersStorageTransport {
			return fmt.Errorf(
				"Incompatible transport %s for storage location %s, only containers-storage transport is supported",
				*c.ContainersTransport,
				*c.StoragePath,
			)
		}
	}

	if c.ContainersTransport == nil {
		return nil
	}

	if *c.ContainersTransport != dockerTransport && *c.ContainersTransport != containersStorageTransport {
		return fmt.Errorf("Unknown containers-transport: %s", *c.ContainersTransport)
	}

	return nil
}
