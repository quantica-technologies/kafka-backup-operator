package restore

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
	"github.com/quantica-technologies/kafka-backup-operator/internal/repository"
	"github.com/quantica-technologies/kafka-backup-operator/pkg/logger"
)

// Worker handles restore operations for assigned topics
type Worker struct {
	id           int
	topics       []domain.TopicMetadata
	restore      *domain.Restore
	kafkaRepo    repository.KafkaRepository
	storageRepo  repository.StorageRepository
	metadataRepo repository.MetadataRepository
	stateRepo    repository.StateRepository
	logger       logger.Logger
	producer     repository.Producer
	admin        repository.Admin
}

// NewWorker creates a new restore worker
func NewWorker(
	id int,
	topics []domain.TopicMetadata,
	restore *domain.Restore,
	kafkaRepo repository.KafkaRepository,
	storageRepo repository.StorageRepository,
	metadataRepo repository.MetadataRepository,
	stateRepo repository.StateRepository,
	logger logger.Logger,
) *Worker {
	return &Worker{
		id:           id,
		topics:       topics,
		restore:      restore,
		kafkaRepo:    kafkaRepo,
		storageRepo:  storageRepo,
		metadataRepo: metadataRepo,
		stateRepo:    stateRepo,
		logger:       logger.WithFields(map[string]interface{}{"workerID": id}),
	}
}

// Start begins the restore process for this worker
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("Starting restore worker", "topics", len(w.topics))

	// Create Kafka producer
	producerConfig := repository.ProducerConfig{
		RequiredAcks: -1, // Wait for all replicas
		MaxRetries:   5,
		Idempotent:   true,
	}

	producer, err := w.kafkaRepo.CreateProducer(ctx, w.restore.TargetCluster, producerConfig)
	if err != nil {
		return fmt.Errorf("failed to create producer: %w", err)
	}
	defer producer.Close()
	w.producer = producer

	// Create admin client for metadata
	admin, err := w.kafkaRepo.CreateAdmin(ctx, w.restore.TargetCluster)
	if err != nil {
		return fmt.Errorf("failed to create admin client: %w", err)
	}
	defer admin.Close()
	w.admin = admin

	// Process each topic
	for _, topicMeta := range w.topics {
		select {
		case <-ctx.Done():
			w.logger.Info("Worker context cancelled")
			return ctx.Err()
		default:
			if err := w.restoreTopic(ctx, topicMeta); err != nil {
				w.logger.Error("Failed to restore topic", "topic", topicMeta.Name, "error", err)
				w.recordError(ctx, topicMeta.Name, err)
				// Continue with other topics
				continue
			}
		}
	}

	w.logger.Info("Worker completed successfully")
	return nil
}

// restoreTopic restores a single topic
func (w *Worker) restoreTopic(ctx context.Context, topicMeta domain.TopicMetadata) error {
	originalName := topicMeta.Name
	targetName := w.restore.GetMappedTopicName(originalName)

	w.logger.Info("Restoring topic",
		"originalName", originalName,
		"targetName", targetName,
		"partitions", topicMeta.Partitions)

	// Check if topic should be restored based on selectors
	if !w.shouldRestoreTopic(originalName) {
		w.logger.Info("Skipping topic (not selected)", "topic", originalName)
		return nil
	}

	// Create topic if needed
	if w.restore.CreateTopics {
		if err := w.ensureTopicExists(ctx, targetName, topicMeta); err != nil {
			w.logger.Warn("Failed to create topic", "topic", targetName, "error", err)
			// Continue anyway - topic might already exist
		}
	}

	// Get segments metadata for the topic
	segments, err := w.getTopicSegments(ctx, originalName, topicMeta.Partitions)
	if err != nil {
		return fmt.Errorf("failed to get topic segments: %w", err)
	}

	w.logger.Info("Found segments to restore",
		"topic", originalName,
		"totalSegments", len(segments))

	// Restore each partition
	for partition := int32(0); partition < topicMeta.Partitions; partition++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Check partition filter
			if !w.restore.ShouldRestorePartition(originalName, partition) {
				w.logger.Debug("Skipping partition (filtered)",
					"topic", originalName,
					"partition", partition)
				continue
			}

			partitionSegments := w.filterSegmentsByPartition(segments, partition)
			if err := w.restorePartition(ctx, originalName, targetName, partition, partitionSegments); err != nil {
				w.logger.Error("Failed to restore partition",
					"topic", originalName,
					"partition", partition,
					"error", err)
				w.recordError(ctx, fmt.Sprintf("%s-partition-%d", originalName, partition), err)
				// Continue with other partitions
			}
		}
	}

	w.logger.Info("Topic restored successfully", "topic", originalName)
	w.updateTopicProgress(ctx, originalName, "completed")
	return nil
}

