package osbuild

// Collection of Inputs for a Stage
type Inputs interface {
	isStageInputs()
}

// Single Input for a Stage
type Input interface {
	isInput()
}

// TODO: define these using type aliases
const (
	InputOriginSource   string = "org.osbuild.source"
	InputOriginPipeline string = "org.osbuild.pipeline"
)

// Fields shared between all Input types (should be embedded in each instance)
type inputCommon struct {
	// osbuild input type
	Type string `json:"type"`
	// Origin should be either 'org.osbuild.source' or 'org.osbuild.pipeline'
	Origin string `json:"origin"`
}

type StageInput interface {
	isStageInput()
}

type References interface {
	isReferences()
}
