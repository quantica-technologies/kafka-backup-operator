package config

import (
	"fmt"
	"os"
	"time"

	"github.com/quantica-technologies/kafka-backup-operator/internal/domain"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Kafka     KafkaConfig   `yaml:"kafka"`
	Storage   StorageConfig `yaml:"storage"`
	Backup    BackupConfig  `yaml:"backup"`
	Restore   RestoreConfig `yaml:"restore"`
	LogLevel  string        `yaml:"log_level"`
	LogFormat string        `yaml:"log_format"`
}

// KafkaConfig holds Kafka connection settings
type KafkaConfig struct {
	BootstrapServers string            `yaml:"bootstrap_servers"`
	SecurityProtocol string            `yaml:"security_protocol"`
	SASL             SASLConfig        `yaml:"sasl"`
	TLS              TLSConfig         `yaml:"tls"`
	Properties       map[string]string `yaml:"properties"`
}

// SASLConfig holds SASL authentication settings
type SASLConfig struct {
	Mechanism string `yaml:"mechanism"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
}

// TLSConfig holds TLS settings
type TLSConfig struct {
	Enabled            bool   `yaml:"enabled"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CAFile             string `yaml:"ca_file"`
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
}

// StorageConfig holds storage backend settings
type StorageConfig struct {
	Type   string      `yaml:"type"` // s3, azure, gcs, local
	Path   string      `yaml:"path"` // bucket name or local path
	Region string      `yaml:"region"`
	Prefix string      `yaml:"prefix"`
	S3     S3Config    `yaml:"s3"`
	Azure  AzureConfig `yaml:"azure"`
	GCS    GCSConfig   `yaml:"gcs"`
}

// S3Config holds AWS S3 specific settings
type S3Config struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	UsePathStyle    bool   `yaml:"use_path_style"`
}

// AzureConfig holds Azure Blob Storage settings
type AzureConfig struct {
	AccountName string `yaml:"account_name"`
	AccountKey  string `yaml:"account_key"`
}

// GCSConfig holds Google Cloud Storage settings
type GCSConfig struct {
	CredentialsFile string `yaml:"credentials_file"`
	ProjectID       string `yaml:"project_id"`
}

// BackupConfig holds backup-specific settings
type BackupConfig struct {
	ID            string        `yaml:"id"`
	Name          string        `yaml:"name"`
	Workers       int           `yaml:"workers"`
	TopicFilter   string        `yaml:"topic_filter"`
	Compression   bool          `yaml:"compression"`
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
	WorkGroup     WorkGroup     `yaml:"work_group"`
}

// WorkGroup defines worker distribution settings
type WorkGroup struct {
	Workers int `yaml:"workers"`
	Seed    int `yaml:"seed"`
}

// RestoreConfig holds restore-specific settings
type RestoreConfig struct {
	ID                string            `yaml:"id"`
	Name              string            `yaml:"name"`
	BackupID          string            `yaml:"backup_id"`
	Workers           int               `yaml:"workers"`
	TopicFilter       string            `yaml:"topic_filter"`
	TopicMapping      map[string]string `yaml:"topic_mapping"`
	CreateTopics      bool              `yaml:"create_topics"`
	OverwriteExisting bool              `yaml:"overwrite_existing"`
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override from environment variables
	cfg.overrideFromEnv()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Kafka: KafkaConfig{
			BootstrapServers: "localhost:9092",
			SecurityProtocol: "PLAINTEXT",
			Properties:       make(map[string]string),
		},
		Storage: StorageConfig{
			Type:   "local",
			Path:   "./backup",
			Prefix: "kafka-backup",
		},
		Backup: BackupConfig{
			Workers:       1,
			TopicFilter:   "*",
			Compression:   true,
			BatchSize:     1000,
			FlushInterval: 30 * time.Second,
			WorkGroup: WorkGroup{
				Workers: 1,
				Seed:    0,
			},
		},
		Restore: RestoreConfig{
			Workers:      1,
			CreateTopics: true,
		},
		LogLevel:  "info",
		LogFormat: "json",
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate Kafka config
	if c.Kafka.BootstrapServers == "" {
		return fmt.Errorf("kafka bootstrap servers must be specified")
	}

	// Validate storage config
	if c.Storage.Type == "" {
		return fmt.Errorf("storage type must be specified")
	}

	if c.Storage.Path == "" {
		return fmt.Errorf("storage path must be specified")
	}

	switch c.Storage.Type {
	case "s3":
		if c.Storage.Region == "" {
			return fmt.Errorf("s3 region must be specified")
		}
	case "azure":
		if c.Storage.Azure.AccountName == "" {
			return fmt.Errorf("azure account name must be specified")
		}
	case "gcs":
		if c.Storage.GCS.CredentialsFile == "" && c.Storage.GCS.ProjectID == "" {
			return fmt.Errorf("gcs credentials or project ID must be specified")
		}
	case "local":
		// No additional validation needed
	default:
		return fmt.Errorf("unsupported storage type: %s", c.Storage.Type)
	}

	// Validate backup config
	if c.Backup.Workers < 1 {
		return fmt.Errorf("backup workers must be at least 1")
	}

	if c.Backup.BatchSize < 1 {
		return fmt.Errorf("batch size must be at least 1")
	}

	// Validate restore config
	if c.Restore.Workers < 1 {
		return fmt.Errorf("restore workers must be at least 1")
	}

	return nil
}

