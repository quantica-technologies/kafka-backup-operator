package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
)

// Admin wraps Sarama cluster admin
type Admin struct {
	client sarama.Client
	admin  sarama.ClusterAdmin
}

func (a *Admin) ListTopics(ctx context.Context) ([]*domain.Topic, error) {
	topicNames, err := a.client.Topics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	topics := make([]*domain.Topic, 0, len(topicNames))
	for _, name := range topicNames {
		// Get topic metadata
		partitions, err := a.client.Partitions(name)
		if err != nil {
			continue // Skip topics we can't access
		}

		if len(partitions) == 0 {
			continue
		}

		// Get replication factor from first partition
		replicas, err := a.client.Replicas(name, partitions[0])
		if err != nil {
			continue
		}

		topics = append(topics, &domain.Topic{
			Name:              name,
			Partitions:        int32(len(partitions)),
			ReplicationFactor: int16(len(replicas)),
		})
	}

	return topics, nil
}

func (a *Admin) DescribeTopic(ctx context.Context, name string) (*domain.Topic, error) {
	// Get partitions for the topic
	partitions, err := a.client.Partitions(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get partitions: %w", err)
	}

	if len(partitions) == 0 {
		return nil, fmt.Errorf("topic has no partitions: %s", name)
	}

	// Get replication factor from first partition
	replicas, err := a.client.Replicas(name, partitions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get replicas: %w", err)
	}

	// Get topic configuration
	configResource := sarama.ConfigResource{
		Type: sarama.TopicResource,
		Name: name,
	}

	configs, err := a.admin.DescribeConfig(configResource)
	if err != nil {
		// Continue without config if describe fails
		return &domain.Topic{
			Name:              name,
			Partitions:        int32(len(partitions)),
			ReplicationFactor: int16(len(replicas)),
			Config:            make(map[string]string),
		}, nil
	}

	// Convert config entries to map
	configMap := make(map[string]string)
	for _, entry := range configs {
		configMap[entry.Name] = entry.Value
	}

	return &domain.Topic{
		Name:              name,
		Partitions:        int32(len(partitions)),
		ReplicationFactor: int16(len(replicas)),
		Config:            configMap,
	}, nil
}

func (a *Admin) CreateTopic(ctx context.Context, topic *domain.Topic) error {
	// Convert config map to Sarama format
	configEntries := make(map[string]*string)
	for k, v := range topic.Config {
		val := v // Create a copy to get pointer
		configEntries[k] = &val
	}

	topicDetail := &sarama.TopicDetail{
		NumPartitions:     topic.Partitions,
		ReplicationFactor: topic.ReplicationFactor,
		ConfigEntries:     configEntries,
	}

	err := a.admin.CreateTopic(topic.Name, topicDetail, false)
	if err != nil {
		// Check if error is because topic already exists
		if topicErr, ok := err.(*sarama.TopicError); ok {
			if topicErr.Err == sarama.ErrTopicAlreadyExists {
				return fmt.Errorf("topic already exists: %s", topic.Name)
			}
		}
		return fmt.Errorf("failed to create topic: %w", err)
	}

	return nil
}

func (a *Admin) DeleteTopic(ctx context.Context, name string) error {
	err := a.admin.DeleteTopic(name)
	if err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}
	return nil
}

func (a *Admin) GetPartitions(ctx context.Context, topic string) ([]int32, error) {
	partitions, err := a.client.Partitions(topic)
	if err != nil {
		return nil, fmt.Errorf("failed to get partitions: %w", err)
	}
	return partitions, nil
}

func (a *Admin) GetOffsets(ctx context.Context, topic string, partition int32) (low, high int64, err error) {
	// Get oldest offset
	low, err = a.client.GetOffset(topic, partition, sarama.OffsetOldest)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get oldest offset: %w", err)
	}

	// Get newest offset
	high, err = a.client.GetOffset(topic, partition, sarama.OffsetNewest)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get newest offset: %w", err)
	}

	return low, high, nil
}

func (a *Admin) Close() error {
	var adminErr, clientErr error

	if a.admin != nil {
		adminErr = a.admin.Close()
	}

	if a.client != nil {
		clientErr = a.client.Close()
	}

	// Return first error encountered
	if adminErr != nil {
		return adminErr
	}
	return clientErr
}
