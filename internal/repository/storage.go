package repository

import (
	"context"
	"io"
)

// StorageRepository defines operations for storage backends
type StorageRepository interface {
	// Put stores data at the given key
	Put(ctx context.Context, key string, data io.Reader, metadata *ObjectMetadata) error

	// Get retrieves data from the given key
	Get(ctx context.Context, key string) (io.ReadCloser, *ObjectMetadata, error)

	// List returns all keys with the given prefix
	List(ctx context.Context, prefix string) ([]*ObjectInfo, error)

	// Delete removes data at the given key
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)

	// GetMetadata gets metadata for an object
	GetMetadata(ctx context.Context, key string) (*ObjectMetadata, error)

	// Close closes the storage connection
	Close() error

	// HealthCheck checks storage connectivity
	HealthCheck(ctx context.Context) error
}

// ObjectMetadata contains metadata about stored objects
type ObjectMetadata struct {
	Key            string
	Size           int64
	ContentType    string
	LastModified   string
	ETag           string
	CustomMetadata map[string]string
}

// ObjectInfo contains basic information about a stored object
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified string
	ETag         string
}
