package osbuild

import (
	"github.com/osbuild/images/pkg/container"
)

type ContainersInputSourceRef struct {
	Name string `json:"name"`
}

type ContainersInput struct {
	inputCommon
	References map[string]ContainersInputSourceRef `json:"references"`
}

func NewContainersInputForSources(containers []container.Spec) ContainersInput {
	refs := make(map[string]ContainersInputSourceRef, len(containers))
	for _, c := range containers {
		ref := ContainersInputSourceRef{
			Name: c.LocalName,
		}
		refs[c.ImageID] = ref
	}

	return ContainersInput{
		References: refs,
		inputCommon: inputCommon{
			Type:   "org.osbuild.containers",
			Origin: InputOriginSource,
		},
	}
}

func (c ContainersInput) isStageInputs() {}
