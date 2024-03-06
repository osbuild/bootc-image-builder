package runner

import "fmt"

type CentOS struct {
	Version uint64
}

func (c *CentOS) String() string {
	return fmt.Sprintf("org.osbuild.centos%d", c.Version)
}

func (c *CentOS) GetBuildPackages() []string {
	packages := []string{
		"glibc",           // ldconfig
		"platform-python", // osbuild
	}
	if c.Version >= 8 {
		packages = append(packages,
			"systemd", // systemd-tmpfiles and systemd-sysusers
		)
	}
	if c.Version < 9 {
		packages = append(packages,
			// The RHEL 8 runner (which is also used for CS8) in osbuild runs
			// with platform-python but explicitly symlinks python 3.6 to
			// /etc/alternatives (which in turn is the target for
			// /usr/bin/python3) for the stages.
			// https://github.com/osbuild/osbuild/blob/ea8261cad6c5c606c00c0f2824c3f483c01a0cc9/runners/org.osbuild.rhel82#L61
			// Install python36 explicitly for CS8.
			"python36",
		)
	} else {
		packages = append(packages,
			"python3", // osbuild stages
		)
	}
	return packages
}
