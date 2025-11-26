package restore

import (
	"context"
	"sync"

	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
	"github.com/quantica-technologies/kafka-backup-operator/internal/repository"
	"github.com/quantica-technologies/kafka-backup-operator/pkg/logger"
)

// Coordinator manages restore workers
type Coordinator struct {
	restore      *domain.Restore
	topics       []domain.TopicMetadata
	kafkaRepo    repository.KafkaRepository
	storageRepo  repository.StorageRepository
	metadataRepo repository.MetadataRepository
	stateRepo    repository.StateRepository
	logger       logger.Logger
	wg           sync.WaitGroup
}

// NewCoordinator creates a new restore coordinator
func NewCoordinator(
	restore *domain.Restore,
	topics []domain.TopicMetadata,
	kafkaRepo repository.KafkaRepository,
	storageRepo repository.StorageRepository,
	metadataRepo repository.MetadataRepository,
	stateRepo repository.StateRepository,
	logger logger.Logger,
) *Coordinator {
	return &Coordinator{
		restore:      restore,
		topics:       topics,
		kafkaRepo:    kafkaRepo,
		storageRepo:  storageRepo,
		metadataRepo: metadataRepo,
		stateRepo:    stateRepo,
		logger:       logger,
	}
}

// Start starts the restore coordinator
func (c *Coordinator) Start(ctx context.Context) error {
	c.logger.Info("Starting restore coordinator", "topics", len(c.topics))

	// Distribute topics across workers
	topicsPerWorker := (len(c.topics) + c.restore.Workers - 1) / c.restore.Workers

	for i := 0; i < c.restore.Workers; i++ {
		start := i * topicsPerWorker
		end := start + topicsPerWorker
		if end > len(c.topics) {
			end = len(c.topics)
		}
		if start >= len(c.topics) {
			break
		}

		workerTopics := c.topics[start:end]
		worker := NewWorker(
			i,
			workerTopics,
			c.restore,
			c.kafkaRepo,
			c.storageRepo,
			c.metadataRepo,
			c.stateRepo,
			c.logger,
		)

		c.wg.Add(1)
		go func(w *Worker) {
			defer c.wg.Done()
			if err := w.Start(ctx); err != nil {
				c.logger.Error("Worker failed", "workerID", w.id, "error", err)
			}
		}(worker)
	}

	c.wg.Wait()
	c.logger.Info("All restore workers completed")
	return nil
}
