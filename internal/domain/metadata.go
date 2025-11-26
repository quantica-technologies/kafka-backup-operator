package domain

import "time"

// BackupMetadata contains metadata about a backup
type BackupMetadata struct {
	BackupID    string
	Timestamp   time.Time
	Topics      []TopicMetadata
	Version     string
	Compression bool
	Config      map[string]interface{}
}

// TopicMetadata contains metadata about a backed-up topic
type TopicMetadata struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
	Config            map[string]string
	Segments          []SegmentMetadata
}

// SegmentMetadata contains metadata about a backup segment
type SegmentMetadata struct {
	PartitionID   int32
	SegmentNumber int
	StartOffset   int64
	EndOffset     int64
	MessageCount  int64
	SizeBytes     int64
	StorageKey    string
	Checksum      string
}
