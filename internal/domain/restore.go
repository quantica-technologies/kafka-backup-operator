package domain

import "time"

// Restore represents a restore operation
type Restore struct {
	ID                string
	Name              string
	SourceStorage     *Storage
	TargetCluster     *KafkaCluster
	BackupID          string
	BackupTimestamp   *time.Time
	TopicSelectors    []TopicSelector
	TopicMapping      map[string]string
	PartitionFilter   map[string][]int32
	Workers           int
	CreateTopics      bool
	OverwriteExisting bool
	Status            RestoreStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// RestoreStatus represents the status of a restore
type RestoreStatus struct {
	Phase            RestorePhase
	StartTime        *time.Time
	CompletionTime   *time.Time
	TopicsRestored   int
	MessagesRestored int64
	BytesRestored    int64
	Progress         []TopicProgress
	Errors           []string
}

type RestorePhase string

const (
	RestorePhasePending            RestorePhase = "Pending"
	RestorePhaseValidating         RestorePhase = "Validating"
	RestorePhaseRunning            RestorePhase = "Running"
	RestorePhaseCompleted          RestorePhase = "Completed"
	RestorePhaseFailed             RestorePhase = "Failed"
	RestorePhasePartiallyCompleted RestorePhase = "PartiallyCompleted"
)

// TopicProgress tracks progress for a topic
type TopicProgress struct {
	TopicName           string
	PartitionsTotal     int
	PartitionsCompleted int
	MessagesRestored    int64
	Status              string
}

// GetMappedTopicName returns the mapped topic name or original if no mapping
func (r *Restore) GetMappedTopicName(originalName string) string {
	if mapped, ok := r.TopicMapping[originalName]; ok {
		return mapped
	}
	return originalName
}

// ShouldRestorePartition checks if a partition should be restored
func (r *Restore) ShouldRestorePartition(topic string, partition int32) bool {
	if r.PartitionFilter == nil {
		return true
	}

	if partitions, ok := r.PartitionFilter[topic]; ok {
		for _, p := range partitions {
			if p == partition {
				return true
			}
		}
		return false
	}

	return true
}
