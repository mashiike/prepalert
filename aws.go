package prepalert

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

var (
	AWSConfigCache *aws.Config
)

func newAWSConfig() (aws.Config, error) {
	if AWSConfigCache != nil {
		return *AWSConfigCache, nil
	}
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return aws.Config{}, err
	}
	AWSConfigCache = &awsCfg
	return awsCfg, nil
}
