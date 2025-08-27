package main_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/cloud/awscloud"

	main "github.com/osbuild/bootc-image-builder/bib/cmd/bootc-image-builder"
)

func TestCanChownInPathHappy(t *testing.T) {
	tmpdir := t.TempDir()
	canChown, err := main.CanChownInPath(tmpdir)
	require.Nil(t, err)
	assert.Equal(t, canChown, true)

	// no tmpfile leftover
	content, err := os.ReadDir(tmpdir)
	require.Nil(t, err)
	assert.Equal(t, len(content), 0)
}

func TestCanChownInPathNotExists(t *testing.T) {
	canChown, err := main.CanChownInPath("/does/not/exists")
	assert.Equal(t, canChown, false)
	assert.ErrorContains(t, err, ": no such file or directory")
}

func TestCanChownInPathCannotChange(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot run as root (fchown never errors here)")
	}

	restore := main.MockOsGetuid(func() int {
		return -2
	})
	defer restore()

	tmpdir := t.TempDir()
	canChown, err := main.CanChownInPath(tmpdir)
	require.Nil(t, err)
	assert.Equal(t, canChown, false)
}

func mockOsArgs(new []string) (restore func()) {
	saved := os.Args
	os.Args = append([]string{"argv0"}, new...)
	return func() {
		os.Args = saved
	}
}

func addRunLog(rootCmd *cobra.Command, runeCall *string) {
	for _, cmd := range rootCmd.Commands() {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			callStr := fmt.Sprintf("<%v>: %v", cmd.Name(), strings.Join(args, ","))
			if *runeCall != "" {
				panic(fmt.Sprintf("runE called with %v but already called before: %v", callStr, *runeCall))
			}
			*runeCall = callStr
			return nil
		}
	}
}

func TestCobraCmdline(t *testing.T) {
	for _, tc := range []struct {
		cmdline      []string
		expectedCall string
	}{
		// trivial: cmd is given explicitly
		{
			[]string{"manifest", "quay.io..."},
			"<manifest>: quay.io...",
		},
		{
			[]string{"build", "quay.io..."},
			"<build>: quay.io...",
		},
		{
			[]string{"version", "quay.io..."},
			"<version>: quay.io...",
		},
		// implicit: no cmd like build/manifest defaults to build
		{
			[]string{"--local", "quay.io..."},
			"<build>: quay.io...",
		},
		{
			[]string{"quay.io..."},
			"<build>: quay.io...",
		},
	} {
		var runeCall string

		restore := mockOsArgs(tc.cmdline)
		defer restore()

		rootCmd, err := main.BuildCobraCmdline()
		assert.NoError(t, err)
		addRunLog(rootCmd, &runeCall)

		t.Run(tc.expectedCall, func(t *testing.T) {
			err = rootCmd.Execute()
			assert.NoError(t, err)
			assert.Equal(t, runeCall, tc.expectedCall)
		})
	}
}

func TestCobraCmdlineVerbose(t *testing.T) {
	for _, tc := range []struct {
		cmdline             []string
		expectedProgress    string
		expectedLogrusLevel logrus.Level
	}{
		{
			[]string{"quay.io..."},
			"auto",
			logrus.ErrorLevel,
		},
		{
			[]string{"-v", "quay.io..."},
			"verbose",
			logrus.InfoLevel,
		},
	} {
		restore := mockOsArgs(tc.cmdline)
		defer restore()

		rootCmd, err := main.BuildCobraCmdline()
		assert.NoError(t, err)

		// collect progressFlag value
		var progressFlag string
		for _, cmd := range rootCmd.Commands() {
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				if progressFlag != "" {
					t.Error("progressFlag set twice")
				}
				progressFlag, err = cmd.Flags().GetString("progress")
				assert.NoError(t, err)
				return nil
			}
		}

		t.Run(tc.expectedProgress, func(t *testing.T) {
			err = rootCmd.Execute()
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedProgress, progressFlag)
			assert.Equal(t, tc.expectedLogrusLevel, logrus.GetLevel())
		})
	}
}

type fakeAwsUploader struct {
	checkCalls int

	region, bucket, ami string
	opts                *awscloud.UploaderOptions

	uploadAndRegisterRead  bytes.Buffer
	uploadAndRegisterCalls int
	uploadAndRegisterErr   error
}

var _ = cloud.Uploader(&fakeAwsUploader{})

func (fa *fakeAwsUploader) Check(status io.Writer) error {
	fa.checkCalls++
	return nil
}

func (fa *fakeAwsUploader) UploadAndRegister(r io.Reader, size uint64, status io.Writer) error {
	fa.uploadAndRegisterCalls++
	_, err := io.Copy(&fa.uploadAndRegisterRead, r)
	if err != nil {
		panic(err)
	}
	return fa.uploadAndRegisterErr
}

func TestHandleAWSFlags(t *testing.T) {
	for _, tc := range []struct {
		extraArgs    []string
		expectedOpts *awscloud.UploaderOptions
	}{
		{nil, &awscloud.UploaderOptions{TargetArch: arch.Current()}},
		{[]string{"--target-arch=aarch64"}, &awscloud.UploaderOptions{TargetArch: arch.ARCH_AARCH64}},
	} {
		var fau fakeAwsUploader
		t.Cleanup(main.MockAwscloudNewUploader(func(region string, bucket string, ami string, opts *awscloud.UploaderOptions) (cloud.Uploader, error) {
			fau.region = region
			fau.bucket = bucket
			fau.ami = ami
			fau.opts = opts
			return &fau, nil
		}))

		rootCmd, err := main.BuildCobraCmdline()
		assert.NoError(t, err)
		// Commands() returns commandsordered by name
		buildCmd := rootCmd.Commands()[0]
		assert.Equal(t, "build", buildCmd.Name())
		err = buildCmd.ParseFlags(append([]string{
			"--aws-bucket=aws-bucket",
			"--aws-ami-name=aws-ami-name",
			"--aws-region=aws-region",
			"--type=ami",
		}, tc.extraArgs...))
		assert.NoError(t, err)

		uploader, err := main.HandleAWSFlags(buildCmd)
		assert.NoError(t, err)
		assert.NotNil(t, uploader)
		assert.Equal(t, 1, fau.checkCalls)
		assert.Equal(t, tc.expectedOpts, fau.opts)
	}
}
