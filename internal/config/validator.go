package config

import (
	"fmt"
	"regexp"
	"strings"
)

// Validator provides configuration validation utilities
type Validator struct{}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateTopicPattern validates a topic pattern
func (v *Validator) ValidateTopicPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("topic pattern cannot be empty")
	}

	// Check for invalid characters
	invalidChars := []string{"/", "\\", "\x00"}
	for _, char := range invalidChars {
		if strings.Contains(pattern, char) {
			return fmt.Errorf("topic pattern contains invalid character: %s", char)
		}
	}

	// If it's a regex pattern, try to compile it
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	return nil
}

// ValidateStorageType validates storage type
func (v *Validator) ValidateStorageType(storageType string) error {
	validTypes := []string{"s3", "azure", "gcs", "local"}

	for _, valid := range validTypes {
		if storageType == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid storage type: %s. Valid types: %v", storageType, validTypes)
}

// ValidateSecurityProtocol validates Kafka security protocol
func (v *Validator) ValidateSecurityProtocol(protocol string) error {
	validProtocols := []string{"PLAINTEXT", "SASL_PLAINTEXT", "SASL_SSL", "SSL"}

	for _, valid := range validProtocols {
		if protocol == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid security protocol: %s. Valid protocols: %v", protocol, validProtocols)
}
