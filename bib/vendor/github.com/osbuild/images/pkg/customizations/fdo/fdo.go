package fdo

import "github.com/osbuild/images/pkg/blueprint"

type Options struct {
	ManufacturingServerURL  string
	DiunPubKeyInsecure      string
	DiunPubKeyHash          string
	DiunPubKeyRootCerts     string
	DiMfgStringTypeMacIface string
}

func FromBP(bpFDO blueprint.FDOCustomization) *Options {
	fdo := Options(bpFDO)
	return &fdo
}
