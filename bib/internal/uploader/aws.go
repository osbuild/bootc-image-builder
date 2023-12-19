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

func checkAWSCredentialFileExists() (bool, error) {
	// SMELL make this path configurable?
	credentialFile := "$HOME/.aws/credentials"

	expandedFilename := os.ExpandEnv(credentialFile)
	_, err := os.Stat(expandedFilename)
	if err != nil {
		return false, fmt.Errorf("AWS credential file not found: %s", expandedFilename)
	}
	return true, nil
}

// NewAWSClient returns an awscloud.AWS configured with a session for a given
// region. It reads the credentials from environment variables:
// AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY.
// or tries to use "$HOME/.aws/credentials" otherwise
func NewAWSClient(region string, awsProfile string) (*awscloud.AWS, error) {
	foundCredentialFile := false

	keyID := os.Getenv("AWS_ACCESS_KEY_ID")
	if keyID == "" {
		var err error
		foundCredentialFile, err = checkAWSCredentialFileExists()

		if err != nil {
			return nil, fmt.Errorf("AWS_ACCESS_KEY_ID environment variable or a valid '$HOME/.aws/credentials' is required")
		}
	}

	if foundCredentialFile {
		// SMELL we might want to implement a separate function
		//  like awscloud.newFromLocalProfile(awsProfile string, region string)
		//  then we can remove this Setenv!
		if  awsProfile != "default" {
			os.Setenv("AWS_PROFILE", awsProfile)
		}

		return awscloud.NewDefault(region)
	} else {
		secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		if secretKey == "" {
			return nil, fmt.Errorf("AWS_SECRET_ACCESS_KEY environment variable is required, when AWS_ACCESS_KEY_ID is specified")
		}
		return awscloud.New(region, keyID, secretKey, "")
	}
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
