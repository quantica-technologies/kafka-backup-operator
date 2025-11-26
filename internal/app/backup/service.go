package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
	"github.com/quantica-technologies/kafka-backup-operator/internal/repository"
	"github.com/quantica-technologies/kafka-backup-operator/internal/usecase"
	"github.com/quantica-technologies/kafka-backup-operator/pkg/logger"
)

// Service implements the backup use case
type Service struct {
	kafkaRepo    repository.KafkaRepository
	storageRepo  repository.StorageRepository
	metadataRepo repository.MetadataRepository
	stateRepo    repository.StateRepository
	logger       logger.Logger
}

// NewService creates a new backup service
func NewService(
	kafkaRepo repository.KafkaRepository,
	storageRepo repository.StorageRepository,
	metadataRepo repository.MetadataRepository,
	stateRepo repository.StateRepository,
	logger logger.Logger,
) usecase.BackupUseCase {
	return &Service{
		kafkaRepo:    kafkaRepo,
		storageRepo:  storageRepo,
		metadataRepo: metadataRepo,
		stateRepo:    stateRepo,
		logger:       logger,
	}
}

// CreateBackup creates a new backup job
func (s *Service) CreateBackup(ctx context.Context, backup *domain.Backup) error {
	s.logger.Info("Creating backup", "backupID", backup.ID)

	// Validate backup configuration
	if err := s.validateBackup(backup); err != nil {
		return fmt.Errorf("invalid backup configuration: %w", err)
	}

	// Check Kafka cluster connectivity
	if err := s.kafkaRepo.HealthCheck(ctx, backup.SourceCluster); err != nil {
		return fmt.Errorf("kafka cluster health check failed: %w", err)
	}

	// Check storage connectivity
	if err := s.storageRepo.HealthCheck(ctx); err != nil {
		return fmt.Errorf("storage health check failed: %w", err)
	}

	// Initialize backup status
	backup.Status = domain.BackupStatus{
		Phase:   domain.BackupPhasePending,
		Workers: make([]domain.WorkerStatus, backup.WorkerConfig.Workers),
	}
	backup.CreatedAt = time.Now()
	backup.UpdatedAt = time.Now()

	// Save backup state
	if err := s.stateRepo.SaveBackupState(ctx, backup); err != nil {
		return fmt.Errorf("failed to save backup state: %w", err)
	}

	s.logger.Info("Backup created successfully", "backupID", backup.ID)
	return nil
}

// StartBackup starts a backup process
func (s *Service) StartBackup(ctx context.Context, backupID string) error {
	s.logger.Info("Starting backup", "backupID", backupID)

	// Retrieve backup state
	backup, err := s.stateRepo.GetBackupState(ctx, backupID)
	if err != nil {
		return fmt.Errorf("failed to get backup state: %w", err)
	}

	// Check if backup is already running
	if backup.Status.Phase == domain.BackupPhaseRunning {
		return fmt.Errorf("backup is already running")
	}

	// Discover topics
	admin, err := s.kafkaRepo.CreateAdmin(ctx, backup.SourceCluster)
	if err != nil {
		return fmt.Errorf("failed to create admin client: %w", err)
	}
	defer admin.Close()

	allTopics, err := admin.ListTopics(ctx)
	if err != nil {
		return fmt.Errorf("failed to list topics: %w", err)
	}

	// Filter topics based on selectors
	matchedTopics := backup.MatchesTopics(allTopics)
	s.logger.Info("Topics discovered", "total", len(allTopics), "matched", len(matchedTopics))

	// Distribute topics across workers
	topicDistribution := backup.DistributeTopics(matchedTopics)

	// Update backup status
	backup.Status.Phase = domain.BackupPhaseRunning
	backup.Status.TopicsDiscovered = len(matchedTopics)
	backup.Status.Workers = make([]domain.WorkerStatus, backup.WorkerConfig.Workers)

	for workerID, topics := range topicDistribution {
		topicNames := make([]string, len(topics))
		for i, t := range topics {
			topicNames[i] = t.Name
		}
		backup.Status.Workers[workerID] = domain.WorkerStatus{
			ID:            workerID,
			Topics:        topicNames,
			Status:        "running",
			LastHeartbeat: time.Now(),
		}
	}

	if err := s.stateRepo.UpdateBackupState(ctx, backupID, &backup.Status); err != nil {
		return fmt.Errorf("failed to update backup state: %w", err)
	}

	// Start worker coordinator
	coordinator := NewCoordinator(
		backup,
		topicDistribution,
		s.kafkaRepo,
		s.storageRepo,
		s.metadataRepo,
		s.stateRepo,
		s.logger,
	)

	go coordinator.Start(ctx)

	s.logger.Info("Backup started successfully", "backupID", backupID)
	return nil
}

