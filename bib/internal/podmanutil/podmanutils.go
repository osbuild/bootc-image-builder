package podmanutil

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// envPath is written by podman
const envPath = "/run/.containerenv"

// rootlessKey is set when we are rootless
const rootlessKey = "rootless=1"

// IsRootless detects if we are running rootless in podman;
// other situations (e.g. docker) will successfuly return false.
func IsRootless() (bool, error) {
	buf, err := os.ReadFile(envPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		if scanner.Text() == rootlessKey {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("parsing %s: %w", envPath, err)
	}
	return false, nil
}
