// This is the primary entrypoint for /usr/bin/coreos-assembler.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/cgwalters/osbuildbootc/cmd/builddiskimpl"
	"github.com/cgwalters/osbuildbootc/cmd/qemuexec"
	"github.com/cgwalters/osbuildbootc/internal/pkg/cmdutil"
)

var (
	rootCmd = &cobra.Command{Use: "app"}

	sourceTransport string
	targetImage     string
	targetInsecure  bool
	skipFetchCheck  bool
	sizeMiB         uint64
	cmdQcow2        = &cobra.Command{
		Use:   "qcow2 [source container] [disk]",
		Short: "Generate a qcow2 from a bootc image",
		Args:  cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			dest := args[1]

			if err := os.MkdirAll("tmp", 0755); err != nil {
				return err
			}

			if targetImage == "" {
				targetImage = args[0]
			}

			installArgs := []string{}
			if targetInsecure {
				installArgs = append(installArgs, "--target-no-signature-verification")
			}
			if skipFetchCheck {
				installArgs = append(installArgs, "--skip-fetch-check")
			}
			installArgs = append(installArgs, "--target-imgref="+targetImage)

			config := builddiskimpl.Config{
				SourceTransport: sourceTransport,
				Source:          source,
				InstallArgs:     installArgs,
				Disk:            "/dev/disk/by-id/virtio-target",
			}
			buf, err := json.Marshal(&config)
			if err != nil {
				return err
			}
			configPath := "tmp/config.json"
			if err := os.WriteFile(configPath, buf, 0644); err != nil {
				return err
			}

			if err := cmdutil.RunCmdSync("qemu-img", "create", "-f", "qcow2", dest, fmt.Sprintf("%dM", sizeMiB)); err != nil {
				return err
			}

			klog.Infof("Generating image; source=%s target=%s", config.Source, targetImage)
			c := exec.Command("/usr/lib/osbuildbootc/qcow2.sh", dest, configPath)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				_ = os.Remove(dest)
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
	cmdQcow2.Flags().StringVar(&sourceTransport, "transport", "docker://", "Source image stransport")
	cmdQcow2.Flags().Uint64VarP(&sizeMiB, "size", "", 10*1024, "Disk size in MiB")
	cmdQcow2.Flags().StringVarP(&targetImage, "target", "t", "", "Target image (e.g. quay.io/exampleuser/someimg:latest)")
	cmdQcow2.Flags().BoolVarP(&targetInsecure, "target-no-signature-verification", "I", false, "Disable signature verification for target")
	cmdQcow2.Flags().BoolVarP(&skipFetchCheck, "skip-fetch-check", "S", false, "Skip verification of target image")
	rootCmd.AddCommand(cmdVMSHell)
	rootCmd.AddCommand(qemuexec.CmdQemuExec)
	rootCmd.AddCommand(builddiskimpl.CmdBuildDiskImpl)
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
