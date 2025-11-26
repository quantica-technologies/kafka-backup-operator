package usecase

import (
	"context"

	"github.com/quantica-technologies/kafka-backup/internal/domain"
)

// TopicUseCase defines operations for topic discovery and management
type TopicUseCase interface {
	// DiscoverTopics discovers all topics in a cluster
	DiscoverTopics(ctx context.Context, cluster *domain.KafkaCluster) ([]*domain.Topic, error)

	// GetTopic gets details of a specific topic
	GetTopic(ctx context.Context, cluster *domain.KafkaCluster, topicName string) (*domain.Topic, error)

	// CreateTopic creates a new topic
	CreateTopic(ctx context.Context, cluster *domain.KafkaCluster, topic *domain.Topic) error

	// FilterTopics filters topics based on selectors
	FilterTopics(topics []*domain.Topic, selectors []domain.TopicSelector) []*domain.Topic
}
