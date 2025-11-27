package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/quantica-technologies/kafka-backup-operator/pkg/config"
)

// Storage defines the interface for storage backends
type Storage interface {
	// Put stores data at the given key
	Put(ctx context.Context, key string, data io.Reader) error

	// Get retrieves data from the given key
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// List returns all keys with the given prefix
	List(ctx context.Context, prefix string) ([]string, error)

	// Delete removes data at the given key
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)

	// Close closes the storage connection
	Close() error
}

// NewStorage creates a new storage backend based on configuration
func NewStorage(ctx context.Context, cfg config.StorageConfig) (Storage, error) {
	switch cfg.Type {
	case "s3":
		return NewS3Storage(ctx, cfg)
	case "local":
		return NewLocalStorage(cfg)
	case "azure":
		return nil, fmt.Errorf("azure storage not yet implemented")
	case "gcs":
		return nil, fmt.Errorf("gcs storage not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}

// S3Storage implements Storage for AWS S3
type S3Storage struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Storage creates a new S3 storage backend
func NewS3Storage(ctx context.Context, cfg config.StorageConfig) (*S3Storage, error) {
	var awsCfg aws.Config
	var err error

	// Load AWS configuration
	if cfg.S3AccessKeyID != "" && cfg.S3SecretAccessKey != "" {
		awsCfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(cfg.Region),
			awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					cfg.S3AccessKeyID,
					cfg.S3SecretAccessKey,
					"",
				),
			),
		)
	} else {
		awsCfg, err = awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint if provided
	var s3Client *s3.Client
	if cfg.S3Endpoint != "" {
		s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = true
		})
	} else {
		s3Client = s3.NewFromConfig(awsCfg)
	}

	return &S3Storage{
		client: s3Client,
		bucket: cfg.Path,
		prefix: cfg.Prefix,
	}, nil
}

func (s *S3Storage) Put(ctx context.Context, key string, data io.Reader) error {
	fullKey := filepath.Join(s.prefix, key)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
		Body:   data,
	})
	return err
}

func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	fullKey := filepath.Join(s.prefix, key)
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := filepath.Join(s.prefix, prefix)
	var keys []string

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
			keys = append(keys, *obj.Key)
		}
	}

	return keys, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	fullKey := filepath.Join(s.prefix, key)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	return err
}

func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := filepath.Join(s.prefix, key)
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (s *S3Storage) Close() error {
	return nil
}

// LocalStorage implements Storage for local filesystem
type LocalStorage struct {
	basePath string
	prefix   string
}

// NewLocalStorage creates a new local storage backend
func NewLocalStorage(cfg config.StorageConfig) (*LocalStorage, error) {
	basePath := cfg.Path
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalStorage{
		basePath: basePath,
		prefix:   cfg.Prefix,
	}, nil
}

func (l *LocalStorage) getFullPath(key string) string {
	return filepath.Join(l.basePath, l.prefix, key)
}

func (l *LocalStorage) Put(ctx context.Context, key string, data io.Reader) error {
	fullPath := l.getFullPath(key)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create and write to file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	return err
}

func (l *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	fullPath := l.getFullPath(key)
	return os.Open(fullPath)
}

func (l *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	searchPath := l.getFullPath(prefix)
	var keys []string

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(filepath.Join(l.basePath, l.prefix), path)
			if err != nil {
				return err
			}
			keys = append(keys, relPath)
		}
		return nil
	})

	return keys, err
}

func (l *LocalStorage) Delete(ctx context.Context, key string) error {
	fullPath := l.getFullPath(key)
	return os.Remove(fullPath)
}

func (l *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	fullPath := l.getFullPath(key)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (l *LocalStorage) Close() error {
	return nil
}
