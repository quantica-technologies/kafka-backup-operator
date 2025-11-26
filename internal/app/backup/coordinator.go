package backup

import (
	"context"
	"sync"

	"github.com/quantica-technologies/kafka-backup/internal/domain"
	"github.com/quantica-technologies/kafka-backup/internal/repository"
	"github.com/quantica-technologies/kafka-backup/pkg/logger"
)

// Coordinator manages backup workers
type Coordinator struct {
	backup            *domain.Backup
	topicDistribution map[int][]*domain.Topic
	kafkaRepo         repository.KafkaRepository
	storageRepo       repository.StorageRepository
	metadataRepo      repository.MetadataRepository
	stateRepo         repository.StateRepository
	logger            logger.Logger
	workers           []*Worker
	wg                sync.WaitGroup
}

// NewCoordinator creates a new backup coordinator
func NewCoordinator(
	backup *domain.Backup,
	topicDistribution map[int][]*domain.Topic,
	kafkaRepo repository.KafkaRepository,
	storageRepo repository.StorageRepository,
	metadataRepo repository.MetadataRepository,
	stateRepo repository.StateRepository,
	logger logger.Logger,
) *Coordinator {
	return &Coordinator{
		backup:            backup,
		topicDistribution: topicDistribution,
		kafkaRepo:         kafkaRepo,
		storageRepo:       storageRepo,
		metadataRepo:      metadataRepo,
		stateRepo:         stateRepo,
		logger:            logger,
	}
}

// Start starts all backup workers
func (c *Coordinator) Start(ctx context.Context) {
	c.logger.Info("Starting backup coordinator", "workers", len(c.topicDistribution))

	for workerID, topics := range c.topicDistribution {
		worker := NewWorker(
			workerID,
			topics,
			c.backup,
			c.kafkaRepo,
			c.storageRepo,
			c.metadataRepo,
			c.stateRepo,
			c.logger,
		)

		c.workers = append(c.workers, worker)
		c.wg.Add(1)

		go func(w *Worker) {
			defer c.wg.Done()
			if err := w.Start(ctx); err != nil {
				c.logger.Error("Worker failed", "workerID", w.id, "error", err)
			}
		}(worker)
	}

	c.wg.Wait()
	c.logger.Info("All backup workers completed")
}

// Stop stops all backup workers
func (c *Coordinator) Stop(ctx context.Context) {
	c.logger.Info("Stopping backup coordinator")
	// Implementation for graceful shutdown
}
