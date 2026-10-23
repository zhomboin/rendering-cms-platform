package storage

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"rendering-cms-platform/backend/internal/config"
)

type Client struct {
	bucket    string
	api       *s3.Client
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
		api:       s3Client,
		presigner: s3.NewPresignClient(s3Client),
	}, nil
}

func (c *Client) PresignUploadURL(ctx context.Context, key string, contentType string, byteSize int64, expires time.Duration) (string, error) {
	// 预签名 PUT URL 只允许上传指定 key、Content-Type 和 Content-Length。
	request, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(byteSize),
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

// PutObject 由后端直接写入对象，用于照片等内容需要服务端处理后再落存储的场景。
// cacheControl 写入对象元数据，公开访问时由对象存储原样返回。
func (c *Client) PutObject(ctx context.Context, key string, body []byte, contentType string, cacheControl string) error {
	_, err := c.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(c.bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(body),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String(cacheControl),
	})
	return err
}

func (c *Client) DeleteObject(ctx context.Context, key string) error {
	_, err := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}
