package storage

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"rendering-cms-platform/backend/internal/config"
)

type Client struct {
	bucket    string
	presigner *s3.PresignClient
}

func NewS3Client(cfg config.S3Config) (*Client, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, errors.New("S3 endpoint, bucket and credentials are required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	s3Client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.Endpoint),
		Region:       region,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     cfg.AccessKeyID,
				SecretAccessKey: cfg.SecretAccessKey,
				Source:          "rendering-cms-static",
			}, nil
		}),
		UsePathStyle: true,
	})

	return &Client{
		bucket:    cfg.Bucket,
		presigner: s3.NewPresignClient(s3Client),
	}, nil
}

func (c *Client) PresignUploadURL(ctx context.Context, key string, contentType string, expires time.Duration) (string, error) {
	request, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

func (c *Client) PresignDownloadURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	request, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}
