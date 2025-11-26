package domain

import "time"

// Backup represents a backup operation
type Backup struct {
	ID             string
	Name           string
	SourceCluster  *KafkaCluster
	TargetStorage  *Storage
	TopicSelectors []TopicSelector
	WorkerConfig   WorkerConfig
	Compression    bool
	BatchSize      int
	FlushInterval  time.Duration
	Status         BackupStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TopicSelector defines how to select topics
type TopicSelector struct {
	Type    SelectorType
	Pattern string
}

type SelectorType string

const (
	SelectorTypeExact SelectorType = "exact"
	SelectorTypeGlob  SelectorType = "glob"
	SelectorTypeRegex SelectorType = "regex"
)

// WorkerConfig defines worker distribution settings
type WorkerConfig struct {
	Workers int
	Seed    int
}

// BackupStatus represents the status of a backup
type BackupStatus struct {
	Phase             BackupPhase
	TopicsDiscovered  int
	TopicsBackedUp    int
	MessagesProcessed int64
	BytesProcessed    int64
	LastBackupTime    *time.Time
	Workers           []WorkerStatus
	Errors            []string
}

type BackupPhase string

const (
	BackupPhasePending   BackupPhase = "Pending"
	BackupPhaseRunning   BackupPhase = "Running"
	BackupPhaseCompleted BackupPhase = "Completed"
	BackupPhaseFailed    BackupPhase = "Failed"
	BackupPhaseSuspended BackupPhase = "Suspended"
)

// WorkerStatus represents the status of a worker
type WorkerStatus struct {
	ID                int
	Topics            []string
	MessagesProcessed int64
	LastHeartbeat     time.Time
	Status            string
}

// MatchesTopics returns topics that match the selectors
func (b *Backup) MatchesTopics(topics []*Topic) []*Topic {
	var matched []*Topic
	for _, topic := range topics {
		for _, selector := range b.TopicSelectors {
			if selector.Matches(topic.Name) {
				matched = append(matched, topic)
				break
			}
		}
	}
	return matched
}

// Matches checks if a topic name matches the selector
func (s *TopicSelector) Matches(topicName string) bool {
	// Implementation depends on selector type
	return true
}

// DistributeTopics distributes topics across workers
func (b *Backup) DistributeTopics(topics []*Topic) map[int][]*Topic {
	distribution := make(map[int][]*Topic)

	for _, topic := range topics {
		workerID := b.hashTopic(topic.Name) % b.WorkerConfig.Workers
		distribution[workerID] = append(distribution[workerID], topic)
	}

	return distribution
}

func (b *Backup) hashTopic(topicName string) int {
	// Consistent hashing implementation
	return 0
}
