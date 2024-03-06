package osbuild

type SkopeoDestination interface {
	isSkopeoDestination()
}

type SkopeoDestinationContainersStorage struct {
	Type          string `json:"type"`
	StoragePath   string `json:"storage-path,omitempty"`
	StorageDriver string `json:"storage-driver,omitempty"`
}

func (SkopeoDestinationContainersStorage) isSkopeoDestination() {}

type SkopeoDestinationOCI struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

func (SkopeoDestinationOCI) isSkopeoDestination() {}

type SkopeoStageOptions struct {
	Destination SkopeoDestination `json:"destination"`
}

func (o SkopeoStageOptions) isStageOptions() {}

type SkopeoStageInputs struct {
	Images        ContainersInput `json:"images"`
	ManifestLists *FilesInput     `json:"manifest-lists,omitempty"`
}

func (SkopeoStageInputs) isStageInputs() {}

func NewSkopeoStageWithContainersStorage(path string, images ContainersInput, manifests *FilesInput) *Stage {

	inputs := SkopeoStageInputs{
		Images:        images,
		ManifestLists: manifests,
	}

	return &Stage{
		Type: "org.osbuild.skopeo",
		Options: &SkopeoStageOptions{
			Destination: SkopeoDestinationContainersStorage{
				Type:        "containers-storage",
				StoragePath: path,
			},
		},
		Inputs: inputs,
	}
}

func NewSkopeoStageWithOCI(path string, images ContainersInput, manifests *FilesInput) *Stage {
	inputs := SkopeoStageInputs{
		Images:        images,
		ManifestLists: manifests,
	}

	return &Stage{
		Type: "org.osbuild.skopeo",
		Options: &SkopeoStageOptions{
			Destination: &SkopeoDestinationOCI{
				Type: "oci",
				Path: path,
			},
		},
		Inputs: inputs,
	}
}
