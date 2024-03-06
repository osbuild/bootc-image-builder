package distro

import (
	"fmt"
	"reflect"

	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/shell"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/subscription"
)

// ImageConfig represents a (default) configuration applied to the image
type ImageConfig struct {
	Timezone            *string
	TimeSynchronization *osbuild.ChronyStageOptions
	Locale              *string
	Keyboard            *osbuild.KeymapStageOptions
	EnabledServices     []string
	DisabledServices    []string
	DefaultTarget       *string
	Sysconfig           []*osbuild.SysconfigStageOptions

	// List of files from which to import GPG keys into the RPM database
	GPGKeyFiles []string

	// Disable SELinux labelling
	NoSElinux *bool

	// Do not use. Forces auto-relabelling on first boot.
	// See https://github.com/osbuild/osbuild/commit/52cb27631b587c1df177cd17625c5b473e1e85d2
	SELinuxForceRelabel *bool

	// Disable documentation
	ExcludeDocs *bool

	ShellInit []shell.InitFile

	// for RHSM configuration, we need to potentially distinguish the case
	// when the user want the image to be subscribed on first boot and when not
	RHSMConfig          map[subscription.RHSMStatus]*osbuild.RHSMStageOptions
	SystemdLogind       []*osbuild.SystemdLogindStageOptions
	CloudInit           []*osbuild.CloudInitStageOptions
	Modprobe            []*osbuild.ModprobeStageOptions
	DracutConf          []*osbuild.DracutConfStageOptions
	SystemdUnit         []*osbuild.SystemdUnitStageOptions
	Authselect          *osbuild.AuthselectStageOptions
	SELinuxConfig       *osbuild.SELinuxConfigStageOptions
	Tuned               *osbuild.TunedStageOptions
	Tmpfilesd           []*osbuild.TmpfilesdStageOptions
	PamLimitsConf       []*osbuild.PamLimitsConfStageOptions
	Sysctld             []*osbuild.SysctldStageOptions
	DNFConfig           []*osbuild.DNFConfigStageOptions
	SshdConfig          *osbuild.SshdConfigStageOptions
	Authconfig          *osbuild.AuthconfigStageOptions
	PwQuality           *osbuild.PwqualityConfStageOptions
	WAAgentConfig       *osbuild.WAAgentConfStageOptions
	Grub2Config         *osbuild.GRUB2Config
	DNFAutomaticConfig  *osbuild.DNFAutomaticConfigStageOptions
	YumConfig           *osbuild.YumConfigStageOptions
	YUMRepos            []*osbuild.YumReposStageOptions
	Firewall            *osbuild.FirewallStageOptions
	UdevRules           *osbuild.UdevRulesStageOptions
	GCPGuestAgentConfig *osbuild.GcpGuestAgentConfigOptions
	WSLConfig           *osbuild.WSLConfStageOptions

	Files       []*fsnode.File
	Directories []*fsnode.Directory
}

// InheritFrom inherits unset values from the provided parent configuration and
// returns a new structure instance, which is a result of the inheritance.
func (c *ImageConfig) InheritFrom(parentConfig *ImageConfig) *ImageConfig {
	finalConfig := ImageConfig(*c)
	if parentConfig != nil {
		// iterate over all struct fields and copy unset values from the parent
		for i := 0; i < reflect.TypeOf(*c).NumField(); i++ {
			fieldName := reflect.TypeOf(*c).Field(i).Name
			field := reflect.ValueOf(&finalConfig).Elem().FieldByName(fieldName)

			// Only container types or pointer are supported.
			// The reason is that with basic types, we can't distinguish between unset value and zero value.
			if kind := field.Kind(); kind != reflect.Ptr && kind != reflect.Slice && kind != reflect.Map {
				panic(fmt.Sprintf("unsupported field type: %s (only container types or pointer are supported)",
					field.Kind()))
			}

			if field.IsNil() {
				field.Set(reflect.ValueOf(parentConfig).Elem().FieldByName(fieldName))
			}
		}
	}
	return &finalConfig
}
