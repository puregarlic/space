package storage

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var s3Client *s3.Client

func S3() *s3.Client {
	if s3Client != nil {
		return s3Client
	}

	sdkConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Printf("Couldn't load default configuration. Here's why: %v\n", err)
		panic(err)
	}

	svc := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + os.Getenv("AWS_S3_ENDPOINT"))
		o.Region = os.Getenv("AWS_REGION")
	})

	s3Client = svc

	return svc
}
