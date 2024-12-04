package progress_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/bootc-image-builder/bib/internal/progress"
)

func TestProgressNew(t *testing.T) {
	for _, tc := range []struct {
		Type     string
		Expected interface{}
	}{
		{"text", &progress.PbProgressBar{}},
		{"debug", &progress.DebugProgressBar{}},
		{"null", &progress.NullProgressBar{}},
		{"bad", nil},
	} {
		pb := progress.New(tc.Type)
		assert.Equal(t, reflect.TypeOf(pb), reflect.TypeOf(tc.Expected), fmt.Sprintf("[%v] %T not the expected %T", tc.Type, pb, tc.Expected))
	}
}
