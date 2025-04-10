package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/osbuild/images/pkg/cloud/awscloud"
)

// check can be deferred from the top of command functions to exit with an
// error code after any other defers are run in the same scope.
func check(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}
}

func uploadAMI(cmd *cobra.Command, args []string) {
	filename := args[0]
	flags := cmd.Flags()

	region, err := flags.GetString("region")
	check(err)
	bucketName, err := flags.GetString("bucket")
	check(err)
	imageName, err := flags.GetString("ami-name")
	check(err)
	targetArch, err := flags.GetString("target-arch")
	check(err)

	opts := &awscloud.UploaderOptions{
		TargetArch: targetArch,
	}
	uploader, err := awscloud.NewUploader(region, bucketName, imageName, opts)
	check(err)

	f, err := os.Open(filename)
	check(err)
	// nolint:errcheck
	defer f.Close()

	check(uploader.UploadAndRegister(f, os.Stderr))
}

func setupCLI() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:                   "upload",
		Long:                  "Upload an image to a cloud provider",
		DisableFlagsInUseLine: true,
	}

	awsCmd := &cobra.Command{
		Use:                   "aws <image>",
		Long:                  "Upload an AMI to AWS.\n\nRequires AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY to be set in the environment",
		Args:                  cobra.ExactArgs(1), // image file
		Run:                   uploadAMI,
		DisableFlagsInUseLine: true,
	}
	awsCmd.Flags().String("region", "", "target region")
	awsCmd.Flags().String("bucket", "", "target S3 bucket name")
	awsCmd.Flags().String("ami-name", "", "AMI name")

	check(awsCmd.MarkFlagRequired("region"))
	check(awsCmd.MarkFlagRequired("bucket"))
	check(awsCmd.MarkFlagRequired("ami-name"))
	rootCmd.AddCommand(awsCmd)

	return rootCmd
}

func main() {
	logrus.SetLevel(logrus.ErrorLevel)
	cmd := setupCLI()
	check(cmd.Execute())
}
