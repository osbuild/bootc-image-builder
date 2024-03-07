package main

import (
	"github.com/cheggaaa/pb/v3"
	"github.com/osbuild/bootc-image-builder/bib/internal/uploader"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/spf13/pflag"
)

func uploadAMI(path, targetArch string, flags *pflag.FlagSet) error {
	region, err := flags.GetString("aws-region")
	if err != nil {
		return err
	}
	bucketName, err := flags.GetString("aws-bucket")
	if err != nil {
		return err
	}
	imageName, err := flags.GetString("aws-ami-name")
	if err != nil {
		return err
	}
	progress, err := flags.GetString("progress")
	if err != nil {
		return err
	}

	client, err := awscloud.NewDefault(region)
	if err != nil {
		return err
	}

	// TODO: extract this as a helper once we add "uploadAzure" or
	// similar. Eventually we may provide json progress here too.
	var pbar *pb.ProgressBar
	switch progress {
	case "text":
		pbar = pb.New(0)
	}

	return uploader.UploadAndRegister(client, path, bucketName, imageName, targetArch, pbar)
}
