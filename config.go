package roll

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/mitchellh/go-homedir"
	"go.uber.org/zap"
)

var (
	AWS_SHARED_CREDENTIALS_FILE = os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	AWS_PROFILE                 = os.Getenv("AWS_PROFILE")

	DefaultSharedCredentialsFile = "~/.aws/config"
	DefaultProfile               = "default"
)

func init() {
	if AWS_SHARED_CREDENTIALS_FILE == "" {
		AWS_SHARED_CREDENTIALS_FILE = DefaultSharedCredentialsFile
	}

	if AWS_PROFILE == "" {
		AWS_PROFILE = DefaultProfile
	}
}

// getAWSCredentials loads the credentials we need to access the AWS API.
func getAWSCredentials() (creds *credentials.Credentials) {
	creds = credentials.NewEnvCredentials()
	if _, err := creds.Get(); err != nil {
		path, _ := homedir.Expand(AWS_SHARED_CREDENTIALS_FILE)
		creds = credentials.NewSharedCredentials(path, AWS_PROFILE)
	}

	return creds
}

// getMetadata returns information about the current EC2 instance
func getMetadata() (data ec2metadata.EC2InstanceIdentityDocument) {
	var err error

	Log.Info("retrieving instance metadata")
	svc := ec2metadata.New(session.New(&aws.Config{}))
	if data, err = svc.GetInstanceIdentityDocument(); err != nil {
		Log.Warn("failed to retrieve instance metadata", zap.Error(err))
		return
	}

	//Log.Info("instance metadata", zap.Any("data", data))

	return data
}
