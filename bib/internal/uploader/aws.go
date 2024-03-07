package uploader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cheggaaa/pb/v3"
	"github.com/google/uuid"

	"github.com/osbuild/images/pkg/arch"
)

var osStdout io.Writer = os.Stdout

type AwsUploader interface {
	UploadFromReader(r io.Reader, bucketName, keyName string) (*s3manager.UploadOutput, error)
	Register(name, bucket, key string, shareWith []string, rpmArch string, bootMode *string) (*string, *string, error)
}

func doUpload(a AwsUploader, file *os.File, bucketName, keyName string, pbar *pb.ProgressBar) (*s3manager.UploadOutput, error) {
	var r io.Reader = file

	// TODO: extract this as a helper once we add "uploadAzure" or
	// similar.
	if pbar != nil {
		st, err := file.Stat()
		if err != nil {
			return nil, fmt.Errorf("cannot stat upload: %v", err)
		}
		pbar.SetTotal(st.Size())
		pbar.Set(pb.Bytes, true)
		pbar.SetWriter(osStdout)
		r = pbar.NewProxyReader(file)
		pbar.Start()
		defer pbar.Finish()
	}

	return a.UploadFromReader(r, bucketName, keyName)
}

func UploadAndRegister(a AwsUploader, filename, bucketName, imageName, targetArch string, pbar *pb.ProgressBar) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot upload: %v", err)
	}
	defer file.Close()

	keyName := fmt.Sprintf("%s-%s", uuid.New().String(), filepath.Base(filename))
	fmt.Fprintf(osStdout, "Uploading %s to %s:%s\n", filename, bucketName, keyName)
	uploadOutput, err := doUpload(a, file, bucketName, keyName, pbar)
	if err != nil {
		return err
	}
	fmt.Fprintf(osStdout, "File uploaded to %s\n", aws.StringValue(&uploadOutput.Location))

	if targetArch == "" {
		targetArch = arch.Current().String()
	}
	bootMode := ec2.BootModeValuesUefiPreferred
	fmt.Fprintf(osStdout, "Registering AMI %s\n", imageName)
	ami, snapshot, err := a.Register(imageName, bucketName, keyName, nil, targetArch, &bootMode)
	fmt.Fprintf(osStdout, "Deleted S3 object %s:%s\n", bucketName, keyName)
	fmt.Fprintf(osStdout, "AMI registered: %s\nSnapshot ID: %s\n", aws.StringValue(ami), aws.StringValue(snapshot))
	if err != nil {
		return err
	}
	return nil
}
