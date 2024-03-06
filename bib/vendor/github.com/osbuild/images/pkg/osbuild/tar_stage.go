package osbuild

import "fmt"

type TarArchiveFormat string

// valid values for the 'format' Tar stage option
const (
	TarArchiveFormatGnu    TarArchiveFormat = "gnu"
	TarArchiveFormatOldgnu TarArchiveFormat = "oldgnu"
	TarArchiveFormatPosix  TarArchiveFormat = "posix"
	TarArchiveFormatUstar  TarArchiveFormat = "ustar"
	TarArchiveFormatV7     TarArchiveFormat = "v7"
)

type TarRootNode string

// valid values for the 'root-node' Tar stage option
const (
	TarRootNodeInclude TarRootNode = "include"
	TarRootNodeOmit    TarRootNode = "omit"
)

type TarStageOptions struct {
	// Filename for tar archive
	Filename string `json:"filename"`

	// Archive format to use
	Format TarArchiveFormat `json:"format,omitempty"`

	// Enable support for POSIX ACLs
	ACLs *bool `json:"acls,omitempty"`

	// Enable support for SELinux contexts
	SELinux *bool `json:"selinux,omitempty"`

	// Enable support for extended attributes
	Xattrs *bool `json:"xattrs,omitempty"`

	// How to handle the root node: include or omit
	RootNode TarRootNode `json:"root-node,omitempty"`

	// List of paths to include, instead of the whole tree
	Paths []string `json:"paths,omitempty"`
}

func (TarStageOptions) isStageOptions() {}

func (o TarStageOptions) validate() error {
	if o.Format != "" {
		allowedArchiveFormatValues := []TarArchiveFormat{
			TarArchiveFormatGnu,
			TarArchiveFormatOldgnu,
			TarArchiveFormatPosix,
			TarArchiveFormatUstar,
			TarArchiveFormatV7,
		}
		valid := false
		for _, value := range allowedArchiveFormatValues {
			if o.Format == value {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("'format' option does not allow %q as a value", o.Format)
		}
	}

	if o.RootNode != "" {
		allowedRootNodeValues := []TarRootNode{
			TarRootNodeInclude,
			TarRootNodeOmit,
		}
		valid := false
		for _, value := range allowedRootNodeValues {
			if o.RootNode == value {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("'root-node' option does not allow %q as a value", o.RootNode)
		}
	}

	if len(o.Paths) > 0 && o.RootNode != "" {
		return fmt.Errorf("'paths' cannot be combined with 'root-node'")
	}

	return nil
}

// Assembles a tree into a tar archive. Compression is determined by the suffix
// (i.e., --auto-compress is used).
func NewTarStage(options *TarStageOptions, inputPipeline string) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.tar",
		Options: options,
		Inputs:  NewPipelineTreeInputs("tree", inputPipeline),
	}
}
