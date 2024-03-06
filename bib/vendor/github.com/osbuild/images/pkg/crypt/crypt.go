package crypt

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// CryptSHA512 encrypts the given password with SHA512 and a random salt.
//
// Note that this function is not deterministic.
func CryptSHA512(phrase string) (string, error) {
	const SHA512SaltLength = 16

	salt, err := genSalt(SHA512SaltLength)

	if err != nil {
		return "", nil
	}

	hashSettings := "$6$" + salt
	return crypt(phrase, hashSettings)
}

func genSalt(length int) (string, error) {
	saltChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789./"

	b := make([]byte, length)

	for i := range b {
		runeIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(saltChars))))
		if err != nil {
			return "", err
		}
		b[i] = saltChars[runeIndex.Int64()]
	}

	return string(b), nil
}

// PasswordIsCrypted returns true if the password appears to be an encrypted
// one, according to a very simple heuristic.
//
// Any string starting with one of $2$, $6$ or $5$ is considered to be
// encrypted. Any other string is consdirede to be unencrypted.
//
// This functionality is taken from pylorax.
func PasswordIsCrypted(s string) bool {
	// taken from lorax src: src/pylorax/api/compose.py:533
	prefixes := [...]string{"$2b$", "$6$", "$5$"}

	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}

	return false
}