// restorePartition restores a single partition
func (w *Worker) restorePartition(
	ctx context.Context,
	originalTopic string,
	targetTopic string,
	partition int32,
	segments []string,
) error {
	w.logger.Info("Restoring partition",
		"originalTopic", originalTopic,
		"targetTopic", targetTopic,
		"partition", partition,
		"segments", len(segments))

	totalMessages := int64(0)
	totalBytes := int64(0)

	// Process segments in order
	for _, segmentKey := range segments {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			messages, bytesRead, err := w.readSegment(ctx, segmentKey)
			if err != nil {
				w.logger.Error("Failed to read segment",
					"segment", segmentKey,
					"error", err)
				continue
			}

			if len(messages) == 0 {
				w.logger.Warn("Empty segment", "segment", segmentKey)
				continue
			}

			// Produce messages to Kafka
			producedCount, err := w.produceMessages(ctx, targetTopic, partition, messages)
			if err != nil {
				return fmt.Errorf("failed to produce messages: %w", err)
			}

			totalMessages += int64(producedCount)
			totalBytes += bytesRead

			w.logger.Debug("Segment restored",
				"segment", segmentKey,
				"messages", producedCount,
				"bytes", bytesRead)

			// Update progress
			w.updateWorkerStatus(ctx, int64(producedCount), bytesRead)
		}
	}

	w.logger.Info("Partition restored successfully",
		"topic", originalTopic,
		"partition", partition,
		"totalMessages", totalMessages,
		"totalBytes", totalBytes)

	return nil
}

// readSegment reads and deserializes a segment from storage
func (w *Worker) readSegment(ctx context.Context, segmentKey string) ([]*domain.Message, int64, error) {
	// Get segment from storage
	reader, metadata, err := w.storageRepo.Get(ctx, segmentKey)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get segment: %w", err)
	}
	defer reader.Close()

	bytesRead := metadata.Size

	// Check if compressed
	var dataReader io.Reader = reader
	if strings.HasSuffix(segmentKey, ".gz") {
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to decompress segment: %w", err)
		}
		defer gzReader.Close()
		dataReader = gzReader
	}

	// Read all data
	data, err := io.ReadAll(dataReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read segment data: %w", err)
	}

	// Deserialize messages
	var messages []*domain.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	return messages, bytesRead, nil
}

// produceMessages produces messages to Kafka
func (w *Worker) produceMessages(
	ctx context.Context,
	topic string,
	partition int32,
	messages []*domain.Message,
) (int, error) {
	producedCount := 0

	for _, msg := range messages {
		select {
		case <-ctx.Done():
			return producedCount, ctx.Err()
		default:
			// Check if we should overwrite or skip existing messages
			if !w.restore.OverwriteExisting {
				// In production, you might want to check if message exists
				// For now, we'll always produce
			}

			// Set partition explicitly
			msg.Partition = partition

			if err := w.producer.Produce(ctx, topic, msg); err != nil {
				w.logger.Warn("Failed to produce message",
					"topic", topic,
					"partition", partition,
					"offset", msg.Offset,
					"error", err)
				// Continue with next message
				continue
			}

			producedCount++
		}
	}

	// Flush producer
	if err := w.producer.Flush(ctx); err != nil {
		return producedCount, fmt.Errorf("failed to flush producer: %w", err)
	}

	return producedCount, nil
}

// getTopicSegments gets all segment keys for a topic
func (w *Worker) getTopicSegments(ctx context.Context, topicName string, partitionCount int32) ([]string, error) {
	prefix := filepath.Join(
		w.restore.SourceStorage.Prefix,
		"backups",
		w.restore.BackupID,
		"topics",
		topicName,
	)

	objects, err := w.storageRepo.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list segments: %w", err)
	}

	// Filter for segment files
	var segments []string
	for _, obj := range objects {
		if strings.Contains(obj.Key, "segment-") {
			segments = append(segments, obj.Key)
		}
	}

	return segments, nil
}

