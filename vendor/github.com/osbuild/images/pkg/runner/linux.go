package runner

type Linux struct {
}

func (r *Linux) String() string {
	return "org.osbuild.linux"
}

func (p *Linux) GetBuildPackages() []string {
	return []string{
		"glibc",   // ldconfig
		"systemd", // systemd-tmpfiles and systemd-sysusers
		"python3", // osbuild
	}
}
