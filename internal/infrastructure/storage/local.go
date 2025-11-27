package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/quantica-technologies/kafka-backup-operator/internal/repository"
)

// LocalRepository implements StorageRepository for local filesystem
type LocalRepository struct {
	basePath string
	prefix   string
}

func (l *LocalRepository) Put(ctx context.Context, key string, data io.Reader, metadata *repository.ObjectMetadata) error {
	fullPath := filepath.Join(l.basePath, l.prefix, key)

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

func (l *LocalRepository) Get(ctx context.Context, key string) (io.ReadCloser, *repository.ObjectMetadata, error) {
	fullPath := filepath.Join(l.basePath, l.prefix, key)

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	}

	metadata := &repository.ObjectMetadata{
		Key:  key,
		Size: info.Size(),
	}

	return file, metadata, nil
}

func (l *LocalRepository) List(ctx context.Context, prefix string) ([]*repository.ObjectInfo, error) {
	searchPath := filepath.Join(l.basePath, l.prefix, prefix)
	var objects []*repository.ObjectInfo

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(filepath.Join(l.basePath, l.prefix), path)
			if err != nil {
				return err
			}
			objects = append(objects, &repository.ObjectInfo{
				Key:  relPath,
				Size: info.Size(),
			})
		}
		return nil
	})

	return objects, err
}

func (l *LocalRepository) Delete(ctx context.Context, key string) error {
	fullPath := filepath.Join(l.basePath, l.prefix, key)
	return os.Remove(fullPath)
}

func (l *LocalRepository) Exists(ctx context.Context, key string) (bool, error) {
	fullPath := filepath.Join(l.basePath, l.prefix, key)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (l *LocalRepository) GetMetadata(ctx context.Context, key string) (*repository.ObjectMetadata, error) {
	fullPath := filepath.Join(l.basePath, l.prefix, key)

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &repository.ObjectMetadata{
		Key:  key,
		Size: info.Size(),
	}, nil
}

func (l *LocalRepository) Close() error {
	return nil
}

func (l *LocalRepository) HealthCheck(ctx context.Context) error {
	// Check if base directory is accessible
	_, err := os.Stat(l.basePath)
	return err
}
