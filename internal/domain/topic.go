package domain

// Topic represents a Kafka topic
type Topic struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
	Config            map[string]string
}

// MatchesPattern checks if topic name matches a pattern
func (t *Topic) MatchesPattern(pattern string) bool {
	// Implement pattern matching logic
	return true
}
