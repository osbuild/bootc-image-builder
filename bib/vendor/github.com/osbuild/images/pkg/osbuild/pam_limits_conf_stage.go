package osbuild

import (
	"encoding/json"
	"fmt"
)

// PamLimitsConfStageOptions represents a single pam_limits module configuration file.
type PamLimitsConfStageOptions struct {
	// Filename of the configuration file to be created. Must end with '.conf'.
	Filename string `json:"filename"`
	// List of configuration directives. The list must contain at least one item.
	Config []PamLimitsConfigLine `json:"config"`
}

func (PamLimitsConfStageOptions) isStageOptions() {}

// NewPamLimitsConfStageOptions creates a new PamLimitsConf Stage options object.
func NewPamLimitsConfStageOptions(filename string, config []PamLimitsConfigLine) *PamLimitsConfStageOptions {
	return &PamLimitsConfStageOptions{
		Filename: filename,
		Config:   config,
	}
}

// Unexported alias for use in PamLimitsConfStageOptions's MarshalJSON() to prevent recursion
type pamLimitsConfStageOptions PamLimitsConfStageOptions

func (o PamLimitsConfStageOptions) MarshalJSON() ([]byte, error) {
	if len(o.Config) == 0 {
		return nil, fmt.Errorf("the 'Config' list must contain at least one item")
	}
	options := pamLimitsConfStageOptions(o)
	return json.Marshal(options)
}

// NewPamLimitsConfStage creates a new PamLimitsConf Stage object.
func NewPamLimitsConfStage(options *PamLimitsConfStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.pam.limits.conf",
		Options: options,
	}
}

type PamLimitsType string

// Valid 'Type' values for the use with the PamLimitsConfigLine structure.
const (
	PamLimitsTypeHard PamLimitsType = "hard"
	PamLimitsTypeSoft PamLimitsType = "soft"
	PamLimitsTypeBoth PamLimitsType = "-"
)

type PamLimitsItem string

// Valid 'Item' values for the use with the PamLimitsConfigLine structure.
const (
	PamLimitsItemCore         PamLimitsItem = "core"
	PamLimitsItemData         PamLimitsItem = "data"
	PamLimitsItemFsize        PamLimitsItem = "fsize"
	PamLimitsItemMemlock      PamLimitsItem = "memlock"
	PamLimitsItemNofile       PamLimitsItem = "nofile"
	PamLimitsItemRss          PamLimitsItem = "rss"
	PamLimitsItemStack        PamLimitsItem = "stack"
	PamLimitsItemCpu          PamLimitsItem = "cpu"
	PamLimitsItemNproc        PamLimitsItem = "nproc"
	PamLimitsItemAs           PamLimitsItem = "as"
	PamLimitsItemMaxlogins    PamLimitsItem = "maxlogins"
	PamLimitsItemMaxsyslogins PamLimitsItem = "maxsyslogins"
	PamLimitsItemNonewprivs   PamLimitsItem = "nonewprivs"
	PamLimitsItemPriority     PamLimitsItem = "priority"
	PamLimitsItemLocks        PamLimitsItem = "locks"
	PamLimitsItemSigpending   PamLimitsItem = "sigpending"
	PamLimitsItemMsgqueue     PamLimitsItem = "msgqueue"
	PamLimitsItemNice         PamLimitsItem = "nice"
	PamLimitsItemRtprio       PamLimitsItem = "rtprio"
)

// PamLimitsValue is defined to represent all valid types of the 'Value'
// item in the PamLimitsConfigLine structure.
type PamLimitsValue interface {
	isPamLimitsValue()
}

// PamLimitsValueStr represents a string type of the 'Value' item in
// the PamLimitsConfigLine structure.
type PamLimitsValueStr string

func (v PamLimitsValueStr) isPamLimitsValue() {}

// Valid string values which can be used for the 'Value' item in
// the PamLimitsConfigLine structure.
const (
	PamLimitsValueUnlimited PamLimitsValueStr = "unlimited"
	PamLimitsValueInfinity  PamLimitsValueStr = "infinity"
)

// PamLimitsValueInt represents an integer type of the 'Value' item in
// the PamLimitsConfigLine structure.
type PamLimitsValueInt int

func (v PamLimitsValueInt) isPamLimitsValue() {}

// PamLimitsConfigLine represents a single line in a pam_limits module configuration.
type PamLimitsConfigLine struct {
	// Domain to which the limit applies. E.g. username, groupname, etc.
	Domain string `json:"domain"`
	// Type of the limit.
	Type PamLimitsType `json:"type"`
	// The resource type, which is being limited.
	Item PamLimitsItem `json:"item"`
	// The limit value.
	Value PamLimitsValue `json:"value"`
}

// Unexported struct used for Unmarshalling of PamLimitsConfigLine due to
// 'value' being an integer or a string.
type rawPamLimitsConfigLine struct {
	// Domain to which the limit applies. E.g. username, groupname, etc.
	Domain string `json:"domain"`
	// Type of the limit.
	Type PamLimitsType `json:"type"`
	// The resource type, which is being limited.
	Item PamLimitsItem `json:"item"`
	// The limit value.
	Value interface{} `json:"value"`
}

func (l *PamLimitsConfigLine) UnmarshalJSON(data []byte) error {
	var rawLine rawPamLimitsConfigLine
	if err := json.Unmarshal(data, &rawLine); err != nil {
		return err
	}

	var value PamLimitsValue
	switch valueType := rawLine.Value.(type) {
	// json.Unmarshal() uses float64 for JSON numbers
	// https://pkg.go.dev/encoding/json#Unmarshal
	// However the expected value is only integer.
	case float64:
		value = PamLimitsValueInt(rawLine.Value.(float64))
	case string:
		value = PamLimitsValueStr(rawLine.Value.(string))
	default:
		return fmt.Errorf("the 'value' item has unsupported type %q", valueType)
	}

	l.Domain = rawLine.Domain
	l.Type = rawLine.Type
	l.Item = rawLine.Item
	l.Value = value

	return nil
}
