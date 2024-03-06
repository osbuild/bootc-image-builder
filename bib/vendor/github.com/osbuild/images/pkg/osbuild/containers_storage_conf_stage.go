package osbuild

import "fmt"

type CSCStorageOptions struct {
	AdditionalImageStores []string `json:"additionalimagestores,omitempty"`
}

// CSCStorage is short for ContainersStorageConfigStorage
type CSCStorage struct {
	Options *CSCStorageOptions `json:"options,omitempty"`
}

type ContainersStorageConfig struct {
	Storage CSCStorage `json:"storage,omitempty"`
}

type ContainersStorageConfStageOptions struct {
	Filename string                  `json:"filename,omitempty"`
	Config   ContainersStorageConfig `json:"config"`
	Comment  []string                `json:"comment,omitempty"`
}

func (ContainersStorageConfStageOptions) isStageOptions() {}

func NewContainerStorageOptions(filename string, additionalImageStores ...string) *ContainersStorageConfStageOptions {
	options := ContainersStorageConfStageOptions{
		Filename: filename,
	}

	if len(additionalImageStores) > 0 {
		options.Config.Storage.Options = &CSCStorageOptions{
			AdditionalImageStores: additionalImageStores,
		}
	}

	return &options
}

func (o *ContainersStorageConfStageOptions) validate() error {
	if o.Filename == "" {
		return fmt.Errorf("`Filename` must be set")
	}

	return nil
}

func NewContainersStorageConfStage(options *ContainersStorageConfStageOptions) *Stage {

	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.containers.storage.conf",
		Options: options,
	}
}
