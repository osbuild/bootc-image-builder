package main

import (
	"fmt"
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
	"iso":          BuildTypeISO,
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
