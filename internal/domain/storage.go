package domain

// Storage represents a storage backend configuration
type Storage struct {
	Type   StorageType
	Path   string
	Region string
	Prefix string
	Config StorageConfig
}

type StorageType string

const (
	StorageTypeS3    StorageType = "s3"
	StorageTypeAzure StorageType = "azure"
	StorageTypeGCS   StorageType = "gcs"
	StorageTypeLocal StorageType = "local"
)

// StorageConfig is a marker interface for storage-specific configs
type StorageConfig interface {
	Validate() error
}

// S3Config holds S3-specific configuration
type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

func (c S3Config) Validate() error {
	return nil
}

// AzureConfig holds Azure Blob Storage configuration
type AzureConfig struct {
	AccountName string
	AccountKey  string
}

func (c AzureConfig) Validate() error {
	return nil
}

// GCSConfig holds Google Cloud Storage configuration
type GCSConfig struct {
	CredentialsJSON []byte
	ProjectID       string
}

func (c GCSConfig) Validate() error {
	return nil
}

// LocalConfig holds local filesystem configuration
type LocalConfig struct {
	BasePath string
}

func (c LocalConfig) Validate() error {
	return nil
}
