package repository

import (
	"context"

	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
)

// KafkaRepository defines operations for interacting with Kafka
type KafkaRepository interface {
	// Consumer operations
	CreateConsumer(ctx context.Context, cluster *domain.KafkaCluster, config ConsumerConfig) (Consumer, error)

	// Producer operations
	CreateProducer(ctx context.Context, cluster *domain.KafkaCluster, config ProducerConfig) (Producer, error)

	// Admin operations
	CreateAdmin(ctx context.Context, cluster *domain.KafkaCluster) (Admin, error)

	// Health check
	HealthCheck(ctx context.Context, cluster *domain.KafkaCluster) error
}

// Consumer defines operations for consuming messages
type Consumer interface {
	// Subscribe subscribes to topics
	Subscribe(topics []string) error

	// Poll polls for messages
	Poll(ctx context.Context) (*domain.Message, error)

	// Seek seeks to a specific offset
	Seek(topic string, partition int32, offset int64) error

	// Commit commits offsets
	Commit(ctx context.Context) error

	// Close closes the consumer
	Close() error
}

// Producer defines operations for producing messages
type Producer interface {
	// Produce sends a message
	Produce(ctx context.Context, topic string, message *domain.Message) error

	// ProduceBatch sends multiple messages
	ProduceBatch(ctx context.Context, topic string, messages []*domain.Message) error

	// Flush flushes pending messages
	Flush(ctx context.Context) error

	// Close closes the producer
	Close() error
}

// Admin defines admin operations
type Admin interface {
	// ListTopics lists all topics
	ListTopics(ctx context.Context) ([]*domain.Topic, error)

	// DescribeTopic gets topic details
	DescribeTopic(ctx context.Context, name string) (*domain.Topic, error)

	// CreateTopic creates a new topic
	CreateTopic(ctx context.Context, topic *domain.Topic) error

	// DeleteTopic deletes a topic
	DeleteTopic(ctx context.Context, name string) error

	// GetPartitions gets partitions for a topic
	GetPartitions(ctx context.Context, topic string) ([]int32, error)

	// GetOffsets gets partition offsets
	GetOffsets(ctx context.Context, topic string, partition int32) (low, high int64, err error)

	// Close closes the admin client
	Close() error
}

// ConsumerConfig holds consumer configuration
type ConsumerConfig struct {
	GroupID        string
	AutoCommit     bool
	StartOffset    string
	SessionTimeout int
	MaxPollRecords int
}

// ProducerConfig holds producer configuration
type ProducerConfig struct {
	RequiredAcks int
	MaxRetries   int
	Compression  string
	Idempotent   bool
}
