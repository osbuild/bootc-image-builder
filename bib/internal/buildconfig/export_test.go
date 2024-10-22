package buildconfig

import (
	"os"
)

func MockConfigRootDir(newDir string) (restore func()) {
	saved := configRootDir
	configRootDir = newDir
	return func() {
		configRootDir = saved
	}
}

func MockOsStdin(new *os.File) (restore func()) {
	saved := osStdin
	osStdin = new
	return func() {
		osStdin = saved
	}
}
