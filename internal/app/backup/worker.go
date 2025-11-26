package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
	"github.com/quantica-technologies/kafka-backup-operator/internal/repository"
	"github.com/quantica-technologies/kafka-backup-operator/pkg/logger"
)

// Worker handles backup operations for assigned topics
type Worker struct {
	id           int
	topics       []*domain.Topic
	backup       *domain.Backup
	kafkaRepo    repository.KafkaRepository
	storageRepo  repository.StorageRepository
	metadataRepo repository.MetadataRepository
	stateRepo    repository.StateRepository
	logger       logger.Logger
	consumer     repository.Consumer
	admin        repository.Admin
}

// NewWorker creates a new backup worker
func NewWorker(
	id int,
	topics []*domain.Topic,
	backup *domain.Backup,
	kafkaRepo repository.KafkaRepository,
	storageRepo repository.StorageRepository,
	metadataRepo repository.MetadataRepository,
	stateRepo repository.StateRepository,
	logger logger.Logger,
) *Worker {
	return &Worker{
		id:           id,
		topics:       topics,
		backup:       backup,
		kafkaRepo:    kafkaRepo,
		storageRepo:  storageRepo,
		metadataRepo: metadataRepo,
		stateRepo:    stateRepo,
		logger:       logger.WithFields(map[string]interface{}{"workerID": id}),
	}
}

// Start begins the backup process for this worker
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("Starting backup worker", "topics", len(w.topics))

	// Create Kafka consumer
	consumerConfig := repository.ConsumerConfig{
		GroupID:     fmt.Sprintf("%s-worker-%d", w.backup.ID, w.id),
		AutoCommit:  false,
		StartOffset: "earliest",
	}

	consumer, err := w.kafkaRepo.CreateConsumer(ctx, w.backup.SourceCluster, consumerConfig)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}
	defer consumer.Close()
	w.consumer = consumer

	// Create admin client for metadata
	admin, err := w.kafkaRepo.CreateAdmin(ctx, w.backup.SourceCluster)
	if err != nil {
		return fmt.Errorf("failed to create admin client: %w", err)
	}
	defer admin.Close()
	w.admin = admin

	// Process each topic
	for _, topic := range w.topics {
		select {
		case <-ctx.Done():
			w.logger.Info("Worker context cancelled")
			return ctx.Err()
		default:
			if err := w.backupTopic(ctx, topic); err != nil {
				w.logger.Error("Failed to backup topic", "topic", topic.Name, "error", err)
				// Continue with other topics
			}
		}
	}

	w.logger.Info("Worker completed successfully")
	return nil
}

// backupTopic backs up a single topic
func (w *Worker) backupTopic(ctx context.Context, topic *domain.Topic) error {
	w.logger.Info("Backing up topic", "topic", topic.Name)

	// Get all partitions for the topic
	partitions, err := w.admin.GetPartitions(ctx, topic.Name)
	if err != nil {
		return fmt.Errorf("failed to get partitions: %w", err)
	}

	// Save topic metadata
	if err := w.saveTopicMetadata(ctx, topic); err != nil {
		w.logger.Warn("Failed to save topic metadata", "error", err)
	}

	// Backup each partition
	for _, partition := range partitions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := w.backupPartition(ctx, topic.Name, partition); err != nil {
				w.logger.Error("Failed to backup partition",
					"topic", topic.Name,
					"partition", partition,
					"error", err)
				// Continue with other partitions
			}
		}
	}

	return nil
}

// backupPartition backs up a single partition
func (w *Worker) backupPartition(ctx context.Context, topicName string, partition int32) error {
	w.logger.Info("Backing up partition", "topic", topicName, "partition", partition)

	// Get partition offsets
	lowOffset, highOffset, err := w.admin.GetOffsets(ctx, topicName, partition)
	if err != nil {
		return fmt.Errorf("failed to get offsets: %w", err)
	}

	w.logger.Debug("Partition offsets",
		"low", lowOffset,
		"high", highOffset,
		"messages", highOffset-lowOffset)

	// Seek to beginning
	if err := w.consumer.Seek(topicName, partition, lowOffset); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	batch := make([]*domain.Message, 0, w.backup.BatchSize)
	segmentNum := 0
	lastFlush := time.Now()
	totalMessages := int64(0)

	// Consume messages
	for {
		select {
		case <-ctx.Done():
			// Flush remaining messages before exit
			if len(batch) > 0 {
				w.flushBatch(ctx, topicName, partition, segmentNum, lowOffset+totalMessages, batch)
			}
			return ctx.Err()
		default:
			// Poll for message
			message, err := w.consumer.Poll(ctx)
			if err != nil {
				// Handle different error types
				continue
			}

			if message == nil {
				// No more messages
				if len(batch) > 0 {
					w.flushBatch(ctx, topicName, partition, segmentNum, lowOffset+totalMessages, batch)
				}
				return nil
			}

			batch = append(batch, message)
			totalMessages++

			// Flush batch if size reached
			if len(batch) >= w.backup.BatchSize {
				startOffset := lowOffset + totalMessages - int64(len(batch))
				if err := w.flushBatch(ctx, topicName, partition, segmentNum, startOffset, batch); err != nil {
					return err
				}
				batch = batch[:0]
				segmentNum++
				lastFlush = time.Now()
			}

			// Flush batch if interval reached
			if time.Since(lastFlush) >= w.backup.FlushInterval && len(batch) > 0 {
				startOffset := lowOffset + totalMessages - int64(len(batch))
				if err := w.flushBatch(ctx, topicName, partition, segmentNum, startOffset, batch); err != nil {
					return err
				}
				batch = batch[:0]
				segmentNum++
				lastFlush = time.Now()
			}
		}
	}
}

