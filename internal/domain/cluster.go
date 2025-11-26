package domain

// KafkaCluster represents connection details for a Kafka cluster
type KafkaCluster struct {
	ID               string
	BootstrapServers string
	SecurityConfig   SecurityConfig
	Properties       map[string]string
}

// SecurityConfig holds authentication configuration
type SecurityConfig struct {
	Protocol      SecurityProtocol
	SASLMechanism SASLMechanism
	Username      string
	Password      string
	TLSConfig     *TLSConfig
}

type SecurityProtocol string

const (
	SecurityProtocolPlaintext SecurityProtocol = "PLAINTEXT"
	SecurityProtocolSASLPlain SecurityProtocol = "SASL_PLAINTEXT"
	SecurityProtocolSASLSSL   SecurityProtocol = "SASL_SSL"
	SecurityProtocolSSL       SecurityProtocol = "SSL"
)

type SASLMechanism string

const (
	SASLMechanismPlain       SASLMechanism = "PLAIN"
	SASLMechanismScramSHA256 SASLMechanism = "SCRAM-SHA-256"
	SASLMechanismScramSHA512 SASLMechanism = "SCRAM-SHA-512"
)

type TLSConfig struct {
	Enabled            bool
	InsecureSkipVerify bool
	CACert             []byte
	ClientCert         []byte
	ClientKey          []byte
}
