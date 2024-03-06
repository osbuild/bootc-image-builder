package osbuild

// Transforms the tree to an ostree layout

type OSTreePrepTreeStageOptions struct {
	// Array of group names to still keep in /etc/group
	EtcGroupMembers []string `json:"etc_group_members,omitempty"`

	// Array of arguments passed to dracut
	InitramfsArgs []string `json:"initramfs-args,omitempty"`

	// Create a regular directory for /tmp
	TmpIsDir *bool `json:"tmp-is-dir,omitempty"`
}

func (OSTreePrepTreeStageOptions) isStageOptions() {}

// The OSTree PrepTree (org.osbuild.ostree.preptree) stage transforms the
// tree to an ostree layout.
func NewOSTreePrepTreeStage(options *OSTreePrepTreeStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.preptree",
		Options: options,
	}
}
