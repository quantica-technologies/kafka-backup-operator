package domain

import "time"

// Message represents a Kafka message in the domain
type Message struct {
	Offset    int64
	Partition int32
	Key       []byte
	Value     []byte
	Timestamp time.Time
	Headers   []Header
}

// Header represents a message header
type Header struct {
	Key   string
	Value []byte
}
