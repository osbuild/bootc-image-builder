// Package osbuild provides primitives for representing and (un)marshalling
// OSBuild (schema v2) types.
package osbuild

// A Manifest represents an OSBuild source and pipeline manifest
type Manifest struct {
	Version   string     `json:"version"`
	Pipelines []Pipeline `json:"pipelines"`
	Sources   Sources    `json:"sources"`
}

// A Pipeline represents an OSBuild pipeline
type Pipeline struct {
	Name string `json:"name,omitempty"`
	// The build environment which can run this pipeline
	Build string `json:"build,omitempty"`

	Runner string `json:"runner,omitempty"`

	// Sequence of stages that produce the filesystem tree, which is the
	// payload of the produced image.
	Stages []*Stage `json:"stages,omitempty"`
}

// SetBuild sets the pipeline and runner for generating the build environment
// for a pipeline.
func (p *Pipeline) SetBuild(build string) {
	p.Build = build
}

// AddStage appends a stage to the list of stages of a pipeline. The stages
// will be executed in the order they are appended.
// If the argument is nil, it is not added.
func (p *Pipeline) AddStage(stage *Stage) {
	if stage != nil {
		p.Stages = append(p.Stages, stage)
	}
}

// AddStages appends multiple stages to the list of stages of a pipeline. The
// stages will be executed in the order they are appended.
// If the argument is nil, it is not added.
func (p *Pipeline) AddStages(stages ...*Stage) {
	for _, stage := range stages {
		p.AddStage(stage)
	}
}
