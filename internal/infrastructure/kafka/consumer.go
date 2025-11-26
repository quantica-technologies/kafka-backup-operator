package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
)

// Consumer wraps Sarama consumer
type Consumer struct {
	client             sarama.Client
	consumer           sarama.Consumer
	partitionConsumers map[string]map[int32]sarama.PartitionConsumer
}

func (c *Consumer) Subscribe(topics []string) error {
	c.partitionConsumers = make(map[string]map[int32]sarama.PartitionConsumer)

	for _, topic := range topics {
		partitions, err := c.consumer.Partitions(topic)
		if err != nil {
			return fmt.Errorf("failed to get partitions for %s: %w", topic, err)
		}

		c.partitionConsumers[topic] = make(map[int32]sarama.PartitionConsumer)

		for _, partition := range partitions {
			pc, err := c.consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
			if err != nil {
				return fmt.Errorf("failed to consume partition %d: %w", partition, err)
			}
			c.partitionConsumers[topic][partition] = pc
		}
	}

	return nil
}

func (c *Consumer) Poll(ctx context.Context) (*domain.Message, error) {
	// Simplified - in production, use multiplexing across partitions
	for _, partitions := range c.partitionConsumers {
		for _, pc := range partitions {
			select {
			case msg := <-pc.Messages():
				return c.convertMessage(msg), nil
			case err := <-pc.Errors():
				return nil, err
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				continue
			}
		}
	}
	return nil, nil
}

func (c *Consumer) Seek(topic string, partition int32, offset int64) error {
	// Implementation depends on partition consumer access
	return nil
}

func (c *Consumer) Commit(ctx context.Context) error {
	// Manual commit logic
	return nil
}

func (c *Consumer) Close() error {
	for _, partitions := range c.partitionConsumers {
		for _, pc := range partitions {
			pc.Close()
		}
	}
	if c.consumer != nil {
		c.consumer.Close()
	}
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *Consumer) convertMessage(msg *sarama.ConsumerMessage) *domain.Message {
	headers := make([]domain.Header, len(msg.Headers))
	for i, h := range msg.Headers {
		headers[i] = domain.Header{
			Key:   string(h.Key),
			Value: h.Value,
		}
	}

	return &domain.Message{
		Offset:    msg.Offset,
		Partition: msg.Partition,
		Key:       msg.Key,
		Value:     msg.Value,
		Timestamp: msg.Timestamp,
		Headers:   headers,
	}
}
