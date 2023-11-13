package osbuild

import (
	"fmt"
	"regexp"
)

const configFilenameRegex = "^[a-zA-Z0-9_\\.-]{1,250}\\.conf$"

type SystemdJournaldStageOptions struct {
	Filename string                      `json:"filename"`
	Config   SystemdJournaldConfigDropin `json:"config"`
}

func (SystemdJournaldStageOptions) isStageOptions() {}

func (o SystemdJournaldStageOptions) validate() error {
	filenameRegex := regexp.MustCompile(configFilenameRegex)
	if !filenameRegex.MatchString(o.Filename) {
		return fmt.Errorf("filename %q doesn't conform to schema (%s)", o.Filename, repoFilenameRegex)
	}
	if o.Config.Journal == (SystemdJournaldConfigJournalSection{}) {
		return fmt.Errorf("the 'Journal' section is required")
	}

	return nil
}

func NewSystemdJournaldStage(options *SystemdJournaldStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    "org.osbuild.systemd-journald",
		Options: options,
	}
}

type SystemdJournaldConfigDropin struct {
	Journal SystemdJournaldConfigJournalSection `json:"Journal"`
}

type ConfigStorage string

const (
	StorageVolatile   ConfigStorage = "volatile"
	StoragePresistent ConfigStorage = "persistent"
	StorageAuto       ConfigStorage = "auto"
	StorageNone       ConfigStorage = "none"
)

type ConfigSplitMode string

const (
	SplitUuid ConfigSplitMode = "uuid"
	SplitNone ConfigSplitMode = "none"
)

type ConfigAudit string

const (
	AuditYes ConfigAudit = "yes"
	AuditNo  ConfigAudit = "no"
)

// 'Journal' configuration section, at least one option must be specified
type SystemdJournaldConfigJournalSection struct {
	// Controls where to store journal data.
	Storage ConfigStorage `json:"Storage,omitempty"`

	// Sets whether the data objects stored in the journal should be
	// compressed or not. Can also take threshold values.
	Compress string `json:"Compress,omitempty"`

	// Splits journal files per user or to a single file.
	SplitMode ConfigSplitMode `json:"SplitMode,omitempty"`

	// Max time to store entries in a single file. By default seconds, may be
	// sufixed with units (year, month, week, day, h, m) to override this.
	MaxFileSec string `json:"MaxFileSec,omitempty"`

	// Maximum time to store journal entries. By default seconds, may be sufixed
	// with units (year, month, week, day, h, m) to override this.
	MaxRetentionSec string `json:"MaxRetentionSec,omitempty"`

	// Timeout before synchronizing journal files to disk. Minimum 0.
	SyncIntervalSec int `json:"SyncIntervalSec,omitempty"`

	// Enables/Disables kernel auditing on start-up, leaves it as is if
	// unspecified.
	Audit ConfigAudit `json:"Audit,omitempty"`
}