// flushBatch writes a batch of messages to storage
func (w *Worker) flushBatch(
	ctx context.Context,
	topic string,
	partition int32,
	segment int,
	startOffset int64,
	messages []*domain.Message,
) error {
	w.logger.Debug("Flushing batch",
		"topic", topic,
		"partition", partition,
		"segment", segment,
		"messages", len(messages))

	// Serialize messages to JSON
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	// Compress if enabled
	if w.backup.Compression {
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		if _, err := gzWriter.Write(data); err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		gzWriter.Close()
		data = buf.Bytes()
	}

	// Build storage key
	key := w.buildStorageKey(topic, partition, segment)

	// Prepare metadata
	metadata := &repository.ObjectMetadata{
		Key:         key,
		Size:        int64(len(data)),
		ContentType: "application/json",
		CustomMetadata: map[string]string{
			"backup_id":  w.backup.ID,
			"topic":      topic,
			"partition":  fmt.Sprintf("%d", partition),
			"segment":    fmt.Sprintf("%d", segment),
			"messages":   fmt.Sprintf("%d", len(messages)),
			"compressed": fmt.Sprintf("%t", w.backup.Compression),
		},
	}

	// Write to storage
	if err := w.storageRepo.Put(ctx, key, bytes.NewReader(data), metadata); err != nil {
		return fmt.Errorf("failed to write to storage: %w", err)
	}

	// Save segment metadata
	endOffset := startOffset + int64(len(messages)) - 1
	segmentMeta := &domain.SegmentMetadata{
		PartitionID:   partition,
		SegmentNumber: segment,
		StartOffset:   startOffset,
		EndOffset:     endOffset,
		MessageCount:  int64(len(messages)),
		SizeBytes:     int64(len(data)),
		StorageKey:    key,
	}

	if err := w.metadataRepo.SaveSegmentMetadata(ctx, w.backup.ID, segmentMeta); err != nil {
		w.logger.Warn("Failed to save segment metadata", "error", err)
	}

	// Update worker status
	w.updateWorkerStatus(ctx, int64(len(messages)), int64(len(data)))

	w.logger.Debug("Batch flushed successfully", "key", key, "size", len(data))
	return nil
}

// buildStorageKey builds the storage key for a segment
func (w *Worker) buildStorageKey(topic string, partition int32, segment int) string {
	key := filepath.Join(
		w.backup.TargetStorage.Prefix,
		"backups",
		w.backup.ID,
		"topics",
		topic,
		fmt.Sprintf("partition-%d", partition),
		fmt.Sprintf("segment-%06d.json", segment),
	)

	if w.backup.Compression {
		key += ".gz"
	}

	return key
}

// saveTopicMetadata saves topic metadata to storage
func (w *Worker) saveTopicMetadata(ctx context.Context, topic *domain.Topic) error {
	topicMeta := &domain.TopicMetadata{
		Name:              topic.Name,
		Partitions:        topic.Partitions,
		ReplicationFactor: topic.ReplicationFactor,
		Config:            topic.Config,
	}

	data, err := json.Marshal(topicMeta)
	if err != nil {
		return err
	}

	key := filepath.Join(
		w.backup.TargetStorage.Prefix,
		"backups",
		w.backup.ID,
		"metadata",
		"topics",
		fmt.Sprintf("%s.json", topic.Name),
	)

	metadata := &repository.ObjectMetadata{
		Key:         key,
		Size:        int64(len(data)),
		ContentType: "application/json",
	}

	return w.storageRepo.Put(ctx, key, bytes.NewReader(data), metadata)
}

// updateWorkerStatus updates the worker's status
func (w *Worker) updateWorkerStatus(ctx context.Context, messages, bytes int64) {
	// Get current backup state
	backup, err := w.stateRepo.GetBackupState(ctx, w.backup.ID)
	if err != nil {
		w.logger.Warn("Failed to get backup state", "error", err)
		return
	}

	// Update worker status
	if w.id < len(backup.Status.Workers) {
		backup.Status.Workers[w.id].MessagesProcessed += messages
		backup.Status.Workers[w.id].LastHeartbeat = time.Now()
	}

	// Update overall status
	backup.Status.MessagesProcessed += messages
	backup.Status.BytesProcessed += bytes
	now := time.Now()
	backup.Status.LastBackupTime = &now

	// Save updated state
	if err := w.stateRepo.UpdateBackupState(ctx, w.backup.ID, &backup.Status); err != nil {
		w.logger.Warn("Failed to update backup state", "error", err)
	}
}
