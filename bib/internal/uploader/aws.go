package uploader

import (
	"fmt"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/cloud/awscloud"
)

func UploadAndRegister(a *awscloud.AWS, filename, bucketName, imageName, targetArch string) error {
	keyName := fmt.Sprintf("%s-%s", uuid.New().String(), filepath.Base(filename))

	fmt.Printf("Uploading %s to %s:%s\n", filename, bucketName, keyName)
	uploadOutput, err := a.Upload(filename, bucketName, keyName)
	if err != nil {
		return err
	}
	fmt.Printf("File uploaded to %s\n", aws.StringValue(&uploadOutput.Location))

	if targetArch == "" {
		targetArch = arch.Current().String()
	}
	bootMode := ec2.BootModeValuesUefiPreferred
	fmt.Printf("Registering AMI %s\n", imageName)
	ami, snapshot, err := a.Register(imageName, bucketName, keyName, nil, targetArch, &bootMode)
	fmt.Printf("Deleted S3 object %s:%s\n", bucketName, keyName)
	fmt.Printf("AMI registered: %s\nSnapshot ID: %s\n", aws.StringValue(ami), aws.StringValue(snapshot))
	if err != nil {
		return err
	}
	return nil
}
