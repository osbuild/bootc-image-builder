package runner

import "fmt"

type Fedora struct {
	Version uint64
}

func (r *Fedora) String() string {
	return fmt.Sprintf("org.osbuild.fedora%d", r.Version)
}

func (p *Fedora) GetBuildPackages() []string {
	return []string{
		"glibc",   // ldconfig
		"systemd", // systemd-tmpfiles and systemd-sysusers
		"python3", // osbuild
	}
}