// filterSegmentsByPartition filters segments for a specific partition
func (w *Worker) filterSegmentsByPartition(segments []string, partition int32) []string {
	partitionPrefix := fmt.Sprintf("partition-%d", partition)
	var filtered []string

	for _, segment := range segments {
		if strings.Contains(segment, partitionPrefix) {
			filtered = append(filtered, segment)
		}
	}

	return filtered
}

// ensureTopicExists creates a topic if it doesn't exist
func (w *Worker) ensureTopicExists(ctx context.Context, topicName string, topicMeta domain.TopicMetadata) error {
	// Check if topic exists
	_, err := w.admin.DescribeTopic(ctx, topicName)
	if err == nil {
		w.logger.Debug("Topic already exists", "topic", topicName)
		return nil
	}

	// Create topic
	topic := &domain.Topic{
		Name:              topicName,
		Partitions:        topicMeta.Partitions,
		ReplicationFactor: topicMeta.ReplicationFactor,
		Config:            topicMeta.Config,
	}

	if err := w.admin.CreateTopic(ctx, topic); err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	w.logger.Info("Topic created", "topic", topicName, "partitions", topic.Partitions)
	return nil
}

// shouldRestoreTopic checks if a topic should be restored based on selectors
func (w *Worker) shouldRestoreTopic(topicName string) bool {
	if w.restore.TopicSelectors == nil || len(w.restore.TopicSelectors) == 0 {
		return true
	}

	for _, selector := range w.restore.TopicSelectors {
		if selector.Matches(topicName) {
			return true
		}
	}

	return false
}

// updateWorkerStatus updates the worker's status in the restore state
func (w *Worker) updateWorkerStatus(ctx context.Context, messages, bytes int64) {
	// Get current restore state
	restore, err := w.stateRepo.GetRestoreState(ctx, w.restore.ID)
	if err != nil {
		w.logger.Warn("Failed to get restore state", "error", err)
		return
	}

	// Update overall status
	restore.Status.MessagesRestored += messages
	restore.Status.BytesRestored += bytes

	// Save updated state
	if err := w.stateRepo.UpdateRestoreState(ctx, w.restore.ID, &restore.Status); err != nil {
		w.logger.Warn("Failed to update restore state", "error", err)
	}
}

// updateTopicProgress updates progress for a specific topic
func (w *Worker) updateTopicProgress(ctx context.Context, topicName string, status string) {
	restore, err := w.stateRepo.GetRestoreState(ctx, w.restore.ID)
	if err != nil {
		w.logger.Warn("Failed to get restore state", "error", err)
		return
	}

	// Find and update topic progress
	for i := range restore.Status.Progress {
		if restore.Status.Progress[i].TopicName == topicName {
			restore.Status.Progress[i].Status = status
			if status == "completed" {
				restore.Status.Progress[i].PartitionsCompleted = restore.Status.Progress[i].PartitionsTotal
			}
			break
		}
	}

	// Save updated state
	if err := w.stateRepo.UpdateRestoreState(ctx, w.restore.ID, &restore.Status); err != nil {
		w.logger.Warn("Failed to update restore state", "error", err)
	}
}

// recordError records an error in the restore status
func (w *Worker) recordError(ctx context.Context, location string, err error) {
	restore, getErr := w.stateRepo.GetRestoreState(ctx, w.restore.ID)
	if getErr != nil {
		w.logger.Warn("Failed to get restore state", "error", getErr)
		return
	}

	errorMsg := fmt.Sprintf("Worker %d - %s: %v", w.id, location, err)
	restore.Status.Errors = append(restore.Status.Errors, errorMsg)

	if updateErr := w.stateRepo.UpdateRestoreState(ctx, w.restore.ID, &restore.Status); updateErr != nil {
		w.logger.Warn("Failed to update restore state with error", "error", updateErr)
	}
}

// GetProgress returns the current progress of the worker
func (w *Worker) GetProgress() WorkerProgress {
	return WorkerProgress{
		WorkerID:         w.id,
		TopicsAssigned:   len(w.topics),
		TopicsCompleted:  0, // Updated during execution
		MessagesRestored: 0, // Updated during execution
		Status:           "running",
	}
}

// WorkerProgress represents the progress of a restore worker
type WorkerProgress struct {
	WorkerID         int
	TopicsAssigned   int
	TopicsCompleted  int
	MessagesRestored int64
	Status           string
}
