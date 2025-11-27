package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration
type Config struct {
	Kafka         KafkaConfig   `yaml:"kafka"`
	Storage       StorageConfig `yaml:"storage"`
	Workers       int           `yaml:"workers"`
	TopicFilter   string        `yaml:"topic_filter"`
	WorkGroup     WorkGroup     `yaml:"work_group"`
	Compression   bool          `yaml:"compression"`
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
}

// KafkaConfig contains Kafka connection settings
type KafkaConfig struct {
	BootstrapServers string            `yaml:"bootstrap_servers"`
	SecurityProtocol string            `yaml:"security_protocol"`
	SASLMechanism    string            `yaml:"sasl_mechanism"`
	SASLUsername     string            `yaml:"sasl_username"`
	SASLPassword     string            `yaml:"sasl_password"`
	GroupID          string            `yaml:"group_id"`
	Properties       map[string]string `yaml:"properties"`
}

// StorageConfig contains storage backend settings
type StorageConfig struct {
	Type   string `yaml:"type"` // s3, azure, gcs, local
	Path   string `yaml:"path"` // bucket name or local path
	Region string `yaml:"region"`

	// AWS S3 specific
	S3Endpoint        string `yaml:"s3_endpoint"`
	S3AccessKeyID     string `yaml:"s3_access_key_id"`
	S3SecretAccessKey string `yaml:"s3_secret_access_key"`

	// Azure specific
	AzureAccountName string `yaml:"azure_account_name"`
	AzureAccountKey  string `yaml:"azure_account_key"`

	// GCS specific
	GCSCredentialsFile string `yaml:"gcs_credentials_file"`

	// Common settings
	Prefix string `yaml:"prefix"` // prefix for all stored objects
}

// WorkGroup defines worker distribution settings
type WorkGroup struct {
	Workers int `yaml:"workers"`
	Seed    int `yaml:"seed"` // for topic distribution hashing
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Kafka: KafkaConfig{
			BootstrapServers: "localhost:9092",
			SecurityProtocol: "PLAINTEXT",
			GroupID:          "kafka-backup-consumer",
		},
		Storage: StorageConfig{
			Type:   "local",
			Path:   "./backup",
			Prefix: "kafka-backup",
		},
		Workers:       1,
		TopicFilter:   "*",
		Compression:   true,
		BatchSize:     1000,
		FlushInterval: 30 * time.Second,
		WorkGroup: WorkGroup{
			Workers: 1,
			Seed:    0,
		},
	}
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Kafka.BootstrapServers == "" {
		return fmt.Errorf("kafka bootstrap servers must be specified")
	}

	if c.Storage.Type == "" {
		return fmt.Errorf("storage type must be specified")
	}

	if c.Storage.Path == "" {
		return fmt.Errorf("storage path must be specified")
	}

	if c.Workers < 1 {
		return fmt.Errorf("workers must be at least 1")
	}

	if c.BatchSize < 1 {
		return fmt.Errorf("batch size must be at least 1")
	}

	// Validate storage-specific settings
	switch c.Storage.Type {
	case "s3":
		if c.Storage.Region == "" {
			return fmt.Errorf("s3 region must be specified")
		}
	case "azure":
		if c.Storage.AzureAccountName == "" || c.Storage.AzureAccountKey == "" {
			return fmt.Errorf("azure account name and key must be specified")
		}
	case "gcs":
		if c.Storage.GCSCredentialsFile == "" {
			return fmt.Errorf("gcs credentials file must be specified")
		}
	case "local":
		// No additional validation needed
	default:
		return fmt.Errorf("unsupported storage type: %s", c.Storage.Type)
	}

	return nil
}

// Save writes the configuration to a YAML file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
