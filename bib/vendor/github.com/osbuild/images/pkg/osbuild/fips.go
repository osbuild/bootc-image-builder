package osbuild

import (
	"os"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/disk"
)

var (
	FIPSDracutConfStageOptions = &DracutConfStageOptions{
		Filename: "40-fips.conf",
		Config: DracutConfigFile{
			AddModules: []string{"fips"},
		},
	}
)

func GenFIPSKernelOptions(pt *disk.PartitionTable) []string {
	cmdline := make([]string, 0)
	cmdline = append(cmdline, "fips=1")
	if bootMnt := pt.FindMountable("/boot"); bootMnt != nil {
		boot := bootMnt.GetFSSpec()
		if label := boot.Label; label != "" {
			karg := "boot=LABEL=" + label
			cmdline = append(cmdline, karg)
		} else if uuid := boot.UUID; uuid != "" {
			karg := "boot=UUID=" + uuid
			cmdline = append(cmdline, karg)
		}
	}
	return cmdline
}

func GenFIPSFiles() (files []*fsnode.File) {
	file, _ := fsnode.NewFile("/etc/system-fips", common.ToPtr(os.FileMode(0644)),
		"root", "root", []byte("# FIPS module installation complete\n"))
	files = append(files, file)
	return
}

func GenFIPSStages() (stages []*Stage) {
	stages = []*Stage{
		NewUpdateCryptoPoliciesStage(
			&UpdateCryptoPoliciesStageOptions{
				Policy: "FIPS",
			}),
		NewDracutConfStage(FIPSDracutConfStageOptions),
	}
	stages = append(stages, GenFileNodesStages(GenFIPSFiles())...)
	return
}
