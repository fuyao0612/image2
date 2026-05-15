package storage

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Store struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
	pathPrefix    string
}

func newS3Store(cfg Config) (*s3Store, error) {
	bucket := strings.TrimSpace(cfg.S3Bucket)
	region := strings.TrimSpace(cfg.S3Region)
	accessKeyID := strings.TrimSpace(cfg.S3AccessKeyID)
	secretAccessKey := strings.TrimSpace(cfg.S3SecretAccessKey)
	if bucket == "" || region == "" || accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("s3 image storage requires bucket, region, access key and secret key")
	}

	options := s3.Options{
		Region:      region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	}
	if endpoint := strings.TrimSpace(cfg.S3Endpoint); endpoint != "" {
		if cfg.S3DisableSSL && strings.HasPrefix(endpoint, "https://") {
			endpoint = "http://" + strings.TrimPrefix(endpoint, "https://")
		}
		options.BaseEndpoint = aws.String(strings.TrimRight(endpoint, "/"))
		options.UsePathStyle = cfg.S3ForcePathStyle
	}

	publicBaseURL := strings.TrimRight(strings.TrimSpace(cfg.S3PublicBaseURL), "/")
	if publicBaseURL == "" && strings.TrimSpace(cfg.S3Endpoint) != "" {
		publicBaseURL = strings.TrimRight(strings.TrimSpace(cfg.S3Endpoint), "/") + "/" + bucket
	}
	if publicBaseURL == "" {
		publicBaseURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", bucket, region)
	}

	return &s3Store{
		client:        s3.New(options),
		bucket:        bucket,
		publicBaseURL: publicBaseURL,
		pathPrefix:    strings.Trim(strings.TrimSpace(cfg.S3PathPrefix), "/"),
	}, nil
}

func (s *s3Store) Save(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	key = strings.Trim(strings.TrimSpace(key), "/")
	if s.pathPrefix != "" {
		key = path.Join(s.pathPrefix, key)
	}
	if key == "" {
		return "", fmt.Errorf("s3 image key is empty")
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(s.bucket),
		Key:          aws.String(key),
		Body:         readSeekable(data),
		ContentType:  aws.String(strings.TrimSpace(contentType)),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return "", fmt.Errorf("put image to s3: %w", err)
	}

	return s.publicBaseURL + "/" + strings.TrimLeft(key, "/"), nil
}

func (s *s3Store) DeleteByURL(ctx context.Context, rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("parse s3 image url: %w", err)
	}
	if parsed.Path == "" {
		return nil
	}

	key := strings.TrimLeft(parsed.Path, "/")
	key = strings.Trim(strings.TrimSpace(key), "/")
	if key == "" {
		return nil
	}

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete image from s3: %w", err)
	}
	return nil
}

func (s *s3Store) SizeByURL(ctx context.Context, rawURL string) (int64, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse s3 image url: %w", err)
	}
	if parsed.Path == "" {
		return 0, nil
	}

	key := strings.TrimLeft(parsed.Path, "/")
	key = strings.Trim(strings.TrimSpace(key), "/")
	if key == "" {
		return 0, nil
	}

	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("head image from s3: %w", err)
	}
	if resp.ContentLength == nil {
		return 0, nil
	}
	return *resp.ContentLength, nil
}
