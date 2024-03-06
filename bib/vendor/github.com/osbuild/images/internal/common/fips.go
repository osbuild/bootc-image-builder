package common

import (
	"bufio"
	"os"
	"strings"
)

const (
	FIPSEnabledImageWarning = `The host building this image is not ` +
		`running in FIPS mode. The image will still be FIPS compliant. ` +
		`If you have custom steps that generate keys or perform ` +
		`cryptographic operations, those must be considered non-compliant.`
)

var (
	FIPSEnabledFilePath = "/proc/sys/crypto/fips_enabled"
)

func IsBuildHostFIPSEnabled() (enabled bool) {
	file, err := os.Open(FIPSEnabledFilePath)
	if err != nil {
		return
	}
	defer file.Close()
	buf := []byte{}
	_, err = file.Read(buf)
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return
	}
	return strings.TrimSpace(scanner.Text()) == "1"
}
