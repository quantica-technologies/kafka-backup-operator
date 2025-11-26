package usecase

import (
	"context"

	"github.com/quantica-technologies/kafka-backup/internal/domain"
)

// RestoreUseCase defines the interface for restore operations
type RestoreUseCase interface {
	// CreateRestore creates a new restore job
	CreateRestore(ctx context.Context, restore *domain.Restore) error

	// StartRestore starts a restore process
	StartRestore(ctx context.Context, restoreID string) error

	// GetRestore retrieves restore details
	GetRestore(ctx context.Context, restoreID string) (*domain.Restore, error)

	// ListRestores lists all restores
	ListRestores(ctx context.Context, filters RestoreFilters) ([]*domain.Restore, error)

	// DeleteRestore deletes a restore
	DeleteRestore(ctx context.Context, restoreID string) error

	// GetRestoreStatus gets the current status of a restore
	GetRestoreStatus(ctx context.Context, restoreID string) (*domain.RestoreStatus, error)

	// UpdateRestoreStatus updates the status of a restore
	UpdateRestoreStatus(ctx context.Context, restoreID string, status *domain.RestoreStatus) error

	// ValidateRestore validates a restore configuration
	ValidateRestore(ctx context.Context, restore *domain.Restore) error
}

// RestoreFilters defines filters for listing restores
type RestoreFilters struct {
	Phase         domain.RestorePhase
	BackupID      string
	TargetID      string
	CreatedAfter  *string
	CreatedBefore *string
}
