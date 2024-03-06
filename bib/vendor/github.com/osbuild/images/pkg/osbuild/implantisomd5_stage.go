package osbuild

type Implantisomd5StageOptions struct {
	// Path in the ISO where the md5 checksum will be implanted
	Filename string `json:"filename"`
}

func (Implantisomd5StageOptions) isStageOptions() {}

// Implant an MD5 checksum in an ISO9660 image
func NewImplantisomd5Stage(options *Implantisomd5StageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.implantisomd5",
		Options: options,
	}
}
