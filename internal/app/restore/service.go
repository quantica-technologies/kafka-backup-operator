package restore

import (
	"context"
	"fmt"
	"time"

	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
	"github.com/quantica-technologies/kafka-backup-operator/internal/repository"
	"github.com/quantica-technologies/kafka-backup-operator/internal/usecase"
	"github.com/quantica-technologies/kafka-backup-operator/pkg/logger"
)

// Service implements the restore use case
type Service struct {
	kafkaRepo    repository.KafkaRepository
	storageRepo  repository.StorageRepository
	metadataRepo repository.MetadataRepository
	stateRepo    repository.StateRepository
	logger       logger.Logger
}

// NewService creates a new restore service
func NewService(
	kafkaRepo repository.KafkaRepository,
	storageRepo repository.StorageRepository,
	metadataRepo repository.MetadataRepository,
	stateRepo repository.StateRepository,
	logger logger.Logger,
) usecase.RestoreUseCase {
	return &Service{
		kafkaRepo:    kafkaRepo,
		storageRepo:  storageRepo,
		metadataRepo: metadataRepo,
		stateRepo:    stateRepo,
		logger:       logger,
	}
}

// CreateRestore creates a new restore job
func (s *Service) CreateRestore(ctx context.Context, restore *domain.Restore) error {
	s.logger.Info("Creating restore", "restoreID", restore.ID)

	// Validate restore configuration
	if err := s.validateRestore(restore); err != nil {
		return fmt.Errorf("invalid restore configuration: %w", err)
	}

	// Initialize restore status
	restore.Status = domain.RestoreStatus{
		Phase: domain.RestorePhasePending,
	}
	restore.CreatedAt = time.Now()
	restore.UpdatedAt = time.Now()

	// Save restore state
	if err := s.stateRepo.SaveRestoreState(ctx, restore); err != nil {
		return fmt.Errorf("failed to save restore state: %w", err)
	}

	s.logger.Info("Restore created successfully", "restoreID", restore.ID)
	return nil
}

// ValidateRestore validates a restore configuration
func (s *Service) ValidateRestore(ctx context.Context, restore *domain.Restore) error {
	s.logger.Info("Validating restore", "restoreID", restore.ID)

	// Update status to validating
	restore.Status.Phase = domain.RestorePhaseValidating
	if err := s.stateRepo.UpdateRestoreState(ctx, restore.ID, &restore.Status); err != nil {
		return err
	}

	// Check target Kafka cluster connectivity
	if err := s.kafkaRepo.HealthCheck(ctx, restore.TargetCluster); err != nil {
		return fmt.Errorf("target cluster health check failed: %w", err)
	}

	// Check storage connectivity
	if err := s.storageRepo.HealthCheck(ctx); err != nil {
		return fmt.Errorf("storage health check failed: %w", err)
	}

	// Verify backup exists
	backupMeta, err := s.metadataRepo.GetBackupMetadata(ctx, restore.BackupID)
	if err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}

	s.logger.Info("Backup found", "backupID", backupMeta.BackupID, "topics", len(backupMeta.Topics))

	// Validate topic selectors if specified
	if restore.TopicSelectors != nil {
		availableTopics := make(map[string]bool)
		for _, topicMeta := range backupMeta.Topics {
			availableTopics[topicMeta.Name] = true
		}

		// Check if selected topics exist in backup
		for _, selector := range restore.TopicSelectors {
			found := false
			for topicName := range availableTopics {
				if selector.Matches(topicName) {
					found = true
					break
				}
			}
			if !found {
				s.logger.Warn("No topics match selector", "pattern", selector.Pattern)
			}
		}
	}

	s.logger.Info("Restore validation completed successfully")
	return nil
}

