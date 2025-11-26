package usecase

import (
	"context"

	"github.com/quantica-technologies/kafka-backup/internal/domain"
)

// BackupUseCase defines the interface for backup operations
type BackupUseCase interface {
	// CreateBackup creates a new backup job
	CreateBackup(ctx context.Context, backup *domain.Backup) error

	// StartBackup starts a backup process
	StartBackup(ctx context.Context, backupID string) error

	// StopBackup stops a running backup
	StopBackup(ctx context.Context, backupID string) error

	// GetBackup retrieves backup details
	GetBackup(ctx context.Context, backupID string) (*domain.Backup, error)

	// ListBackups lists all backups
	ListBackups(ctx context.Context, filters BackupFilters) ([]*domain.Backup, error)

	// DeleteBackup deletes a backup
	DeleteBackup(ctx context.Context, backupID string) error

	// GetBackupStatus gets the current status of a backup
	GetBackupStatus(ctx context.Context, backupID string) (*domain.BackupStatus, error)

	// UpdateBackupStatus updates the status of a backup
	UpdateBackupStatus(ctx context.Context, backupID string, status *domain.BackupStatus) error
}

// BackupFilters defines filters for listing backups
type BackupFilters struct {
	Phase         domain.BackupPhase
	SourceID      string
	StorageID     string
	CreatedAfter  *string
	CreatedBefore *string
}