// overrideFromEnv overrides configuration from environment variables
func (c *Config) overrideFromEnv() {
	// Kafka overrides
	if val := os.Getenv("KAFKA_BOOTSTRAP_SERVERS"); val != "" {
		c.Kafka.BootstrapServers = val
	}
	if val := os.Getenv("KAFKA_SECURITY_PROTOCOL"); val != "" {
		c.Kafka.SecurityProtocol = val
	}
	if val := os.Getenv("KAFKA_SASL_USERNAME"); val != "" {
		c.Kafka.SASL.Username = val
	}
	if val := os.Getenv("KAFKA_SASL_PASSWORD"); val != "" {
		c.Kafka.SASL.Password = val
	}

	// Storage overrides
	if val := os.Getenv("STORAGE_TYPE"); val != "" {
		c.Storage.Type = val
	}
	if val := os.Getenv("STORAGE_PATH"); val != "" {
		c.Storage.Path = val
	}
	if val := os.Getenv("STORAGE_REGION"); val != "" {
		c.Storage.Region = val
	}

	// S3 overrides
	if val := os.Getenv("AWS_ACCESS_KEY_ID"); val != "" {
		c.Storage.S3.AccessKeyID = val
	}
	if val := os.Getenv("AWS_SECRET_ACCESS_KEY"); val != "" {
		c.Storage.S3.SecretAccessKey = val
	}

	// Azure overrides
	if val := os.Getenv("AZURE_STORAGE_ACCOUNT"); val != "" {
		c.Storage.Azure.AccountName = val
	}
	if val := os.Getenv("AZURE_STORAGE_KEY"); val != "" {
		c.Storage.Azure.AccountKey = val
	}

	// Log level override
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		c.LogLevel = val
	}
}

// ToBackupDomain converts config to domain Backup entity
func (c *Config) ToBackupDomain() *domain.Backup {
	return &domain.Backup{
		ID:   c.Backup.ID,
		Name: c.Backup.Name,
		SourceCluster: &domain.KafkaCluster{
			ID:               "default",
			BootstrapServers: c.Kafka.BootstrapServers,
			SecurityConfig:   c.toSecurityConfig(),
			Properties:       c.Kafka.Properties,
		},
		TargetStorage: &domain.Storage{
			Type:   domain.StorageType(c.Storage.Type),
			Path:   c.Storage.Path,
			Region: c.Storage.Region,
			Prefix: c.Storage.Prefix,
			Config: c.toStorageConfig(),
		},
		TopicSelectors: []domain.TopicSelector{
			{
				Type:    domain.SelectorTypeGlob,
				Pattern: c.Backup.TopicFilter,
			},
		},
		WorkerConfig: domain.WorkerConfig{
			Workers: c.Backup.Workers,
			Seed:    c.Backup.WorkGroup.Seed,
		},
		Compression:   c.Backup.Compression,
		BatchSize:     c.Backup.BatchSize,
		FlushInterval: c.Backup.FlushInterval,
	}
}

