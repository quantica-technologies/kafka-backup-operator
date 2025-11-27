package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/quantica-technologies/kafka-backup-operator/internal/config"
	"github.com/quantica-technologies/kafka-backup-operator/internal/repository"
)

// S3Repository implements StorageRepository for AWS S3
type S3Repository struct {
	client *s3.Client
	bucket string
	region string
	prefix string
	config config.S3Config
}

func (s *S3Repository) Put(ctx context.Context, key string, data io.Reader, metadata *repository.ObjectMetadata) error {
	fullKey := fmt.Sprintf("%s/%s", s.prefix, key)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
		Body:   data,
	})
	return err
}

func (s *S3Repository) Get(ctx context.Context, key string) (io.ReadCloser, *repository.ObjectMetadata, error) {
	fullKey := fmt.Sprintf("%s/%s", s.prefix, key)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return nil, nil, err
	}

	metadata := &repository.ObjectMetadata{
		Key:  key,
		Size: *result.ContentLength,
	}

	return result.Body, metadata, nil
}

func (s *S3Repository) List(ctx context.Context, prefix string) ([]*repository.ObjectInfo, error) {
	fullPrefix := fmt.Sprintf("%s/%s", s.prefix, prefix)
	var objects []*repository.ObjectInfo

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(fullPrefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			objects = append(objects, &repository.ObjectInfo{
				Key:  *obj.Key,
				Size: *obj.Size,
			})
		}
	}

	return objects, nil
}

func (s *S3Repository) Delete(ctx context.Context, key string) error {
	fullKey := fmt.Sprintf("%s/%s", s.prefix, key)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	return err
}

func (s *S3Repository) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := fmt.Sprintf("%s/%s", s.prefix, key)

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (s *S3Repository) GetMetadata(ctx context.Context, key string) (*repository.ObjectMetadata, error) {
	fullKey := fmt.Sprintf("%s/%s", s.prefix, key)

	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return nil, err
	}

	return &repository.ObjectMetadata{
		Key:  key,
		Size: *result.ContentLength,
	}, nil
}

func (s *S3Repository) Close() error {
	return nil
}

func (s *S3Repository) HealthCheck(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	return err
}
