package distro

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// GetHostDistroName returns the name of the host distribution, such as
// "fedora-32" or "rhel-8.2". It does so by reading the /etc/os-release file.
func GetHostDistroName() (string, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", err
	}
	defer f.Close()
	osrelease, err := readOSRelease(f)
	if err != nil {
		return "", err
	}

	name := osrelease["ID"] + "-" + osrelease["VERSION_ID"]

	return name, nil
}

func readOSRelease(r io.Reader) (map[string]string, error) {
	osrelease := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("readOSRelease: invalid input")
		}

		key := strings.TrimSpace(parts[0])
		// drop all surrounding whitespace and double-quotes
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		osrelease[key] = value
	}

	return osrelease, nil
}
