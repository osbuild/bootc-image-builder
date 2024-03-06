package osbuild

import (
	"encoding/json"
	"errors"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
)

// A Sources map contains all the sources made available to an osbuild run
type Sources map[string]Source

// Source specifies the operations of a given source-type.
type Source interface {
	isSource()
}

type SourceOptions interface {
	isSourceOptions()
}

type rawSources map[string]json.RawMessage

// UnmarshalJSON unmarshals JSON into a Source object. Each type of source has
// a custom unmarshaller for its options, selected based on the source name.
func (sources *Sources) UnmarshalJSON(data []byte) error {
	var rawSources rawSources
	err := json.Unmarshal(data, &rawSources)
	if err != nil {
		return err
	}
	*sources = make(map[string]Source)
	for name, rawSource := range rawSources {
		var source Source
		switch name {
		case "org.osbuild.curl":
			source = new(CurlSource)
		case "org.osbuild.inline":
			source = new(InlineSource)
		case "org.osbuild.ostree":
			source = new(OSTreeSource)
		default:
			return errors.New("unexpected source name: " + name)
		}
		err = json.Unmarshal(rawSource, source)
		if err != nil {
			return err
		}
		(*sources)[name] = source
	}

	return nil
}

func GenSources(packages []rpmmd.PackageSpec, ostreeCommits []ostree.CommitSpec, inlineData []string, containers []container.Spec) (Sources, error) {
	sources := Sources{}

	// collect rpm package sources
	if len(packages) > 0 {
		curl := NewCurlSource()
		for _, pkg := range packages {
			err := curl.AddPackage(pkg)
			if err != nil {
				return nil, err
			}
		}
		sources["org.osbuild.curl"] = curl
	}

	// collect ostree commit sources
	if len(ostreeCommits) > 0 {
		ostree := NewOSTreeSource()
		for _, commit := range ostreeCommits {
			ostree.AddItem(commit)
		}
		if len(ostree.Items) > 0 {
			sources["org.osbuild.ostree"] = ostree
		}
	}

	// collect inline data sources
	if len(inlineData) > 0 {
		ils := NewInlineSource()
		for _, data := range inlineData {
			ils.AddItem(data)
		}

		sources["org.osbuild.inline"] = ils
	}

	// collect skopeo container sources
	if len(containers) > 0 {
		skopeo := NewSkopeoSource()
		skopeoIndex := NewSkopeoIndexSource()
		for _, c := range containers {
			skopeo.AddItem(c.Source, c.Digest, c.ImageID, c.TLSVerify, c.ContainersTransport, c.StoragePath)

			// if we have a list digest, add a skopeo-index source as well
			if c.ListDigest != "" {
				skopeoIndex.AddItem(c.Source, c.ListDigest, c.TLSVerify, c.ContainersTransport, c.StoragePath)
			}
		}
		if len(skopeo.Items) > 0 {
			sources["org.osbuild.skopeo"] = skopeo
		}
		if len(skopeoIndex.Items) > 0 {
			sources["org.osbuild.skopeo-index"] = skopeoIndex
		}
	}

	return sources, nil
}
