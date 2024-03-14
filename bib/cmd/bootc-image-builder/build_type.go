package main

import (
	"fmt"
	"sort"
)

type BuildType int

const (
	BuildTypeDisk BuildType = iota + 1
	BuildTypeISO
)

var supportedImageTypes = map[string]BuildType{
	"ami":          BuildTypeDisk,
	"qcow2":        BuildTypeDisk,
	"raw":          BuildTypeDisk,
	"vmdk":         BuildTypeDisk,
	"anaconda-iso": BuildTypeISO,
}

// imageTypeAliases contains aliases for our images
var imageTypeAliases = map[string]string{
	"iso": "anaconda-iso", // deprecated
}

func NewBuildType(imageTypes []string) (BuildType, error) {
	if len(imageTypes) == 0 {
		return 0, fmt.Errorf("cannot convert empty array of image types")
	}

	buildType := supportedImageTypes[imageTypes[0]]
	for _, typ := range imageTypes {
		if bt, ok := supportedImageTypes[typ]; ok {
			if buildType != bt { // build types can't be mixed
				return 0, fmt.Errorf("cannot build %q with different target types", typ)
			}
		} else {
			return 0, fmt.Errorf("NewBuildType(): unsupported image type %q", typ)
		}

	}

	return supportedImageTypes[imageTypes[0]], nil
}

// allImageTypesString returns a comma-separated list of supported types
func allImageTypesString() string {
	keys := make([]string, 0, len(supportedImageTypes))
	for k := range supportedImageTypes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	r := ""
	for i, k := range keys {
		if i > 0 {
			r += ", "
		}
		r += k
	}
	return r
}
