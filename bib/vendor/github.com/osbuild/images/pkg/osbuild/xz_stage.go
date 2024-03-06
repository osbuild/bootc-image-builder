package osbuild

type XzStageOptions struct {
	// Filename for xz archive
	Filename string `json:"filename"`
}

func (XzStageOptions) isStageOptions() {}

func NewXzStageOptions(filename string) *XzStageOptions {
	return &XzStageOptions{
		Filename: filename,
	}
}

type XzStageInputs struct {
	File *FilesInput `json:"file"`
}

func (*XzStageInputs) isStageInputs() {}

func NewXzStageInputs(references FilesInputRef) *XzStageInputs {
	return &XzStageInputs{
		File: NewFilesInput(references),
	}
}

// Compresses a file into a xz archive.
func NewXzStage(options *XzStageOptions, inputs *XzStageInputs) *Stage {
	var stageInputs Inputs
	if inputs != nil {
		stageInputs = inputs
	}

	return &Stage{
		Type:    "org.osbuild.xz",
		Options: options,
		Inputs:  stageInputs,
	}
}
