package osbuild

import (
	"github.com/osbuild/images/pkg/container"
)

type ContainersInputReferences interface {
	isContainersInputReferences()
	Len() int
}

type ContainersInputSourceRef struct {
	Name string `json:"name"`
}

type ContainersInputSourceMap map[string]ContainersInputSourceRef

func (ContainersInputSourceMap) isContainersInputReferences() {}

func (cism ContainersInputSourceMap) Len() int {
	return len(cism)
}

type ContainersInput struct {
	inputCommon
	References ContainersInputReferences `json:"references"`
}

const InputTypeContainers string = "org.osbuild.containers"

func NewContainersInputForSources(containers []container.Spec) ContainersInput {
	refs := make(ContainersInputSourceMap, len(containers))
	for _, c := range containers {
		ref := ContainersInputSourceRef{
			Name: c.LocalName,
		}
		refs[c.ImageID] = ref
	}

	return ContainersInput{
		References: refs,
		inputCommon: inputCommon{
			Type:   InputTypeContainers,
			Origin: InputOriginSource,
		},
	}
}

type ContainersInputs map[string]ContainersInput

func (c ContainersInputs) isStageInputs() {}
