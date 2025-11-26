package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KafkaClusterSpec defines the Kafka cluster connection details
type KafkaClusterSpec struct {
	// BootstrapServers is the Kafka bootstrap servers
	BootstrapServers string `json:"bootstrapServers"`

	// SecurityProtocol defines the security protocol
	// +optional
	// +kubebuilder:default="PLAINTEXT"
	// +kubebuilder:validation:Enum=PLAINTEXT;SASL_PLAINTEXT;SASL_SSL;SSL
	SecurityProtocol string `json:"securityProtocol,omitempty"`

	// SASL configuration
	// +optional
	SASL *SASLConfig `json:"sasl,omitempty"`

	// TLS configuration
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`

	// Properties for additional Kafka configuration
	// +optional
	Properties map[string]string `json:"properties,omitempty"`
}

// SASLConfig defines SASL authentication
type SASLConfig struct {
	// Mechanism is the SASL mechanism
	// +kubebuilder:validation:Enum=PLAIN;SCRAM-SHA-256;SCRAM-SHA-512
	Mechanism string `json:"mechanism"`

	// Username for SASL authentication
	Username string `json:"username,omitempty"`

	// Password for SASL authentication
	Password string `json:"password,omitempty"`

	// SecretRef references a secret containing credentials
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// TLSConfig defines TLS settings
type TLSConfig struct {
	// Enabled specifies whether TLS is enabled
	Enabled bool `json:"enabled"`

	// InsecureSkipVerify skips certificate verification
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// CASecretRef references a secret containing the CA certificate
	// +optional
	CASecretRef *SecretReference `json:"caSecretRef,omitempty"`

	// CertSecretRef references a secret containing client certificate
	// +optional
	CertSecretRef *SecretReference `json:"certSecretRef,omitempty"`
}

// SecretReference references a Kubernetes secret
type SecretReference struct {
	// Name of the secret
	Name string `json:"name"`

	// Namespace of the secret
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Key in the secret
	// +optional
	Key string `json:"key,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=kc

// KafkaCluster defines a Kafka cluster configuration
type KafkaCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KafkaClusterSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// KafkaClusterList contains a list of KafkaCluster
type KafkaClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaCluster `json:"items"`
}

// StorageConfigSpec defines storage backend configuration
type StorageConfigSpec struct {
	// Type of storage backend
	// +kubebuilder:validation:Enum=s3;azure;gcs;local
	Type string `json:"type"`

	// Path is the bucket name or local path
	Path string `json:"path"`

	// Region for cloud storage
	// +optional
	Region string `json:"region,omitempty"`

	// Prefix for all stored objects
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// S3 configuration
	// +optional
	S3 *S3Config `json:"s3,omitempty"`

	// Azure configuration
	// +optional
	Azure *AzureConfig `json:"azure,omitempty"`

	// GCS configuration
	// +optional
	GCS *GCSConfig `json:"gcs,omitempty"`
}

// S3Config defines AWS S3 configuration
type S3Config struct {
	// Endpoint for S3-compatible storage
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// AccessKeyID for S3 authentication
	// +optional
	AccessKeyID string `json:"accessKeyId,omitempty"`

	// SecretAccessKey for S3 authentication
	// +optional
	SecretAccessKey string `json:"secretAccessKey,omitempty"`

	// SecretRef references a secret containing credentials
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// UsePathStyle enables path-style URLs
	// +optional
	UsePathStyle bool `json:"usePathStyle,omitempty"`
}

// AzureConfig defines Azure Blob Storage configuration
type AzureConfig struct {
	// AccountName for Azure storage
	AccountName string `json:"accountName,omitempty"`

	// AccountKey for Azure storage
	// +optional
	AccountKey string `json:"accountKey,omitempty"`

	// SecretRef references a secret containing credentials
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// GCSConfig defines Google Cloud Storage configuration
type GCSConfig struct {
	// CredentialsFile path to GCS credentials
	// +optional
	CredentialsFile string `json:"credentialsFile,omitempty"`

	// SecretRef references a secret containing credentials
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=sc

// StorageConfig defines a storage backend configuration
type StorageConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec StorageConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// StorageConfigList contains a list of StorageConfig
type StorageConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaCluster{}, &KafkaClusterList{})
	SchemeBuilder.Register(&StorageConfig{}, &StorageConfigList{})
}