// StartRestore starts a restore process
func (s *Service) StartRestore(ctx context.Context, restoreID string) error {
	s.logger.Info("Starting restore", "restoreID", restoreID)

	// Retrieve restore state
	restore, err := s.stateRepo.GetRestoreState(ctx, restoreID)
	if err != nil {
		return fmt.Errorf("failed to get restore state: %w", err)
	}

	// Check if restore is already running
	if restore.Status.Phase == domain.RestorePhaseRunning {
		return fmt.Errorf("restore is already running")
	}

	// Validate restore before starting
	if err := s.ValidateRestore(ctx, restore); err != nil {
		restore.Status.Phase = domain.RestorePhaseFailed
		restore.Status.Errors = append(restore.Status.Errors, err.Error())
		s.stateRepo.UpdateRestoreState(ctx, restoreID, &restore.Status)
		return err
	}

	// Get backup metadata
	backupMeta, err := s.metadataRepo.GetBackupMetadata(ctx, restore.BackupID)
	if err != nil {
		return fmt.Errorf("failed to get backup metadata: %w", err)
	}

	// Filter topics based on selectors
	topicsToRestore := s.filterTopics(backupMeta.Topics, restore.TopicSelectors)
	s.logger.Info("Topics to restore", "count", len(topicsToRestore))

	// Create topics if needed
	if restore.CreateTopics {
		if err := s.createTopics(ctx, restore.TargetCluster, topicsToRestore); err != nil {
			s.logger.Warn("Failed to create some topics", "error", err)
		}
	}

	// Update restore status
	now := time.Now()
	restore.Status.Phase = domain.RestorePhaseRunning
	restore.Status.StartTime = &now
	restore.Status.Progress = make([]domain.TopicProgress, len(topicsToRestore))

	for i, topicMeta := range topicsToRestore {
		restore.Status.Progress[i] = domain.TopicProgress{
			TopicName:       topicMeta.Name,
			PartitionsTotal: int(topicMeta.Partitions),
			Status:          "pending",
		}
	}

	if err := s.stateRepo.UpdateRestoreState(ctx, restoreID, &restore.Status); err != nil {
		return fmt.Errorf("failed to update restore state: %w", err)
	}

	// Start restore workers
	coordinator := NewCoordinator(
		restore,
		topicsToRestore,
		s.kafkaRepo,
		s.storageRepo,
		s.metadataRepo,
		s.stateRepo,
		s.logger,
	)

	go func() {
		if err := coordinator.Start(ctx); err != nil {
			s.logger.Error("Restore coordinator failed", "error", err)
			restore.Status.Phase = domain.RestorePhaseFailed
			restore.Status.Errors = append(restore.Status.Errors, err.Error())
		} else {
			restore.Status.Phase = domain.RestorePhaseCompleted
			completionTime := time.Now()
			restore.Status.CompletionTime = &completionTime
		}
		s.stateRepo.UpdateRestoreState(ctx, restoreID, &restore.Status)
	}()

	s.logger.Info("Restore started successfully", "restoreID", restoreID)
	return nil
}

// GetRestore retrieves restore details
func (s *Service) GetRestore(ctx context.Context, restoreID string) (*domain.Restore, error) {
	return s.stateRepo.GetRestoreState(ctx, restoreID)
}

// ListRestores lists all restores
func (s *Service) ListRestores(ctx context.Context, filters usecase.RestoreFilters) ([]*domain.Restore, error) {
	return s.stateRepo.ListRestoreStates(ctx)
}

// DeleteRestore deletes a restore
func (s *Service) DeleteRestore(ctx context.Context, restoreID string) error {
	s.logger.Info("Deleting restore", "restoreID", restoreID)
	return s.stateRepo.DeleteRestoreState(ctx, restoreID)
}

// GetRestoreStatus gets the current status of a restore
func (s *Service) GetRestoreStatus(ctx context.Context, restoreID string) (*domain.RestoreStatus, error) {
	restore, err := s.stateRepo.GetRestoreState(ctx, restoreID)
	if err != nil {
		return nil, err
	}
	return &restore.Status, nil
}

// UpdateRestoreStatus updates the status of a restore
func (s *Service) UpdateRestoreStatus(ctx context.Context, restoreID string, status *domain.RestoreStatus) error {
	return s.stateRepo.UpdateRestoreState(ctx, restoreID, status)
}

func (s *Service) validateRestore(restore *domain.Restore) error {
	if restore.SourceStorage == nil {
		return fmt.Errorf("source storage is required")
	}

	if restore.TargetCluster == nil {
		return fmt.Errorf("target cluster is required")
	}

	if restore.BackupID == "" && restore.BackupTimestamp == nil {
		return fmt.Errorf("either backup ID or timestamp is required")
	}

	if restore.Workers < 1 {
		return fmt.Errorf("at least one worker is required")
	}

	return nil
}

func (s *Service) filterTopics(
	allTopics []domain.TopicMetadata,
	selectors []domain.TopicSelector,
) []domain.TopicMetadata {
	if selectors == nil || len(selectors) == 0 {
		return allTopics
	}

	var filtered []domain.TopicMetadata
	for _, topicMeta := range allTopics {
		for _, selector := range selectors {
			if selector.Matches(topicMeta.Name) {
				filtered = append(filtered, topicMeta)
				break
			}
		}
	}

	return filtered
}

func (s *Service) createTopics(
	ctx context.Context,
	cluster *domain.KafkaCluster,
	topicMetadata []domain.TopicMetadata,
) error {
	admin, err := s.kafkaRepo.CreateAdmin(ctx, cluster)
	if err != nil {
		return err
	}
	defer admin.Close()

	for _, topicMeta := range topicMetadata {
		topic := &domain.Topic{
			Name:              topicMeta.Name,
			Partitions:        topicMeta.Partitions,
			ReplicationFactor: topicMeta.ReplicationFactor,
			Config:            topicMeta.Config,
		}

		if err := admin.CreateTopic(ctx, topic); err != nil {
			s.logger.Warn("Failed to create topic", "topic", topic.Name, "error", err)
		} else {
			s.logger.Info("Topic created", "topic", topic.Name)
		}
	}

	return nil
}
