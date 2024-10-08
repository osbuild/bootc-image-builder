package container

func MockSecretDirSrc(new string) (restore func()) {
	saved := secretDirSrc
	secretDirSrc = new
	return func() {
		secretDirSrc = saved
	}
}
