package uploader_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cheggaaa/pb/v3"

	"github.com/osbuild/bootc-image-builder/bib/internal/uploader"
)

type FakeAwsUploader struct {
	uploadCalled   int
	registerCalled int
}

func (f *FakeAwsUploader) UploadFromReader(r io.Reader, bucketName, keyName string) (*s3manager.UploadOutput, error) {
	f.uploadCalled++

	if _, err := io.ReadAll(r); err != nil {
		panic(err)
	}

	return &s3manager.UploadOutput{Location: "some-location"}, nil
}

func (f *FakeAwsUploader) Register(name, bucket, key string, shareWith []string, rpmArch string, bootMode *string) (*string, *string, error) {
	f.registerCalled++

	s1 := "ret1"
	s2 := "ret2"
	return &s1, &s2, nil
}

func TestUploadAndRegisterNoProgressBar(t *testing.T) {
	fakeStdout := bytes.NewBuffer(nil)
	restore := uploader.MockOsStdout(fakeStdout)
	defer restore()

	fakeDiskFile := filepath.Join(t.TempDir(), "fake-disk.img")
	err := os.WriteFile(fakeDiskFile, nil, 0644)
	require.Nil(t, err)
	fakeUploader := &FakeAwsUploader{}

	err = uploader.UploadAndRegister(fakeUploader, fakeDiskFile, "bucketName", "imageName", "", nil)
	require.Nil(t, err)

	assert.Equal(t, fakeUploader.uploadCalled, 1)
	assert.Equal(t, fakeUploader.registerCalled, 1)

	assert.Contains(t, fakeStdout.String(), "Uploading ")
	assert.Contains(t, fakeStdout.String(), "Registering AMI ")
}

func TestUploadAndRegisterProgressBar(t *testing.T) {
	if os.Getenv("BIB_TESTING_FARM") == "1" {
		t.Skip("for inexplicable reasons this test fails in testing farm")
		return
	}

	fakeStdout := bytes.NewBuffer(nil)
	restore := uploader.MockOsStdout(fakeStdout)
	defer restore()

	fakeDiskFile := filepath.Join(t.TempDir(), "fake-disk.img")
	err := os.WriteFile(fakeDiskFile, nil, 0644)
	require.Nil(t, err)
	err = os.Truncate(fakeDiskFile, 10*1024*1024)
	require.Nil(t, err)

	fakeUploader := &FakeAwsUploader{}

	pbar := pb.New(0)

	err = uploader.UploadAndRegister(fakeUploader, fakeDiskFile, "bucketName", "imageName", "", pbar)
	require.Nil(t, err)

	assert.Equal(t, fakeUploader.uploadCalled, 1)
	assert.Equal(t, fakeUploader.registerCalled, 1)

	assert.Contains(t, fakeStdout.String(), "Uploading ")
	assert.Regexp(t, `10.00 MiB / 10.00 MiB \[-+\] 100.00%`, fakeStdout.String())
	assert.Contains(t, fakeStdout.String(), "Registering AMI ")
}
