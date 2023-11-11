package builddiskimpl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/cgwalters/osbuildbootc/internal/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type Config struct {
	SourceTransport string   `json:"source-transport"`
	Source          string   `json:"source"`
	InstallArgs     []string `json:"installargs"`
	Disk            string   `json:"disk"`
}

var (
	CmdBuildDiskImpl = &cobra.Command{
		Use:    "build-disk-impl",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE:   run,
	}
)

func concatTransport(transport, image string) string {
	if transport == "docker://" {
		return transport + image
	}
	if !strings.HasSuffix(transport, ":") {
		return transport + ":" + image
	}
	return transport + image
}

func run(cmd *cobra.Command, args []string) error {
	configPath := args[0]
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		return err
	}
	source := concatTransport(config.SourceTransport, config.Source)
	tmp := "localhost/image"
	if err := cmdutil.RunCmdSync("skopeo", "copy", source, "containers-storage:"+tmp); err != nil {
		return err
	}

	podmanArgs := []string{"run", "--net=host", "--rm", "--privileged", "--pid=host", "--security-opt", "label=type:unconfined_t"}
	podmanArgs = append(podmanArgs, tmp)
	podmanArgs = append(podmanArgs, "bootc", "install")
	podmanArgs = append(podmanArgs, config.InstallArgs...)
	disk, err := filepath.EvalSymlinks(config.Disk)
	if err != nil {
		return err
	}
	podmanArgs = append(podmanArgs, disk)
	if err := cmdutil.RunCmdSync("podman", podmanArgs...); err != nil {
		return err
	}
	return nil
}
