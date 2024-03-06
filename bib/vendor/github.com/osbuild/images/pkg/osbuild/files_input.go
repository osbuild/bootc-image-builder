package osbuild

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/osbuild/images/pkg/container"
)

// SPECIFIC INPUT STRUCTURE

type FilesInput struct {
	inputCommon
	References FilesInputRef `json:"references"`
}

const InputTypeFiles string = "org.osbuild.files"

func NewFilesInput(references FilesInputRef) *FilesInput {
	input := new(FilesInput)
	input.Type = InputTypeFiles

	switch t := references.(type) {
	case *FilesInputPipelineArrayRef, *FilesInputPipelineObjectRef:
		input.Origin = InputOriginPipeline
	case *FilesInputSourcePlainRef, *FilesInputSourceArrayRef, *FilesInputSourceObjectRef:
		input.Origin = InputOriginSource
	default:
		panic(fmt.Sprintf("unknown FilesInputReferences type: %v", t))
	}

	input.References = references

	return input
}

type rawFilesInput struct {
	inputCommon
	References json.RawMessage `json:"references"`
}

func (f *FilesInput) UnmarshalJSON(data []byte) error {
	var rawFilesInput rawFilesInput
	if err := json.Unmarshal(data, &rawFilesInput); err != nil {
		return err
	}

	switch rawFilesInput.Origin {
	case InputOriginPipeline:
		possibleRefs := []FilesInputRef{
			&FilesInputPipelineArrayRef{},
			&FilesInputPipelineObjectRef{},
		}
		var err error
		for _, ref := range possibleRefs {
			if err = json.Unmarshal(rawFilesInput.References, ref); err == nil {
				f.References = ref
				break
			}
		}
		if err != nil {
			return fmt.Errorf("FilesInput: failed to unmarshal pipeline references into any supported type")
		}
	case InputOriginSource:
		possibleRefs := []FilesInputRef{
			&FilesInputSourcePlainRef{},
			&FilesInputSourceArrayRef{},
			&FilesInputSourceObjectRef{},
		}
		var err error
		for _, ref := range possibleRefs {
			if err = json.Unmarshal(rawFilesInput.References, ref); err == nil {
				f.References = ref
				break
			}
		}
		if err != nil {
			return fmt.Errorf("FilesInput: failed to unmarshal source references into any supported type")
		}
	default:
		return fmt.Errorf("FilesInput: unknown input origin: %s", rawFilesInput.Origin)
	}

	f.Type = rawFilesInput.Type
	f.Origin = rawFilesInput.Origin

	return nil
}

// SUPPORTED FILE INPUT REFERENCES

// FilesInputRef is an interface that is implemented by all types that can be
// used as a reference in the files input.
type FilesInputRef interface {
	isFilesInputRef()
}

// Type to represent stage-specific metadata that can be passed via the files
// input to the stage.
// The expected JSON structure is:
//
//	`{
//		"<metadata.str1>": <anything_but_object_with_additional_properties>,
//		"<metadata.str2>": <anything_but_object_with_additional_properties>,
//		...
//	}`
type FilesInputRefMetadata interface {
	isFilesInputRefMetadata()
}

// Pipeline Object Reference
// The expected JSON structure is:
//
//	`{
//		"name:<pipeline_name>": {
//			"metadata": {
//				"<metadata.str1>": <anything_but_object_with_additional_properties>,
//				"<metadata.str2>": <anything_but_object_with_additional_properties>,
//				...
//			},
//			"file": "<filename>"
//		},
//		...
//	}`
type FilesInputPipelineObjectRef map[string]FilesInputPipelineOptions

func (*FilesInputPipelineObjectRef) isFilesInputRef() {}

type FilesInputPipelineOptions struct {
	// File to access with in a pipeline
	File string `json:"file,omitempty"`
	// Additional metadata to forward to the stage
	Metadata FilesInputRefMetadata `json:"metadata,omitempty"`
}

func NewFilesInputPipelineObjectRef(pipeline, filename string, metadata FilesInputRefMetadata) FilesInputRef {
	// The files input schema allows for multiple pipelines to be specified, but we don't use it.
	ref := &FilesInputPipelineObjectRef{
		fmt.Sprintf("name:%s", pipeline): {
			File:     filename,
			Metadata: metadata,
		},
	}
	return ref
}

// Pipeline Array Reference
// The expected JSON structure is:
//
//	`[
//		{
//			"id": "name:<pipeline_name>",
//			"options": {
//				"metadata": {
//					"<metadata.str1>": <anything_but_object_with_additional_properties>,
//					"<metadata.str2>": <anything_but_object_with_additional_properties>,
//					...
//				},
//				"file": "<filename>"
//			}
//		},
//		...
//	]`
type FilesInputPipelineArrayRef []FilesInputPipelineArrayRefEntry

func (*FilesInputPipelineArrayRef) isFilesInputRef() {}

type FilesInputPipelineArrayRefEntry struct {
	ID      string                    `json:"id"`
	Options FilesInputPipelineOptions `json:"options,omitempty"`
}

