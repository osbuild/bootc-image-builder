package disk

import (
	"fmt"
	"math/rand"

	"github.com/google/uuid"

	"github.com/osbuild/images/internal/common"
)

type Argon2id struct {
	Iterations  uint
	Memory      uint
	Parallelism uint
}

type ClevisBind struct {
	Pin              string
	Policy           string
	RemovePassphrase bool
}
type LUKSContainer struct {
	Passphrase string
	UUID       string
	Cipher     string
	Label      string
	Subsystem  string
	SectorSize uint64

	// password-based key derivation function
	PBKDF Argon2id

	Clevis *ClevisBind

	Payload Entity
}

func (lc *LUKSContainer) IsContainer() bool {
	return true
}

func (lc *LUKSContainer) GetItemCount() uint {
	if lc.Payload == nil {
		return 0
	}
	return 1
}

func (lc *LUKSContainer) GetChild(n uint) Entity {
	if n != 0 {
		panic(fmt.Sprintf("invalid child index for LUKSContainer: %d != 0", n))
	}
	return lc.Payload
}

func (lc *LUKSContainer) Clone() Entity {
	if lc == nil {
		return nil
	}
	clc := &LUKSContainer{
		Passphrase: lc.Passphrase,
		UUID:       lc.UUID,
		Cipher:     lc.Cipher,
		Label:      lc.Label,
		Subsystem:  lc.Subsystem,
		SectorSize: lc.SectorSize,
		PBKDF: Argon2id{
			Iterations:  lc.PBKDF.Iterations,
			Memory:      lc.PBKDF.Memory,
			Parallelism: lc.PBKDF.Parallelism,
		},
		Payload: lc.Payload.Clone(),
	}
	if lc.Clevis != nil {
		clc.Clevis = &ClevisBind{
			Pin:              lc.Clevis.Pin,
			Policy:           lc.Clevis.Policy,
			RemovePassphrase: lc.Clevis.RemovePassphrase,
		}
	}
	return clc
}

func (lc *LUKSContainer) GenUUID(rng *rand.Rand) {
	if lc == nil {
		return
	}

	if lc.UUID == "" {
		lc.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}
}

func (lc *LUKSContainer) MetadataSize() uint64 {
	if lc == nil {
		return 0
	}

	// 16 MiB is the default size for the LUKS2 header
	return 16 * common.MiB
}
