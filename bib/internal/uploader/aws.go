package uploader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/uuid"

	"github.com/osbuild/images/pkg/arch"

	"github.com/osbuild/bootc-image-builder/bib/internal/progress"
)

var osStdout io.Writer = os.Stdout

type AwsUploader interface {
	UploadFromReader(r io.Reader, bucketName, keyName string) (*s3manager.UploadOutput, error)
	Register(name, bucket, key string, shareWith []string, rpmArch string, bootMode, importRole *string) (*string, *string, error)
}

type proxyReader struct {
	subLevel    int
	r           io.Reader
	pbar        progress.ProgressBar
	done, total int64
}

func (r *proxyReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.done += int64(n)
	r.pbar.SetProgress(r.subLevel, "Uploading", r.done, r.total)
	return n, err
}

func doUpload(a AwsUploader, file *os.File, bucketName, keyName string, pbar progress.ProgressBar) (*s3manager.UploadOutput, error) {
	var r io.Reader = file

	// TODO: extract this as a helper once we add "uploadAzure" or
	// similar.
	if pbar != nil {
		st, err := file.Stat()
		if err != nil {
			return nil, fmt.Errorf("cannot stat upload: %v", err)
		}
		pbar.SetMessagef("Uploading %s to %s", file.Name(), bucketName)
		r = &proxyReader{0, file, pbar, 0, st.Size()}
	}

	return a.UploadFromReader(r, bucketName, keyName)
}

func UploadAndRegister(a AwsUploader, filename, bucketName, imageName, targetArch string, pbar progress.ProgressBar) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot upload: %v", err)
	}
	defer file.Close()

	keyName := fmt.Sprintf("%s-%s", uuid.New().String(), filepath.Base(filename))
	pbar.SetMessagef("Uploading %s to %s:%s\n", filename, bucketName, keyName)
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
	ami, snapshot, err := a.Register(imageName, bucketName, keyName, nil, targetArch, &bootMode, nil)
	fmt.Fprintf(osStdout, "Deleted S3 object %s:%s\n", bucketName, keyName)
	fmt.Fprintf(osStdout, "AMI registered: %s\nSnapshot ID: %s\n", aws.StringValue(ami), aws.StringValue(snapshot))
	if err != nil {
		return err
	}
	return nil
}
