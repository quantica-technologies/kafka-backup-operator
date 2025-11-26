package kafka

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
)

// Producer wraps Sarama producer
type Producer struct {
	client   sarama.Client
	producer sarama.SyncProducer
}

func (p *Producer) Produce(ctx context.Context, topic string, message *domain.Message) error {
	msg := p.convertMessage(topic, message)

	_, _, err := p.producer.SendMessage(msg)
	return err
}

func (p *Producer) ProduceBatch(ctx context.Context, topic string, messages []*domain.Message) error {
	for _, msg := range messages {
		if err := p.Produce(ctx, topic, msg); err != nil {
			return err
		}
	}
	return nil
}

func (p *Producer) Flush(ctx context.Context) error {
	// Sarama sync producer flushes automatically
	return nil
}

func (p *Producer) Close() error {
	if p.producer != nil {
		if err := p.producer.Close(); err != nil {
			return err
		}
	}
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

func (p *Producer) convertMessage(topic string, msg *domain.Message) *sarama.ProducerMessage {
	headers := make([]sarama.RecordHeader, len(msg.Headers))
	for i, h := range msg.Headers {
		headers[i] = sarama.RecordHeader{
			Key:   []byte(h.Key),
			Value: h.Value,
		}
	}

	return &sarama.ProducerMessage{
		Topic:     topic,
		Key:       sarama.ByteEncoder(msg.Key),
		Value:     sarama.ByteEncoder(msg.Value),
		Timestamp: msg.Timestamp,
		Headers:   headers,
	}
}
