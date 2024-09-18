package imagetypes

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

type imageType struct {
	Export string
	ISO    bool
}

var supportedImageTypes = map[string]imageType{
	"ami":          imageType{Export: "image"},
	"qcow2":        imageType{Export: "qcow2"},
	"raw":          imageType{Export: "image"},
	"vmdk":         imageType{Export: "vmdk"},
	"vhd":          imageType{Export: "vpc"},
	"anaconda-iso": imageType{Export: "bootiso", ISO: true},
	"iso":          imageType{Export: "bootiso", ISO: true},
}

// Available() returns a comma-separated list of supported image types
func Available() string {
	keys := make([]string, 0, len(supportedImageTypes))
	for k := range supportedImageTypes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return strings.Join(keys, ", ")
}

// ImageTypes contains the image types that are requested to be build
type ImageTypes []string

// New takes image type names as input and returns a ImageTypes
// object or an error if the image types are invalid.
//
// Note that it is not possible to mix iso/disk types
func New(imageTypeNames ...string) (ImageTypes, error) {
	if len(imageTypeNames) == 0 {
		return nil, fmt.Errorf("cannot use an empty array as a build request")
	}

	var ISOs, disks int
	for _, name := range imageTypeNames {
		imgType, ok := supportedImageTypes[name]
		if !ok {
			return nil, fmt.Errorf("unsupported image type %q, valid types are %s", name, Available())
		}
		if imgType.ISO {
			ISOs++
		} else {
			disks++
		}
	}
	if ISOs > 0 && disks > 0 {
		return nil, fmt.Errorf("cannot mix ISO/disk images in request %v", imageTypeNames)
	}

	return ImageTypes(imageTypeNames), nil
}

// Exports returns the list of osbuild manifest exports require to build
// all images types.
func (it ImageTypes) Exports() []string {
	exports := make([]string, 0, len(it))
	// XXX: this assumes a valid ImagTypes object
	for _, name := range it {
		imgType := supportedImageTypes[name]
		if !slices.Contains(exports, imgType.Export) {
			exports = append(exports, imgType.Export)
		}
	}

	return exports
}

// BuildsISO returns true if the image types build an ISO, note that
// it is not possible to mix disk/iso.
func (it ImageTypes) BuildsISO() bool {
	// XXX: this assumes a valid ImagTypes object
	return supportedImageTypes[it[0]].ISO
}
