package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/osbuild/images/pkg/arch"
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

func newClientFromArgs(flags *pflag.FlagSet) (*awscloud.AWS, error) {
	region, err := flags.GetString("region")
	if err != nil {
		return nil, err
	}
	keyID := os.Getenv("AWS_ACCESS_KEY_ID")
	if keyID == "" {
		fmt.Fprintln(os.Stderr, "AWS_ACCESS_KEY_ID environment variable is required")
		os.Exit(1)
	}
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if secretKey == "" {
		fmt.Fprintln(os.Stderr, "AWS_SECRET_ACCESS_KEY environment variable is required")
		os.Exit(1)
	}
	return awscloud.New(region, keyID, secretKey, "")
}

func uploadAws(cmd *cobra.Command, args []string) {

	filename := args[0]
	flags := cmd.Flags()

	a, err := newClientFromArgs(flags)
	check(err)
	bucketName, err := flags.GetString("bucket")
	check(err)

	keyName := fmt.Sprintf("%s-%s", uuid.New().String(), filepath.Base(filename))

	fmt.Printf("Uploading %s to %s:%s\n", filename, bucketName, keyName)
	uploadOutput, err := a.Upload(filename, bucketName, keyName)
	check(err)

	fmt.Printf("File uploaded to %s\n", aws.StringValue(&uploadOutput.Location))

	imageName, err := flags.GetString("ami-name")
	check(err)

	hostArch := arch.Current()

	bootMode := ec2.BootModeValuesUefiPreferred
	fmt.Printf("Registering AMI %s\n", imageName)
	ami, snapshot, err := a.Register(imageName, bucketName, keyName, nil, hostArch.String(), &bootMode)
	check(err)

	fmt.Printf("AMI registered: %s\nSnapshot ID: %s\n", aws.StringValue(ami), aws.StringValue(snapshot))
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
		Run:                   uploadAws,
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
