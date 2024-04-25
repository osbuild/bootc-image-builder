package buildconfig

func MockConfigRootDir(newDir string) (restore func()) {
	saved := configRootDir
	configRootDir = newDir
	return func() {
		configRootDir = saved
	}
}