func NewFilesInputPipelineArrayRef(pipeline, filename string, metadata FilesInputRefMetadata) FilesInputRef {
	// The files input schema allows for multiple pipelines to be specified, but we don't use it.
	ref := &FilesInputPipelineArrayRef{
		{
			ID: fmt.Sprintf("name:%s", pipeline),
			Options: FilesInputPipelineOptions{
				File:     filename,
				Metadata: metadata,
			},
		},
	}
	return ref
}

// Source Plain Reference
// The expected JSON structure is:
//
//	`[
//		"sha256:<sha256sum>",
//		...
//	]`
type FilesInputSourcePlainRef []string

func (*FilesInputSourcePlainRef) isFilesInputRef() {}

// NewFilesInputSourcePlainRef creates a FilesInputSourcePlainRef from a list
// of checksums. The checksums must be prefixed by the name of the corresponding
// hashing algorithm followed by a colon (e.g. sha256:, sha1:, etc).
func NewFilesInputSourcePlainRef(checksums []string) FilesInputRef {
	for _, cs := range checksums {
		validateChecksum(cs)
	}
	refs := FilesInputSourcePlainRef(checksums)
	return &refs
}

// Source Array Reference
// The expected JSON structure is:
//
//	`[
//		{
//			"id": "sha256:<sha256sum>",
//			"options": {
//				"metadata": {
//					"<metadata.str1>": <anything_but_object_with_additional_properties>,
//					"<metadata.str2>": <anything_but_object_with_additional_properties>,
//					...
//				}
//			}
//		},
//		...
//	]`
type FilesInputSourceArrayRef []FilesInputSourceArrayRefEntry

func (*FilesInputSourceArrayRef) isFilesInputRef() {}

type FilesInputSourceOptions struct {
	// Additional metadata to forward to the stage
	Metadata FilesInputRefMetadata `json:"metadata,omitempty"`
}

type FilesInputSourceArrayRefEntry struct {
	ID      string                   `json:"id"`
	Options *FilesInputSourceOptions `json:"options,omitempty"`
}

// NewFilesInputSourceArrayRefEntry creates a FilesInputSourceArrayRefEntry
// from a checksum and metadata. The checksum must be prefixed by the name of
// the corresponding hashing algorithm followed by a colon (e.g. sha256:,
// sha1:, etc).
func NewFilesInputSourceArrayRefEntry(checksum string, metadata FilesInputRefMetadata) FilesInputSourceArrayRefEntry {
	validateChecksum(checksum)
	ref := FilesInputSourceArrayRefEntry{
		ID: checksum,
	}
	if metadata != nil {
		ref.Options = &FilesInputSourceOptions{Metadata: metadata}
	}
	return ref
}

func NewFilesInputSourceArrayRef(entries []FilesInputSourceArrayRefEntry) FilesInputRef {
	ref := FilesInputSourceArrayRef(entries)
	return &ref
}

// Source Object Reference
// The expected JSON structure is:
//
//	`{
//		"sha256:<sha256sum>": {
//			"metadata": {
//				"<metadata.str1>": <anything_but_object_with_additional_properties>,
//				"<metadata.str2>": <anything_but_object_with_additional_properties>,
//				...
//			}
//		},
//		...
//	}`
type FilesInputSourceObjectRef map[string]FilesInputSourceOptions

func (*FilesInputSourceObjectRef) isFilesInputRef() {}

// NewFilesInputSourceObjectRef creates a FilesInputSourceObjectRef from a map
// of checksums to metadata. The checksums must be prefixed by the name of the
// corresponding hashing algorithm followed by a colon (e.g. sha256:, sha1:,
// etc).
func NewFilesInputSourceObjectRef(entries map[string]FilesInputRefMetadata) FilesInputRef {
	refs := FilesInputSourceObjectRef{}
	for checksum, metadata := range entries {
		validateChecksum(checksum)
		refs[checksum] = FilesInputSourceOptions{Metadata: metadata}
	}
	return &refs
}

// NewFilesInputForManifestLists creates a FilesInput for container manifest
// lists. If there are no list digests in the container specs, it returns nil.
func NewFilesInputForManifestLists(containers []container.Spec) *FilesInput {
	refs := make([]string, 0, len(containers))
	for _, c := range containers {
		if c.ListDigest != "" {
			refs = append(refs, c.ListDigest)
		}
	}
	if len(refs) == 0 {
		return nil
	}
	filesRef := FilesInputSourcePlainRef(refs)
	return NewFilesInput(&filesRef)
}

// validateChecksum panics if the given string does not contain an algorithm
// prefix. It checks this by making sure that the : delimiter exists and is not
// at the start of the string.
func validateChecksum(checksum string) {
	if strings.IndexRune(checksum, ':') == 0 || strings.Count(checksum, ":") != 1 {
		panic(fmt.Sprintf("invalid checksum format; must contain algorithm prefix: %s", checksum))
	}
}
