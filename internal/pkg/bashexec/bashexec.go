// Package bashexec provides helpers to execute bash code.
// What this primarily offers over directly writing e.g. `exec.Command("bash")`
// is:
//
// - By default, all fragments are executed in "bash strict mode": http://redsymbol.net/articles/unofficial-bash-strict-mode/
// - The code encourages adding a "name" for in-memory scripts, similar to e.g.
//   Ansible tasks as well as many CI systems like Github actions
// - The code to execute is piped to stdin instead of passed via `-c` which
//   avoids argument length limits and makes the output of e.g. `ps` readable.
// - Scripts are assumed synchronous, and stdin/stdout/stderr are passed directly
//   instead of piped.
// - We use prctl(PR_SET_PDEATHSIG) (assuming Linux) to lifecycle bind the script to the caller
//
package bashexec

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// StrictMode enables http://redsymbol.net/articles/unofficial-bash-strict-mode/
const StrictMode = "set -euo pipefail"

// BashRunner is a wrapper for executing in-memory bash scripts
type BashRunner struct {
	name string
	cmd  *exec.Cmd
}

// NewBashRunner creates a bash executor from in-memory shell script.
func NewBashRunner(name, src string, args ...string) (*BashRunner, error) {
	// This will be proxied to fd 3
	f, err := os.CreateTemp("", name)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(f, strings.NewReader(src)); err != nil {
		return nil, err
	}
	if err := os.Remove(f.Name()); err != nil {
		return nil, err
	}

	bashCmd := fmt.Sprintf("%s\n. /proc/self/fd/3\n", StrictMode)
	fullargs := append([]string{"-c", bashCmd, name}, args...)
	cmd := exec.Command("/bin/bash", fullargs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
	cmd.Stdin = os.Stdin
	cmd.ExtraFiles = append(cmd.ExtraFiles, f)

	return &BashRunner{
		name: name,
		cmd:  cmd,
	}, nil
}

// Exec synchronously spawns the child process, passing stdin/stdout/stderr directly.
func (r *BashRunner) Exec() error {
	r.cmd.Stdin = os.Stdin
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr
	err := r.cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute internal script %s: %w", r.name, err)
	}
	return nil
}

// Run spawns the script, gathering stdout/stderr into a buffer that is displayed only on error.
func (r *BashRunner) Run() error {
	buf, err := r.cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute internal script %s: %w\n%s", r.name, err, buf)
	}
	return nil
}

// Run spawns a named script (without any arguments),
// gathering stdout/stderr into a buffer that is displayed only on error.
func Run(name, cmd string) error {
	sh, err := NewBashRunner(name, cmd)
	if err != nil {
		return err
	}
	return sh.Run()
}

// RunA spawns an anonymous script, and is otherwise the same as `Run`.
func RunA(cmd string) error {
	sh, err := NewBashRunner("", cmd)
	if err != nil {
		return err
	}
	return sh.Run()
}
