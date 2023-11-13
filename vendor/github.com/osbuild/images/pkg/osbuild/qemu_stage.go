package osbuild

import (
	"encoding/json"
	"fmt"
)

// Convert a disk image to a different format.
//
// Some formats support format-specific options:
//   qcow2: The compatibility version can be specified via 'compat'

type QEMUStageOptions struct {
	// Filename for resulting image
	Filename string `json:"filename"`

	// Image format and options
	Format QEMUFormatOptions `json:"format"`
}

func (QEMUStageOptions) isStageOptions() {}

type QEMUFormat string
type VMDKSubformat string

const (
	QEMUFormatQCOW2 QEMUFormat = "qcow2"
	QEMUFormatVDI   QEMUFormat = "vdi"
	QEMUFormatVMDK  QEMUFormat = "vmdk"
	QEMUFormatVPC   QEMUFormat = "vpc"
	QEMUFormatVHDX  QEMUFormat = "vhdx"

	VMDKSubformatMonolithicSparse     VMDKSubformat = "monolithicSparse"
	VMDKSubformatMonolithicFlat       VMDKSubformat = "monolithicFlat"
	VMDKSubformatTwoGbMaxExtentSparse VMDKSubformat = "twoGbMaxExtentSparse"
	VMDKSubformatTwoGbMaxExtentFlat   VMDKSubformat = "twoGbMaxExtentFlat"
	VMDKSubformatStreamOptimized      VMDKSubformat = "streamOptimized"
)

type QEMUFormatOptions interface {
	isQEMUFormatOptions()
	validate() error
	formatType() QEMUFormat
}

type QCOW2Options struct {
	// The type of the format must be 'qcow2'
	Type QEMUFormat `json:"type"`

	// The qcow2-compatibility-version to use
	Compat string `json:"compat"`
}

func (QCOW2Options) isQEMUFormatOptions() {}

func (o QCOW2Options) validate() error {
	if o.Type != QEMUFormatQCOW2 {
		return fmt.Errorf("invalid format type %q for %q options", o.Type, QEMUFormatQCOW2)
	}
	return nil
}

func (o QCOW2Options) formatType() QEMUFormat {
	return o.Type
}

type VDIOptions struct {
	// The type of the format must be 'vdi'
	Type QEMUFormat `json:"type"`
}

func (VDIOptions) isQEMUFormatOptions() {}

func (o VDIOptions) validate() error {
	if o.Type != QEMUFormatVDI {
		return fmt.Errorf("invalid format type %q for %q options", o.Type, QEMUFormatVDI)
	}
	return nil
}

func (o VDIOptions) formatType() QEMUFormat {
	return o.Type
}

type VPCOptions struct {
	// The type of the format must be 'vpc'
	Type QEMUFormat `json:"type"`

	// VPC related options
	ForceSize *bool `json:"force_size,omitempty"`
}

func (VPCOptions) isQEMUFormatOptions() {}

func (o VPCOptions) validate() error {
	if o.Type != QEMUFormatVPC {
		return fmt.Errorf("invalid format type %q for %q options", o.Type, QEMUFormatVPC)
	}
	return nil
}

func (o VPCOptions) formatType() QEMUFormat {
	return o.Type
}

type VMDKOptions struct {
	// The type of the format must be 'vmdk'
	Type QEMUFormat `json:"type"`

	Subformat VMDKSubformat `json:"subformat,omitempty"`
}

func (VMDKOptions) isQEMUFormatOptions() {}

