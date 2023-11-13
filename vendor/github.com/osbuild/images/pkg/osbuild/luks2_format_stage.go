package osbuild

import "fmt"

// Create LUKS2 container

type LUKS2CreateStageOptions struct {
	Passphrase string `json:"passphrase"`
	UUID       string `json:"uuid"`
	Cipher     string `json:"cipher,omitempty"`
	Label      string `json:"label,omitempty"`
	Subsystem  string `json:"subsystem,omitempty"`
	SectorSize uint64 `json:"sector-size"`

	// password-based key derivation function
	PBKDF Argon2id `json:"pbkdf"`
}

type Argon2id struct {
	// Method must be Argin2id
	Method      string `json:"method"`
	Iterations  uint   `json:"iterations"`
	Memory      uint   `json:"memory,omitempty"`
	Parallelism uint   `json:"parallelism,omitempty"`
}

func (LUKS2CreateStageOptions) isStageOptions() {}

func (o LUKS2CreateStageOptions) validate() error {
	if o.PBKDF.Method != "argon2i" && o.PBKDF.Method != "argon2id" {
		return fmt.Errorf("PBKDF method should be argon2i or argon2id")
	}
	if o.PBKDF.Memory < 32 || o.PBKDF.Memory > 4194304 {
		return fmt.Errorf("PBKDF memory should be between 32 and 4194304")
	}
	if o.PBKDF.Iterations < 4 || o.PBKDF.Iterations > 4294967295 {
		return fmt.Errorf("PBKDF iterations should be between 4 and 4294967295")
	}
	if o.PBKDF.Parallelism < 1 || o.PBKDF.Parallelism > 4 {
		return fmt.Errorf("PBKDF parallelism should be between 1 and 4")
	}
	return nil
}

func NewLUKS2CreateStage(options *LUKS2CreateStageOptions, devices Devices) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.luks2.format",
		Options: options,
		Devices: devices,
	}
}
