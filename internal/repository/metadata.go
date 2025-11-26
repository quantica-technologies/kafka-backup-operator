package repository

import (
	"context"

	"github.com/quantica-technologies/kafka-backup/internal/domain"
)

// MetadataRepository defines operations for backup/restore metadata
type MetadataRepository interface {
	// SaveBackupMetadata saves backup metadata
	SaveBackupMetadata(ctx context.Context, metadata *domain.BackupMetadata) error

	// GetBackupMetadata retrieves backup metadata
	GetBackupMetadata(ctx context.Context, backupID string) (*domain.BackupMetadata, error)

	// ListBackupMetadata lists all backup metadata with filters
	ListBackupMetadata(ctx context.Context, filters MetadataFilters) ([]*domain.BackupMetadata, error)

	// DeleteBackupMetadata deletes backup metadata
	DeleteBackupMetadata(ctx context.Context, backupID string) error

	// SaveSegmentMetadata saves segment metadata
	SaveSegmentMetadata(ctx context.Context, backupID string, segment *domain.SegmentMetadata) error

	// GetSegmentMetadata retrieves segment metadata
	GetSegmentMetadata(ctx context.Context, backupID string, topic string, partition int32) ([]*domain.SegmentMetadata, error)
}

// MetadataFilters defines filters for listing metadata
type MetadataFilters struct {
	TimestampAfter  *string
	TimestampBefore *string
	Topics          []string
	Limit           int
	Offset          int
}
