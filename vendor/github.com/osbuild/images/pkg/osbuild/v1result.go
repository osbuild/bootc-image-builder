package osbuild

// Functions and types for handling old (v1) results.

import (
	"encoding/json"
)

type v1StageResult struct {
	Name     string          `json:"name"`
	Options  json.RawMessage `json:"options"`
	Success  bool            `json:"success"`
	Output   string          `json:"output"`
	Metadata json.RawMessage `json:"metadata"`
}

type v1BuildResult struct {
	Stages  []v1StageResult `json:"stages"`
	TreeID  string          `json:"tree_id"`
	Success bool            `json:"success"`
}

type v1Result struct {
	TreeID    string          `json:"tree_id"`
	OutputID  string          `json:"output_id"`
	Build     *v1BuildResult  `json:"build"`
	Stages    []v1StageResult `json:"stages"`
	Assembler *v1StageResult  `json:"assembler"`
	Success   bool            `json:"success"`
}

func (res *Result) fromV1(resv1 v1Result) error {
	res.Success = resv1.Success
	res.Type = "result"

	log := make(map[string]PipelineResult)
	metadata := make(map[string]PipelineMetadata)

	// make build pipeline from build result
	if resv1.Build != nil {
		buildResult, buildMetadata, err := convertStageResults(resv1.Build.Stages)
		if err != nil {
			return err
		}
		log["build"] = buildResult
		if len(buildMetadata) > 0 {
			metadata["build"] = buildMetadata
		}
	}

	// make assembler pipeline from assembler result
	if resv1.Assembler != nil {
		assemblerResult, assemblerMetadata, err := convertStageResult(resv1.Assembler)
		if err != nil {
			return err
		}
		log["assembler"] = []StageResult{*assemblerResult}
		if assemblerMetadata != nil {
			metadata["assembler"] = map[string]StageMetadata{
				resv1.Assembler.Name: assemblerMetadata,
			}
		}
	}

	// make os pipeline from main stage results
	if len(resv1.Stages) > 0 {
		osResult, osMetadata, err := convertStageResults(resv1.Stages)
		if err != nil {
			return err
		}
		log["os"] = osResult
		if len(osMetadata) > 0 {
			metadata["os"] = osMetadata
		}
	}

	res.Log = log
	res.Metadata = metadata
	return nil
}

func convertStageResults(v1Stages []v1StageResult) (PipelineResult, PipelineMetadata, error) {
	result := make([]StageResult, len(v1Stages))
	metadata := make(map[string]StageMetadata)
	for idx, srv1 := range v1Stages {
		// Implicit memory alasing doesn't couse any bug in this case
		/* #nosec G601 */
		stageResult, stageMetadata, err := convertStageResult(&srv1)
		if err != nil {
			return nil, nil, err
		}
		result[idx] = *stageResult
		if stageMetadata != nil {
			metadata[stageResult.Type] = stageMetadata
		}
	}
	return result, metadata, nil
}

func convertStageResult(sr1 *v1StageResult) (*StageResult, StageMetadata, error) {
	sr := &StageResult{
		ID:      "",
		Type:    sr1.Name,
		Output:  sr1.Output,
		Success: sr1.Success,
		Error:   "",
	}

	var metadata StageMetadata
	switch sr1.Name {
	case "org.osbuild.rpm":
		metadata = new(RPMStageMetadata)
		if err := json.Unmarshal(sr1.Metadata, metadata); err != nil {
			return nil, nil, err
		}
	case "org.osbuild.ostree.commit":
		metadata = new(OSTreeCommitStageMetadata)
		if err := json.Unmarshal(sr1.Metadata, metadata); err != nil {
			return nil, nil, err
		}
	default:
		metadata = RawStageMetadata(sr1.Metadata)
	}

	return sr, metadata, nil
}

// isV1Result returns true if data contains a json-encoded osbuild result
// in version 1 schema.
//
// It detects the schema version by checking if the decoded json contains at
// least one of the three top-level result objects: Build, Stages, or Assembler
//
// error is non-nil when data isn't a json-encoded object.
func isV1Result(data []byte) (bool, error) {
	var v1ResultStub struct {
		Build     interface{} `json:"build"`
		Stages    interface{} `json:"stages"`
		Assembler interface{} `json:"assembler"`
	}

	err := json.Unmarshal(data, &v1ResultStub)
	if err != nil {
		return false, err
	}

	return v1ResultStub.Build != nil || v1ResultStub.Stages != nil || v1ResultStub.Assembler != nil, nil
}
