package osbuild

// A new org.osbuild.ostree.init stage to create an OSTree repository
func OSTreeInitFsStage() *Stage {
	return &Stage{
		Type: "org.osbuild.ostree.init-fs",
	}
}