// StopBackup stops a running backup
func (s *Service) StopBackup(ctx context.Context, backupID string) error {
	s.logger.Info("Stopping backup", "backupID", backupID)

	backup, err := s.stateRepo.GetBackupState(ctx, backupID)
	if err != nil {
		return fmt.Errorf("failed to get backup state: %w", err)
	}

	if backup.Status.Phase != domain.BackupPhaseRunning {
		return fmt.Errorf("backup is not running")
	}

	// Update status to suspended
	backup.Status.Phase = domain.BackupPhaseSuspended
	if err := s.stateRepo.UpdateBackupState(ctx, backupID, &backup.Status); err != nil {
		return fmt.Errorf("failed to update backup state: %w", err)
	}

	s.logger.Info("Backup stopped successfully", "backupID", backupID)
	return nil
}

// GetBackup retrieves backup details
func (s *Service) GetBackup(ctx context.Context, backupID string) (*domain.Backup, error) {
	return s.stateRepo.GetBackupState(ctx, backupID)
}

// ListBackups lists all backups
func (s *Service) ListBackups(ctx context.Context, filters usecase.BackupFilters) ([]*domain.Backup, error) {
	// For now, return all backups - in production, implement filtering
	return s.stateRepo.ListBackupStates(ctx)
}

// DeleteBackup deletes a backup
func (s *Service) DeleteBackup(ctx context.Context, backupID string) error {
	s.logger.Info("Deleting backup", "backupID", backupID)

	// Stop backup if running
	backup, err := s.stateRepo.GetBackupState(ctx, backupID)
	if err != nil {
		return fmt.Errorf("failed to get backup state: %w", err)
	}

	if backup.Status.Phase == domain.BackupPhaseRunning {
		if err := s.StopBackup(ctx, backupID); err != nil {
			s.logger.Warn("Failed to stop backup before deletion", "error", err)
		}
	}

	// Delete backup state
	if err := s.stateRepo.DeleteBackupState(ctx, backupID); err != nil {
		return fmt.Errorf("failed to delete backup state: %w", err)
	}

	// Delete backup metadata
	if err := s.metadataRepo.DeleteBackupMetadata(ctx, backupID); err != nil {
		s.logger.Warn("Failed to delete backup metadata", "error", err)
	}

	s.logger.Info("Backup deleted successfully", "backupID", backupID)
	return nil
}

// GetBackupStatus gets the current status of a backup
func (s *Service) GetBackupStatus(ctx context.Context, backupID string) (*domain.BackupStatus, error) {
	backup, err := s.stateRepo.GetBackupState(ctx, backupID)
	if err != nil {
		return nil, err
	}
	return &backup.Status, nil
}

// UpdateBackupStatus updates the status of a backup
func (s *Service) UpdateBackupStatus(ctx context.Context, backupID string, status *domain.BackupStatus) error {
	return s.stateRepo.UpdateBackupState(ctx, backupID, status)
}

func (s *Service) validateBackup(backup *domain.Backup) error {
	if backup.SourceCluster == nil {
		return fmt.Errorf("source cluster is required")
	}

	if backup.TargetStorage == nil {
		return fmt.Errorf("target storage is required")
	}

	if len(backup.TopicSelectors) == 0 {
		return fmt.Errorf("at least one topic selector is required")
	}

	if backup.WorkerConfig.Workers < 1 {
		return fmt.Errorf("at least one worker is required")
	}

	if backup.BatchSize < 1 {
		return fmt.Errorf("batch size must be at least 1")
	}

	return nil
}
