package osbuild

type SkopeoDestination struct {
	Type          string `json:"type"`
	StoragePath   string `json:"storage-path,omitempty"`
	StorageDriver string `json:"sotrage-driver,omitempty"`
}

type SkopeoStageOptions struct {
	Destination SkopeoDestination `json:"destination"`
}

func (o SkopeoStageOptions) isStageOptions() {}

type SkopeoStageInputs struct {
	Images        ContainersInput `json:"images"`
	ManifestLists *FilesInput     `json:"manifest-lists,omitempty"`
}

func (SkopeoStageInputs) isStageInputs() {}

func NewSkopeoStage(path string, images ContainersInput, manifests *FilesInput) *Stage {

	inputs := SkopeoStageInputs{
		Images:        images,
		ManifestLists: manifests,
	}

	return &Stage{
		Type: "org.osbuild.skopeo",
		Options: &SkopeoStageOptions{
			Destination: SkopeoDestination{
				Type:        "containers-storage",
				StoragePath: path,
			},
		},
		Inputs: inputs,
	}
}
