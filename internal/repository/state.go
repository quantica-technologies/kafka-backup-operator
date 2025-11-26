package repository

import (
	"context"

	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
)

// StateRepository defines operations for managing backup/restore state
type StateRepository interface {
	// SaveBackupState saves backup state
	SaveBackupState(ctx context.Context, backup *domain.Backup) error

	// GetBackupState retrieves backup state
	GetBackupState(ctx context.Context, backupID string) (*domain.Backup, error)

	// UpdateBackupState updates backup state
	UpdateBackupState(ctx context.Context, backupID string, status *domain.BackupStatus) error

	// ListBackupStates lists all backup states
	ListBackupStates(ctx context.Context) ([]*domain.Backup, error)

	// DeleteBackupState deletes backup state
	DeleteBackupState(ctx context.Context, backupID string) error

	// SaveRestoreState saves restore state
	SaveRestoreState(ctx context.Context, restore *domain.Restore) error

	// GetRestoreState retrieves restore state
	GetRestoreState(ctx context.Context, restoreID string) (*domain.Restore, error)

	// UpdateRestoreState updates restore state
	UpdateRestoreState(ctx context.Context, restoreID string, status *domain.RestoreStatus) error

	// ListRestoreStates lists all restore states
	ListRestoreStates(ctx context.Context) ([]*domain.Restore, error)

	// DeleteRestoreState deletes restore state
	DeleteRestoreState(ctx context.Context, restoreID string) error

	// Lock acquires a distributed lock
	Lock(ctx context.Context, key string, ttl int) (Lock, error)
}

// Lock represents a distributed lock
type Lock interface {
	// Unlock releases the lock
	Unlock(ctx context.Context) error

	// Refresh refreshes the lock TTL
	Refresh(ctx context.Context, ttl int) error
}
