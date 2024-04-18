package util

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// IsMountpoint checks if the target path is a mount point
func IsMountpoint(path string) bool {
	return exec.Command("mountpoint", path).Run() == nil
}

// Synchronously invoke a command, propagating stdout and stderr
// to the current process's stdout and stderr
func RunCmdSync(cmdName string, args ...string) error {
	logrus.Debugf("Running: %s %s", cmdName, strings.Join(args, " "))
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running %s %s: %w", cmdName, strings.Join(args, " "), err)
	}
	return nil
}

// OutputErr takes an error from exec.Command().Output() and tries
// generate an error with stderr details
func OutputErr(err error) error {
	if err, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("%w, stderr:\n%s", err, err.Stderr)
	}
	return err
}
