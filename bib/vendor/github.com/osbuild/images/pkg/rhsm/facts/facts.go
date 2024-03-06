package facts

import "fmt"

type APIType uint64

func (at APIType) String() string {
	switch at {
	case TEST_APITYPE:
		return "test-manifest"
	case CLOUDV2_APITYPE:
		return "cloudapi-v2"
	case WELDR_APITYPE:
		return "weldr"
	}
	panic(fmt.Sprintf("invalid APIType value %d", at))
}

const (
	TEST_APITYPE APIType = iota
	CLOUDV2_APITYPE
	WELDR_APITYPE
)

// The ImageOptions specify things to be stored into the Insights facts
// storage. This mostly relates to how the build of the image was performed.
type ImageOptions struct {
	APIType APIType
}
