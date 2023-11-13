package runner

import "fmt"

type RHEL struct {
	Major uint64
	Minor uint64
}

func (r *RHEL) String() string {
	return fmt.Sprintf("org.osbuild.rhel%d%d", r.Major, r.Minor)
}

func (p *RHEL) GetBuildPackages() []string {
	packages := []string{
		"glibc", // ldconfig
	}
	if p.Major >= 8 {
		packages = append(packages,
			"systemd",         // systemd-tmpfiles and systemd-sysusers
			"platform-python", // osbuild
		)
	}

	if p.Major < 9 {
		packages = append(packages,
			// The RHEL 8 runner in osbuild runs with platform-python but
			// explicitly symlinks python 3.6 to /etc/alternatives (which in turn
			// is the target for /usr/bin/python3) for the stages.
			// https://github.com/osbuild/osbuild/blob/ea8261cad6c5c606c00c0f2824c3f483c01a0cc9/runners/org.osbuild.rhel82#L61
			// Install python36 explicitly for RHEL 8.
			"python36",
		)
	} else {
		packages = append(packages,
			"python3", // osbuild stages
		)
	}
	return packages
}
