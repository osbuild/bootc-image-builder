package osbuild

func NewBootupdGenMetadataStage() *Stage {
	return &Stage{
		Type: "org.osbuild.bootupd.gen-metadata",
	}
}
