package uploader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/cloud/awscloud"
)

// NewAWSClient returns an awscloud.AWS configured with a session for a given
// region. It reads the credentials from environment variables:
// AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY.
func NewAWSClient(region string) (*awscloud.AWS, error) {
	keyID := os.Getenv("AWS_ACCESS_KEY_ID")
	if keyID == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID environment variable is required")
	}
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if secretKey == "" {
		return nil, fmt.Errorf("AWS_SECRET_ACCESS_KEY environment variable is required")
	}
	return awscloud.New(region, keyID, secretKey, "")
}

func UploadAndRegister(a *awscloud.AWS, filename, bucketName, imageName string) error {
	keyName := fmt.Sprintf("%s-%s", uuid.New().String(), filepath.Base(filename))

	fmt.Printf("Uploading %s to %s:%s\n", filename, bucketName, keyName)
	uploadOutput, err := a.Upload(filename, bucketName, keyName)
	if err != nil {
		return err
	}
	fmt.Printf("File uploaded to %s\n", aws.StringValue(&uploadOutput.Location))

	hostArch := arch.Current()
	bootMode := ec2.BootModeValuesUefiPreferred
	fmt.Printf("Registering AMI %s\n", imageName)
	ami, snapshot, err := a.Register(imageName, bucketName, keyName, nil, hostArch.String(), &bootMode)
	fmt.Printf("Deleted S3 object %s:%s\n", bucketName, keyName)
	fmt.Printf("AMI registered: %s\nSnapshot ID: %s\n", aws.StringValue(ami), aws.StringValue(snapshot))
	if err != nil {
		return err
	}
	return nil
}