// ToRestoreDomain converts config to domain Restore entity
func (c *Config) ToRestoreDomain(backupID string) *domain.Restore {
	restore := &domain.Restore{
		ID:       c.Restore.ID,
		Name:     c.Restore.Name,
		BackupID: backupID,
		SourceStorage: &domain.Storage{
			Type:   domain.StorageType(c.Storage.Type),
			Path:   c.Storage.Path,
			Region: c.Storage.Region,
			Prefix: c.Storage.Prefix,
			Config: c.toStorageConfig(),
		},
		TargetCluster: &domain.KafkaCluster{
			ID:               "default",
			BootstrapServers: c.Kafka.BootstrapServers,
			SecurityConfig:   c.toSecurityConfig(),
			Properties:       c.Kafka.Properties,
		},
		Workers:           c.Restore.Workers,
		CreateTopics:      c.Restore.CreateTopics,
		OverwriteExisting: c.Restore.OverwriteExisting,
		TopicMapping:      c.Restore.TopicMapping,
	}

	// Add topic selectors if filter is specified
	if c.Restore.TopicFilter != "" {
		restore.TopicSelectors = []domain.TopicSelector{
			{
				Type:    domain.SelectorTypeGlob,
				Pattern: c.Restore.TopicFilter,
			},
		}
	}

	return restore
}

func (c *Config) toSecurityConfig() domain.SecurityConfig {
	secConfig := domain.SecurityConfig{
		Protocol:      domain.SecurityProtocol(c.Kafka.SecurityProtocol),
		SASLMechanism: domain.SASLMechanism(c.Kafka.SASL.Mechanism),
		Username:      c.Kafka.SASL.Username,
		Password:      c.Kafka.SASL.Password,
	}

	if c.Kafka.TLS.Enabled {
		secConfig.TLSConfig = &domain.TLSConfig{
			Enabled:            c.Kafka.TLS.Enabled,
			InsecureSkipVerify: c.Kafka.TLS.InsecureSkipVerify,
		}

		// Load TLS files if specified
		if c.Kafka.TLS.CAFile != "" {
			if data, err := os.ReadFile(c.Kafka.TLS.CAFile); err == nil {
				secConfig.TLSConfig.CACert = data
			}
		}
		if c.Kafka.TLS.CertFile != "" {
			if data, err := os.ReadFile(c.Kafka.TLS.CertFile); err == nil {
				secConfig.TLSConfig.ClientCert = data
			}
		}
		if c.Kafka.TLS.KeyFile != "" {
			if data, err := os.ReadFile(c.Kafka.TLS.KeyFile); err == nil {
				secConfig.TLSConfig.ClientKey = data
			}
		}
	}

	return secConfig
}

func (c *Config) toStorageConfig() domain.StorageConfig {
	switch c.Storage.Type {
	case "s3":
		return domain.S3Config{
			Endpoint:        c.Storage.S3.Endpoint,
			AccessKeyID:     c.Storage.S3.AccessKeyID,
			SecretAccessKey: c.Storage.S3.SecretAccessKey,
			UsePathStyle:    c.Storage.S3.UsePathStyle,
		}
	case "azure":
		return domain.AzureConfig{
			AccountName: c.Storage.Azure.AccountName,
			AccountKey:  c.Storage.Azure.AccountKey,
		}
	case "gcs":
		var credData []byte
		if c.Storage.GCS.CredentialsFile != "" {
			credData, _ = os.ReadFile(c.Storage.GCS.CredentialsFile)
		}
		return domain.GCSConfig{
			CredentialsJSON: credData,
			ProjectID:       c.Storage.GCS.ProjectID,
		}
	case "local":
		return domain.LocalConfig{
			BasePath: c.Storage.Path,
		}
	default:
		return domain.LocalConfig{
			BasePath: c.Storage.Path,
		}
	}
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
