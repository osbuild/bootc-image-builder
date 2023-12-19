package main

import (
	"github.com/osbuild/bootc-image-builder/bib/internal/uploader"
	"github.com/spf13/pflag"
)

func uploadAMI(path string, flags *pflag.FlagSet) error {
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
	awsProfile, err := flags.GetString("aws-profile")
	if err != nil {
		return err
	}

	client, err := uploader.NewAWSClient(region, awsProfile)
	if err != nil {
		return err
	}
	return uploader.UploadAndRegister(client, path, bucketName, imageName)
}