func (o VMDKOptions) validate() error {
	if o.Type != QEMUFormatVMDK {
		return fmt.Errorf("invalid format type %q for %q options", o.Type, QEMUFormatVMDK)
	}

	if o.Subformat != "" {
		allowedVMDKSubformats := []VMDKSubformat{
			VMDKSubformatMonolithicFlat,
			VMDKSubformatMonolithicSparse,
			VMDKSubformatTwoGbMaxExtentFlat,
			VMDKSubformatTwoGbMaxExtentSparse,
			VMDKSubformatStreamOptimized,
		}
		valid := false
		for _, value := range allowedVMDKSubformats {
			if o.Subformat == value {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("'subformat' option does not allow %q as a value", o.Subformat)
		}
	}

	return nil
}

func (o VMDKOptions) formatType() QEMUFormat {
	return o.Type
}

type VHDXOptions struct {
	// The type of the format must be 'vhdx'
	Type QEMUFormat `json:"type"`
}

func (VHDXOptions) isQEMUFormatOptions() {}

func (o VHDXOptions) validate() error {
	if o.Type != QEMUFormatVHDX {
		return fmt.Errorf("invalid format type %q for %q options", o.Type, QEMUFormatVHDX)
	}
	return nil
}

func (o VHDXOptions) formatType() QEMUFormat {
	return o.Type
}

type QEMUStageInputs struct {
	Image *FilesInput `json:"image"`
}

func (QEMUStageInputs) isStageInputs() {}

// NewQEMUStage creates a new QEMU Stage object.
func NewQEMUStage(options *QEMUStageOptions, inputs *QEMUStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.qemu",
		Options: options,
		Inputs:  inputs,
	}
}

// NewQEMUStageOptions creates a new QEMU Stage options object.
//
// In case the format-specific options are provided, they are used for
// the Stage options.
func NewQEMUStageOptions(filename string, format QEMUFormat, formatOptions QEMUFormatOptions) *QEMUStageOptions {
	if formatOptions != nil {
		// If the format type is not set explicitly in the provided format
		// options, set it to the appropriate value based on the format options
		// object type.
		if formatOptions.formatType() == "" {
			switch o := formatOptions.(type) {
			case QCOW2Options:
				o.Type = QEMUFormatQCOW2
				formatOptions = o
			case VDIOptions:
				o.Type = QEMUFormatVDI
				formatOptions = o
			case VPCOptions:
				o.Type = QEMUFormatVPC
				formatOptions = o
			case VMDKOptions:
				o.Type = QEMUFormatVMDK
				formatOptions = o
			case VHDXOptions:
				o.Type = QEMUFormatVHDX
				formatOptions = o
			default:
				panic(fmt.Sprintf("unknown format options type in qemu stage: %t", o))
			}
		}

		// Ensure that the explicitly provided QEMU format and the format set
		// in the format options structure (set by user or by this function
		// above) are matching.
		if t := formatOptions.formatType(); t != format {
			panic(fmt.Sprintf("mismatch between passed format type %q and format options type %q", format, t))
		}

		if err := formatOptions.validate(); err != nil {
			panic(err)
		}
	} else {
		switch format {
		case QEMUFormatQCOW2:
			formatOptions = QCOW2Options{Type: format}
		case QEMUFormatVDI:
			formatOptions = VDIOptions{Type: format}
		case QEMUFormatVPC:
			formatOptions = VPCOptions{Type: format}
		case QEMUFormatVMDK:
			formatOptions = VMDKOptions{Type: format}
		case QEMUFormatVHDX:
			formatOptions = VHDXOptions{Type: format}
		default:
			panic("unknown format in qemu stage: " + format)
		}
	}

	return &QEMUStageOptions{
		Filename: filename,
		Format:   formatOptions,
	}
}

// alias for custom marshaller
type qemuStageOptions QEMUStageOptions

// Custom marshaller for validating
func (options QEMUStageOptions) MarshalJSON() ([]byte, error) {
	if err := options.Format.validate(); err != nil {
		return nil, err
	}

	return json.Marshal(qemuStageOptions(options))
}

func NewQemuStagePipelineFilesInputs(pipeline, file string) *QEMUStageInputs {
	input := NewFilesInput(NewFilesInputPipelineObjectRef(pipeline, file, nil))
	return &QEMUStageInputs{Image: input}
}
