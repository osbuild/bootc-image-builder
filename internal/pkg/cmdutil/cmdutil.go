package cmdutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Synchronously invoke a command, writing its stdout to our stdout,
// and gathering stderr into a buffer which will be returned in err
// in case of error.
func RunCmdSync(cmdName string, args ...string) error {
	fmt.Printf("Running: %s %s\n", cmdName, strings.Join(args, " "))
	cmd := exec.Command(cmdName, args...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running %s %s: %s: %w", cmdName, strings.Join(args, " "), stderr.String(), err)
	}

	return nil
}
