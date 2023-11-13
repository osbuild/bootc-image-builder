package artifact

type Artifact struct {
	export   string
	filename string
	mimeType string
}

func New(export, filename string, mimeType *string) *Artifact {
	artifact := &Artifact{
		export:   export,
		filename: filename,
		mimeType: "application/octet-stream",
	}
	if mimeType != nil {
		artifact.mimeType = *mimeType
	}
	return artifact
}

func (a *Artifact) Export() string {
	return a.export
}

func (a *Artifact) Filename() string {
	return a.filename
}

func (a *Artifact) MIMEType() string {
	return a.mimeType
}
