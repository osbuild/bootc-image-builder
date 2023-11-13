package osbuild

import (
	"fmt"
	"regexp"
)

type udevOpType int

const (
	udevOpMatch  udevOpType = 0
	udevOpAssign udevOpType = 1
)

var ops = map[string]udevOpType{
	"=":  udevOpAssign,
	"+=": udevOpAssign,
	"-=": udevOpAssign,
	":=": udevOpAssign,
	"==": udevOpMatch,
	"!=": udevOpMatch,
}

type udevKeyType struct {
	Arg    bool
	Assign bool
	Match  bool
}

var keys = map[string]udevKeyType{
	"ACTION":     {Match: true},
	"DEVPATH":    {Match: true},
	"KERNEL":     {Match: true},
	"KERNELS":    {Match: true},
	"NAME":       {Match: true, Assign: true},
	"SYMLINK":    {Match: true, Assign: true},
	"SUBSYSTEM":  {Match: true},
	"SUBSYSTEMS": {Match: true},
	"DRIVER":     {Match: true},
	"DRIVERS":    {Match: true},
	"TAG":        {Match: true, Assign: true},
	"TAGS":       {Match: true},
	"PROGRAM":    {Match: true},
	"RESULT":     {Match: true},

	"ATTR":   {Arg: true, Match: true, Assign: true},
	"ATTRS":  {Arg: true, Match: true},
	"SYSCTL": {Arg: true, Match: true, Assign: true},
	"ENV":    {Arg: true, Match: true, Assign: true},
	"CONST":  {Arg: true, Match: true},
	"TEST":   {Arg: true, Match: true},

	"OWNER":   {Assign: true},
	"GROUP":   {Assign: true},
	"MODE":    {Assign: true},
	"LABEL":   {Assign: true},
	"GOTO":    {Assign: true},
	"OPTIONS": {Assign: true},

	"SECLABEL": {Arg: true, Assign: true},
	"RUN":      {Arg: true, Assign: true},
	"IMPORT":   {Arg: true, Assign: true},
}

func udevOpValidate(key, op, val, arg string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if op == "" {
		return fmt.Errorf("operator is required")
	}
	if val == "" {
		return fmt.Errorf("value is required")
	}

	keyInfo, ok := keys[key]

	if !ok {
		return fmt.Errorf("key '%s' is unknown", key)
	}

	if keyInfo.Arg && arg == "" {
		return fmt.Errorf("arg is required for key '%s'", key)
	}

	opType, ok := ops[op]

	if !ok {
		return fmt.Errorf("'%s' operator is not supported", op)
	}

	if (opType == udevOpMatch && !keyInfo.Match) ||
		(opType == udevOpAssign && !keyInfo.Assign) {
		return fmt.Errorf("key '%s' does not support '%s'", key, op)
	}

	return nil
}

type UdevRulesStageOptions struct {
	Filename string    `json:"filename"`
	Rules    UdevRules `json:"rules"`
}

func (UdevRulesStageOptions) isStageOptions() {}

func (o UdevRulesStageOptions) validate() error {
	if len(o.Rules) == 0 {
		return fmt.Errorf("at least one rule is required")
	}

	re := regexp.MustCompile(`^[.\/\w\-_]{1,250}.rules$`)
	if !re.MatchString(o.Filename) {
		return fmt.Errorf("udev.rules filename '%q' doesn't conform to schema '%s'", o.Filename, re.String())
	}

	return nil
}

func NewUdevRulesStage(options *UdevRulesStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.udev.rules",
		Options: options,
	}
}

type UdevRules []UdevRule

type UdevRule interface {
	isUdevRule()
}

// Comments
type UdevRuleComment struct {
	Comment []string `json:"comment"`
}

func (UdevRuleComment) isUdevRule() {}

func NewUdevRuleComment(comment []string) UdevRule {
	return UdevRuleComment{
		Comment: comment,
	}
}

// Match and Assignments

type UdevOps []UdevOp

func (UdevOps) isUdevRule() {}

type UdevOp interface {
	isUdevOp()
	validate() error
}

type UdevRuleKey interface {
	isUdevRuleKey()
}

type UdevRuleKeySimple struct {
	Key string `json:"key"`
}

func (UdevRuleKeySimple) isUdevRuleKey() {}

type UdevRuleKeyArg struct {
	Name string `json:"name"`
	Arg  string `json:"arg"`
}

func (UdevRuleKeyArg) isUdevRuleKey() {}

type UdevOpSimple struct {
	Key   string `json:"key"`
	Op    string `json:"op"`
	Value string `json:"val"`
}

func (o UdevOpSimple) validate() error {
	err := udevOpValidate(o.Key, o.Op, o.Value, "")
	if err != nil {
		err = fmt.Errorf("invalid op: %v", err)
	}

	return err
}

func (UdevOpSimple) isUdevOp() {}

type UdevOpArg struct {
	Key   UdevRuleKeyArg `json:"key"`
	Op    string         `json:"op"`
	Value string         `json:"val"`
}

func (UdevOpArg) isUdevOp() {}

func (o UdevOpArg) validate() error {
	err := udevOpValidate(o.Key.Name, o.Op, o.Value, o.Key.Arg)
	if err != nil {
		err = fmt.Errorf("invalid op: %v", err)
	}

	return err
}

// UdevKV is a helper struct that in order to be able to create a UdevRule
// more compactly
type UdevKV struct {
	K string // Key, e.g. "ENV"
	A string // Argument for the key, MANAGED, in `ENV{MANAGED}`
	O string // Operator, e.g. "="
	V string // Value, e.g. "1"
}

// NewUdevRule creates a new UdevRule from a list of UdevKV
// helper structs. A UdevOpSimple or a UdevOpArg is created
// depending on the value of the `A` field. The result is
// validated and the function will panic if validation fails.
func NewUdevRule(ops []UdevKV) UdevRule {
	res := make(UdevOps, 0, len(ops))

	for _, o := range ops {

		var op UdevOp

		if o.A == "" {
			op = UdevOpSimple{
				Key:   o.K,
				Op:    o.O,
				Value: o.V,
			}
		} else {
			op = UdevOpArg{
				Key: UdevRuleKeyArg{
					Name: o.K,
					Arg:  o.A,
				},
				Op:    o.O,
				Value: o.V,
			}
		}

		if err := op.validate(); err != nil {
			panic(err)
		}

		res = append(res, op)
	}

	return res
}
