// This is the primary entrypoint for /usr/bin/coreos-assembler.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/cgwalters/osbuildbootc/cmd/qemuexec"
)

var (
	rootCmd = &cobra.Command{Use: "app"}

	cmdQcow2 = &cobra.Command{
		Use:   "qcow2 [local container] [target imgref] [disk]",
		Short: "Generate a qcow2 from a bootc image",
		Args:  cobra.MatchAll(cobra.ExactArgs(3), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			target := args[1]
			dest := args[2]

			c := exec.Command("/usr/lib/osbuildbootc/qcow2.sh", src, target, dest)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	cmdVMSHell = &cobra.Command{
		Use:   "vmshell",
		Short: "Run a shell in the build VM",
		Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("/usr/lib/osbuildbootc/vmshell.sh")
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return err
			}
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(cmdQcow2)
	rootCmd.AddCommand(cmdVMSHell)
	rootCmd.AddCommand(qemuexec.CmdQemuExec)
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// In this case the command we ran gave a non-zero exit
			// code. Let's also exit with that exit code.
			os.Exit(exitErr.ExitCode())
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}
