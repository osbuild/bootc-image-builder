package osbuild

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Result struct {
	Type     string                      `json:"type"`
	Success  bool                        `json:"success"`
	Error    json.RawMessage             `json:"error,omitempty"`
	Log      map[string]PipelineResult   `json:"log"`
	Metadata map[string]PipelineMetadata `json:"metadata"`
	Errors   []ValidationError           `json:"errors,omitempty"`
	Title    string                      `json:"title,omitempty"`
}

type ValidationError struct {
	Message string   `json:"message"`
	Path    []string `json:"path"`
}

type PipelineResult []StageResult

type StageResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Output  string `json:"output"`
	Success bool   `json:"success,omitempty"`
	Error   string `json:"string,omitempty"`
}

type PipelineMetadata map[string]StageMetadata

type StageMetadata interface {
	isStageMetadata()
}

// RawStageMetadata is used to store the metadata from a stage that doesn't
// define its own structure
type RawStageMetadata json.RawMessage

func (RawStageMetadata) isStageMetadata() {}

// UnmarshalJSON decodes json-encoded StageResult.
//
// This method is here only as a workaround for the default value of the
// success field, see the comment inside the method.
func (sr *StageResult) UnmarshalJSON(data []byte) error {
	// Create a StageResult-like object with the Success value set to true
	// before json.Unmarshal is called. If the success field isn't in the
	// input JSON, the json decoder will not touch it and thus it will still
	// be true.
	//
	// The type alias is needed to prevent recursive calls of this method.
	type stageResultAlias StageResult
	stageResultDefault := stageResultAlias{
		Success: true,
	}

	err := json.Unmarshal(data, &stageResultDefault)
	if err != nil {
		return err
	}

	*sr = StageResult(stageResultDefault)

	return nil
}

func (md *PipelineMetadata) UnmarshalJSON(data []byte) error {
	var rawPipelineMetadata map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawPipelineMetadata); err != nil {
		return err
	}
	var pmd PipelineMetadata = make(map[string]StageMetadata)
	for name, rawStageData := range rawPipelineMetadata {
		var metadata StageMetadata
		switch name {
		case "org.osbuild.rpm":
			metadata = new(RPMStageMetadata)
			if err := json.Unmarshal(rawStageData, metadata); err != nil {
				return err
			}
		case "org.osbuild.ostree.commit":
			metadata = new(OSTreeCommitStageMetadata)
			if err := json.Unmarshal(rawStageData, metadata); err != nil {
				return err
			}
		default:
			metadata = RawStageMetadata(rawStageData)
		}
		pmd[name] = metadata
	}
	*md = pmd
	return nil
}

func (res *Result) UnmarshalJSON(data []byte) error {
	// detect if the input is v1 result
	if isV1, err := isV1Result(data); err != nil {
		return err
	} else if isV1 {
		var resv1 v1Result
		if err := json.Unmarshal(data, &resv1); err != nil {
			return err
		}
		return res.fromV1(resv1)
	}

	// otherwise, unmarshal using a type alias to prevent recursive calls to
	// this method
	type resultAlias Result
	var resv2 resultAlias
	if err := json.Unmarshal(data, &resv2); err != nil {
		return err
	}

	*res = Result(resv2)
	return nil
}

func (res *Result) Write(writer io.Writer) error {
	// Error may be included, print them first
	if res != nil && len(res.Errors) > 0 {
		fmt.Fprintf(writer, "Error %s\n", res.Title)
		for _, e := range res.Errors {
			fmt.Fprintf(writer, "%s: %s\n", strings.Join(e.Path, "."), e.Message)
		}
	}

	if res == nil || res.Log == nil {
		fmt.Fprintf(writer, "The compose result is empty.\n")
		return nil
	}

	// The pipeline results don't have a stable order
	// (see https://github.com/golang/go/issues/27179)
	// Sort based on pipeline name to have a stable print order
	pipelineNames := make([]string, 0, len(res.Log))
	for name := range res.Log {
		pipelineNames = append(pipelineNames, name)
	}
	sort.Strings(pipelineNames)

	for _, pipelineName := range pipelineNames {
		fmt.Fprintf(writer, "Pipeline: %s\n", pipelineName)
		pipelineMD := res.Metadata[pipelineName]
		for _, stage := range res.Log[pipelineName] {
			fmt.Fprintf(writer, "Stage: %s\n", stage.Type)
			fmt.Fprintf(writer, "Output:\n%s\n", stage.Output)

			// print structured stage metadata if available
			if pipelineMD != nil {
				if md, ok := pipelineMD[stage.Type]; ok {
					fmt.Fprint(writer, "Metadata:\n")
					enc := json.NewEncoder(writer)
					enc.SetIndent("", "  ")
					err := enc.Encode(md)
					if err != nil {
						return err
					}
				}
			}
			fmt.Fprint(writer, "\n")
		}
	}

	return nil
}

// The ValidationError path from osbuild can contain strings or numbers
// json represents all numbers as float64 but since we know they are really
// ints any fractional part is truncated when converting to a string.
func (ve *ValidationError) UnmarshalJSON(data []byte) error {
	var raw struct {
		Message string
		Path    []interface{}
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	ve.Message = raw.Message

	// Convert the path elements to strings
	var path []string
	for _, p := range raw.Path {
		switch v := p.(type) {
		// json converts numbers, even 0, to float64 not int
		case float64:
			path = append(path, fmt.Sprintf("[%0.0f]", v))
		case string:
			path = append(path, v)
		default:
			return fmt.Errorf("Unexpected type in ValidationError Path: %#v", v)
		}
	}
	ve.Path = path

	return nil
}
