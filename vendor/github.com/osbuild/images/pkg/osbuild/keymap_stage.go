package osbuild

import (
	"encoding/json"
	"fmt"
)

type KeymapStageOptions struct {
	Keymap    string            `json:"keymap"`
	X11Keymap *X11KeymapOptions `json:"x11-keymap,omitempty"`
}

func (KeymapStageOptions) isStageOptions() {}

func NewKeymapStage(options *KeymapStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.keymap",
		Options: options,
	}
}

type X11KeymapOptions struct {
	Layouts []string `json:"layouts"`
}

// Unexported alias for use in X11KeymapOptions's MarshalJSON() to prevent recursion
type x11KeymapOptions X11KeymapOptions

func (o X11KeymapOptions) MarshalJSON() ([]byte, error) {
	if len(o.Layouts) == 0 {
		return nil, fmt.Errorf("at least one layout must be provided for X11 keymap")
	}
	keymapOptions := x11KeymapOptions(o)
	return json.Marshal(keymapOptions)
}
