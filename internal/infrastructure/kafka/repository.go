package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
	"github.com/quantica-technologies/kafka-backup-operator/internal/repository"
)

// Repository implements KafkaRepository using Sarama
type Repository struct {
	// Connection pool or factory
}

// NewRepository creates a new Kafka repository
func NewRepository() repository.KafkaRepository {
	return &Repository{}
}

// CreateConsumer creates a Kafka consumer
func (r *Repository) CreateConsumer(
	ctx context.Context,
	cluster *domain.KafkaCluster,
	config repository.ConsumerConfig,
) (repository.Consumer, error) {
	saramaConfig := r.buildSaramaConfig(cluster)
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	saramaConfig.Consumer.Return.Errors = true

	client, err := sarama.NewClient([]string{cluster.BootstrapServers}, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	return &Consumer{
		client:   client,
		consumer: consumer,
	}, nil
}

// CreateProducer creates a Kafka producer
func (r *Repository) CreateProducer(
	ctx context.Context,
	cluster *domain.KafkaCluster,
	config repository.ProducerConfig,
) (repository.Producer, error) {
	saramaConfig := r.buildSaramaConfig(cluster)
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Idempotent = config.Idempotent

	client, err := sarama.NewClient([]string{cluster.BootstrapServers}, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	return &Producer{
		client:   client,
		producer: producer,
	}, nil
}

// CreateAdmin creates a Kafka admin client
func (r *Repository) CreateAdmin(
	ctx context.Context,
	cluster *domain.KafkaCluster,
) (repository.Admin, error) {
	saramaConfig := r.buildSaramaConfig(cluster)

	client, err := sarama.NewClient([]string{cluster.BootstrapServers}, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	admin, err := sarama.NewClusterAdminFromClient(client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create admin: %w", err)
	}

	return &Admin{
		client: client,
		admin:  admin,
	}, nil
}

// HealthCheck checks Kafka cluster connectivity
func (r *Repository) HealthCheck(ctx context.Context, cluster *domain.KafkaCluster) error {
	saramaConfig := r.buildSaramaConfig(cluster)

	client, err := sarama.NewClient([]string{cluster.BootstrapServers}, saramaConfig)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	return nil
}

func (r *Repository) buildSaramaConfig(cluster *domain.KafkaCluster) *sarama.Config {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0

	// Security configuration
	if cluster.SecurityConfig.Protocol != domain.SecurityProtocolPlaintext {
		config.Net.SASL.Enable = true
		config.Net.SASL.Mechanism = sarama.SASLMechanism(cluster.SecurityConfig.SASLMechanism)
		config.Net.SASL.User = cluster.SecurityConfig.Username
		config.Net.SASL.Password = cluster.SecurityConfig.Password

		if cluster.SecurityConfig.TLSConfig != nil && cluster.SecurityConfig.TLSConfig.Enabled {
			config.Net.TLS.Enable = true
			// Configure TLS
		}
	}

	return config
}
