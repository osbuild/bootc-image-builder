package osbuild

import (
	"crypto/sha256"
	"fmt"
)

type FDOStageInputs struct {
	RootCerts *FilesInput `json:"rootcerts"`
}

func (FDOStageInputs) isStageInputs() {}

// NewFDOStageForCert creates FDOStage
func NewFDOStageForRootCerts(rootCertsData string) *Stage {
	dataBytes := []byte(rootCertsData)
	input := NewFilesInput(NewFilesInputSourcePlainRef([]string{
		fmt.Sprintf("sha256:%x", sha256.Sum256(dataBytes)),
	}))

	return &Stage{
		Type:   "org.osbuild.fdo",
		Inputs: &FDOStageInputs{RootCerts: input},
	}
}
