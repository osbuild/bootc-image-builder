package common

import "fmt"

const (
	KiloByte = 1000                      // kB
	KibiByte = 1024                      // KiB
	MegaByte = 1000 * 1000               // MB
	MebiByte = 1024 * 1024               // MiB
	GigaByte = 1000 * 1000 * 1000        // GB
	GibiByte = 1024 * 1024 * 1024        // GiB
	TeraByte = 1000 * 1000 * 1000 * 1000 // TB
	TebiByte = 1024 * 1024 * 1024 * 1024 // TiB

	// shorthands
	KiB = KibiByte
	MB  = MegaByte
	MiB = MebiByte
	GB  = GigaByte
	GiB = GibiByte
	TiB = TebiByte
)

// These constants are set during buildtime using additional
// compiler flags. Not all of them are necessarily defined
// because RPMs can be build from a tarball and spec file without
// being in a git repository. On the other hand when building
// composer inside of a container, there is no RPM layer so in
// that case the RPM version doesn't exist at all.
var (
	// Git revision from which this code was built
	GitRev = "undefined"

	// RPM Version
	RpmVersion = "undefined"
)

func BuildVersion() string {
	if GitRev != "undefined" {
		return fmt.Sprintf("git-rev:%s", GitRev)
	} else if RpmVersion != "undefined" {
		return fmt.Sprintf("NEVRA:%s", RpmVersion)
	} else {
		return "devel"
	}
}
