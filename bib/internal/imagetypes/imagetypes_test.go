package imagetypes_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/bootc-image-builder/bib/internal/imagetypes"
)

type testCase struct {
	imageTypes      []string
	expectedExports []string
	expectISO       bool
	expectedErr     error
}

func TestImageTypes(t *testing.T) {
	testCases := map[string]testCase{
		"qcow-disk": {
			imageTypes:      []string{"qcow2"},
			expectedExports: []string{"qcow2"},
			expectISO:       false,
		},
		"ami-disk": {
			imageTypes:      []string{"ami"},
			expectedExports: []string{"image"},
			expectISO:       false,
		},
		"qcow-ami-disk": {
			imageTypes:      []string{"qcow2", "ami"},
			expectedExports: []string{"qcow2", "image"},
			expectISO:       false,
		},
		"ami-raw": {
			imageTypes:      []string{"ami", "raw"},
			expectedExports: []string{"image"},
			expectISO:       false,
		},
		"all-disk": {
			imageTypes:      []string{"ami", "raw", "vmdk", "qcow2"},
			expectedExports: []string{"image", "vmdk", "qcow2"},
			expectISO:       false,
		},
		"iso": {
			imageTypes:      []string{"iso"},
			expectedExports: []string{"bootiso"},
			expectISO:       true,
		},
		"anaconda": {
			imageTypes:      []string{"anaconda-iso"},
			expectedExports: []string{"bootiso"},
			expectISO:       true,
		},
		"bad-mix": {
			imageTypes:  []string{"vmdk", "anaconda-iso"},
			expectedErr: errors.New("cannot mix ISO/disk images in request [vmdk anaconda-iso]"),
		},
		"bad-mix-2": {
			imageTypes:  []string{"vmdk", "bootc-installer"},
			expectedErr: errors.New("cannot mix ISO/disk images in request [vmdk bootc-installer]"),
		},
		"bad-mix-3": {
			imageTypes:  []string{"ami", "iso"},
			expectedErr: errors.New("cannot mix ISO/disk images in request [ami iso]"),
		},
		"bad-image-type": {
			imageTypes:  []string{"bad"},
			expectedErr: errors.New(`unsupported image type "bad", valid types are ami, anaconda-iso, bootc-installer, gce, iso, ova, qcow2, raw, vhd, vmdk`),
		},
		"bad-in-good": {
			imageTypes:  []string{"ami", "raw", "vmdk", "qcow2", "something-else-what-is-this"},
			expectedErr: errors.New(`unsupported image type "something-else-what-is-this", valid types are ami, anaconda-iso, bootc-installer, gce, iso, ova, qcow2, raw, vhd, vmdk`),
		},
		"all-bad": {
			imageTypes:  []string{"bad1", "bad2", "bad3", "bad4", "bad5", "bad42"},
			expectedErr: errors.New(`unsupported image type "bad1", valid types are ami, anaconda-iso, bootc-installer, gce, iso, ova, qcow2, raw, vhd, vmdk`),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			it, err := imagetypes.New(tc.imageTypes...)
			if tc.expectedErr != nil {
				assert.Equal(t, err, tc.expectedErr)
			} else {
				assert.Equal(t, it.Exports(), tc.expectedExports)
				assert.Equal(t, it.BuildsISO(), tc.expectISO)
				assert.NoError(t, err)
			}
		})
	}
}
