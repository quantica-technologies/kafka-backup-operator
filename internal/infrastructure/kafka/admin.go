package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/quantica-technologies/kafka-backup/internal/domain"
)

// Admin wraps Sarama cluster admin
type Admin struct {
	client sarama.Client
	admin  sarama.ClusterAdmin
}

func (a *Admin) ListTopics(ctx context.Context) ([]*domain.Topic, error) {
	metadata, err := a.admin.ListTopics()
	if err != nil {
		return nil, err
	}

	topics := make([]*domain.Topic, 0, len(metadata))
	for name, detail := range metadata {
		topics = append(topics, &domain.Topic{
			Name:              name,
			Partitions:        int32(len(detail.Partitions)),
			ReplicationFactor: int16(len(detail.Partitions[0].Replicas)),
		})
	}

	return topics, nil
}

func (a *Admin) DescribeTopic(ctx context.Context, name string) (*domain.Topic, error) {
	metadata, err := a.admin.DescribeTopics([]string{name})
	if err != nil {
		return nil, err
	}

	if len(metadata) == 0 {
		return nil, fmt.Errorf("topic not found: %s", name)
	}

	detail := metadata[0]
	return &domain.Topic{
		Name:              detail.Name,
		Partitions:        int32(len(detail.Partitions)),
		ReplicationFactor: int16(len(detail.Partitions[0].Replicas)),
	}, nil
}

func (a *Admin) CreateTopic(ctx context.Context, topic *domain.Topic) error {
	topicDetail := &sarama.TopicDetail{
		NumPartitions:     topic.Partitions,
		ReplicationFactor: topic.ReplicationFactor,
		ConfigEntries:     topic.Config,
	}

	return a.admin.CreateTopic(topic.Name, topicDetail, false)
}

func (a *Admin) DeleteTopic(ctx context.Context, name string) error {
	return a.admin.DeleteTopic(name)
}

func (a *Admin) GetPartitions(ctx context.Context, topic string) ([]int32, error) {
	return a.client.Partitions(topic)
}

func (a *Admin) GetOffsets(ctx context.Context, topic string, partition int32) (low, high int64, err error) {
	low, err = a.client.GetOffset(topic, partition, sarama.OffsetOldest)
	if err != nil {
		return 0, 0, err
	}

	high, err = a.client.GetOffset(topic, partition, sarama.OffsetNewest)
	if err != nil {
		return 0, 0, err
	}

	return low, high, nil
}

func (a *Admin) Close() error {
	if a.admin != nil {
		if err := a.admin.Close(); err != nil {
			return err
		}
	}
	if a.client != nil {
		return a.client.Close()
	}
	return nil
}
