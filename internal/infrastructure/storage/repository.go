package storage

import (
	"fmt"

	"github.com/quantica-technologies/kafka-backup-operator/internal/config"
	"github.com/quantica-technologies/kafka-backup-operator/internal/repository"
)

// NewRepository creates a new storage repository based on config
func NewRepository(cfg config.StorageConfig) (repository.StorageRepository, error) {
	switch cfg.Type {
	case "s3":
		return NewS3Repository(cfg)
	case "azure":
		return nil, fmt.Errorf("azure storage not yet implemented")
	case "gcs":
		return nil, fmt.Errorf("gcs storage not yet implemented")
	case "local":
		return NewLocalRepository(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}

// NewLocalRepository creates a local filesystem storage repository
func NewLocalRepository(cfg config.StorageConfig) (repository.StorageRepository, error) {
	// Implementation in storage/local.go
	return &LocalRepository{
		basePath: cfg.Path,
		prefix:   cfg.Prefix,
	}, nil
}

// NewS3Repository creates an S3 storage repository
func NewS3Repository(cfg config.StorageConfig) (repository.StorageRepository, error) {
	// Implementation in storage/s3.go
	return &S3Repository{
		bucket: cfg.Path,
		region: cfg.Region,
		prefix: cfg.Prefix,
		config: cfg.S3,
	}, nil
}
