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

	// BaseEndpoint 指向 S3 兼容服务入口：本地是 MinIO，生产是 Cloudflare R2 S3 API。
	s3Client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.Endpoint),
		Region:       region,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			// 使用环境变量中的静态访问密钥生成签名，不把密钥返回给浏览器。
			return aws.Credentials{
				AccessKeyID:     cfg.AccessKeyID,
				SecretAccessKey: cfg.SecretAccessKey,
				Source:          "rendering-cms-static",
			}, nil
		}),
		// false 时生成 R2 需要的虚拟主机风格 URL；true 时兼容本地 MinIO path-style URL。
		UsePathStyle: cfg.UsePathStyle,
	})

	return &Client{
		bucket:    cfg.Bucket,
		presigner: s3.NewPresignClient(s3Client),
	}, nil
}

func (c *Client) PresignUploadURL(ctx context.Context, key string, contentType string, expires time.Duration) (string, error) {
	// 预签名 PUT URL 只允许上传指定 key 和 Content-Type，前端需要原样携带该 Content-Type。
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
	// 下载也通过短期预签名 URL 完成，避免公开 bucket 或泄露对象存储凭据。
	request, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}
