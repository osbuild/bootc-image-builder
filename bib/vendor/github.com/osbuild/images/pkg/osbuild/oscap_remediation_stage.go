package osbuild

import "fmt"

type OscapVerbosityLevel string

const (
	OscapVerbosityLevelDevel   = "DEVEL"
	OscapVerbosityLevelInfo    = "INFO"
	OscapVerbosityLevelError   = "ERROR"
	OscapVerbosityLevelWarning = "WARNING"
)

type OscapRemediationStageOptions struct {
	DataDir string      `json:"data_dir,omitempty"`
	Config  OscapConfig `json:"config"`
}

type OscapConfig struct {
	Datastream   string              `json:"datastream" toml:"datastream"`
	ProfileID    string              `json:"profile_id" toml:"profile_id"`
	DatastreamID string              `json:"datastream_id,omitempty" toml:"datastream_id,omitempty"`
	XCCDFID      string              `json:"xccdf_id,omitempty" toml:"xccdf_id,omitempty"`
	BenchmarkID  string              `json:"benchmark_id,omitempty" toml:"benchmark_id,omitempty"`
	Tailoring    string              `json:"tailoring,omitempty" toml:"tailoring,omitempty"`
	TailoringID  string              `json:"tailoring_id,omitempty" toml:"tailoring_id,omitempty"`
	ArfResult    string              `json:"arf_results,omitempty" toml:"arf_results,omitempty"`
	HtmlReport   string              `json:"html_report,omitempty" toml:"html_report,omitempty"`
	VerboseLog   string              `json:"verbose_log,omitempty" toml:"verbose_log,omitempty"`
	VerboseLevel OscapVerbosityLevel `json:"verbose_level,omitempty" toml:"verbose_level,omitempty"`
	Compression  bool                `json:"compress_results,omitempty" toml:"compress_results,omitempty"`
}

func (OscapRemediationStageOptions) isStageOptions() {}

func (c OscapConfig) validate() error {
	if c.Datastream == "" {
		return fmt.Errorf("'datastream' must be specified")
	}
	if c.ProfileID == "" {
		return fmt.Errorf("'profile_id' must be specified")
	}
	if c.VerboseLevel != "" {
		allowedVerboseLevelValues := []OscapVerbosityLevel{
			OscapVerbosityLevelDevel,
			OscapVerbosityLevelError,
			OscapVerbosityLevelInfo,
			OscapVerbosityLevelWarning,
		}
		valid := false
		for _, value := range allowedVerboseLevelValues {
			if c.VerboseLevel == value {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("'verbose_level' option does not allow %q as a value", c.VerboseLevel)
		}
	}
	return nil
}

func NewOscapRemediationStage(options *OscapRemediationStageOptions) *Stage {
	if err := options.Config.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.oscap.remediation",
		Options: options,
	}
}

func NewOscapRemediationStageOptions(dataDir string, options OscapConfig) *OscapRemediationStageOptions {
	return &OscapRemediationStageOptions{
		DataDir: dataDir,
		Config: OscapConfig{
			ProfileID:    options.ProfileID,
			Datastream:   options.Datastream,
			DatastreamID: options.DatastreamID,
			Tailoring:    options.Tailoring,
			XCCDFID:      options.XCCDFID,
			BenchmarkID:  options.BenchmarkID,
			ArfResult:    options.ArfResult,
			HtmlReport:   options.HtmlReport,
			VerboseLog:   options.VerboseLog,
			VerboseLevel: options.VerboseLevel,
			Compression:  options.Compression,
		},
	}
}
