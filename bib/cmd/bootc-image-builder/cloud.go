package main

import (
	"github.com/spf13/pflag"

	"github.com/osbuild/images/pkg/cloud/awscloud"

	"github.com/osbuild/bootc-image-builder/bib/internal/progress"
	"github.com/osbuild/bootc-image-builder/bib/internal/uploader"
)

func uploadAMI(pbar progress.ProgressBar, path, targetArch string, flags *pflag.FlagSet) error {
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

	client, err := awscloud.NewDefault(region)
	if err != nil {
		return err
	}

	return uploader.UploadAndRegister(client, path, bucketName, imageName, targetArch, pbar)
}
