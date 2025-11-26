package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSpec defines the desired state of Backup
type BackupSpec struct {
	// Source is the name of the Kafka cluster to backup
	Source string `json:"source"`

	// Sink is the name of the storage configuration to use
	Sink string `json:"sink"`

	// TopicSelectors define which topics to backup
	TopicSelectors TopicSelectors `json:"topicSelectors"`

	// WorkGroup defines worker distribution settings
	// +optional
	WorkGroup *WorkGroup `json:"workGroup,omitempty"`

	// Schedule for periodic backups (cron format)
	// If not specified, runs continuously
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Compression enables GZIP compression
	// +optional
	// +kubebuilder:default=true
	Compression bool `json:"compression,omitempty"`

	// BatchSize is the number of messages per batch
	// +optional
	// +kubebuilder:default=1000
	BatchSize int `json:"batchSize,omitempty"`

	// FlushInterval is the time between flushes
	// +optional
	// +kubebuilder:default="30s"
	FlushInterval string `json:"flushInterval,omitempty"`

	// Suspend pauses the backup job
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// Resources defines resource requirements for backup pods
	// +optional
	Resources *Resources `json:"resources,omitempty"`
}

// TopicSelectors defines how to select topics
type TopicSelectors struct {
	// Matchers define topic selection rules
	Matchers []TopicMatcher `json:"matchers"`
}

// TopicMatcher defines a single topic matching rule
type TopicMatcher struct {
	// Name can be exact name or glob pattern
	Name string `json:"name,omitempty"`

	// Glob pattern for topic names
	Glob string `json:"glob,omitempty"`

	// Regex pattern for topic names
	Regex string `json:"regex,omitempty"`
}

// WorkGroup defines worker distribution
type WorkGroup struct {
	// Workers is the number of worker pods
	// +kubebuilder:validation:Minimum=1
	Workers int `json:"workers"`

	// Seed for topic distribution hashing
	// +optional
	Seed int `json:"seed,omitempty"`
}

// Resources defines resource requirements
type Resources struct {
	// Requests describes the minimum resources required
	// +optional
	Requests ResourceList `json:"requests,omitempty"`

	// Limits describes the maximum resources allowed
	// +optional
	Limits ResourceList `json:"limits,omitempty"`
}

// ResourceList is a set of resource quantities
type ResourceList struct {
	// CPU resource quantity
	// +optional
	CPU string `json:"cpu,omitempty"`

	// Memory resource quantity
	// +optional
	Memory string `json:"memory,omitempty"`
}

// BackupStatus defines the observed state of Backup
type BackupStatus struct {
	// Phase represents the current phase of the backup
	Phase BackupPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastBackupTime is the timestamp of the last successful backup
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// TopicsBackedUp is the number of topics being backed up
	TopicsBackedUp int `json:"topicsBackedUp,omitempty"`

	// TotalMessages is the total number of messages backed up
	TotalMessages int64 `json:"totalMessages,omitempty"`

	// TotalBytes is the total size of backed up data
	TotalBytes int64 `json:"totalBytes,omitempty"`

	// WorkerStatus contains status of each worker
	// +optional
	WorkerStatus []WorkerStatus `json:"workerStatus,omitempty"`

	// ObservedGeneration reflects the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// BackupPhase represents the phase of a backup
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed;Suspended
type BackupPhase string

const (
	BackupPhasePending   BackupPhase = "Pending"
	BackupPhaseRunning   BackupPhase = "Running"
	BackupPhaseCompleted BackupPhase = "Completed"
	BackupPhaseFailed    BackupPhase = "Failed"
	BackupPhaseSuspended BackupPhase = "Suspended"
)

// WorkerStatus represents the status of a single worker
type WorkerStatus struct {
	// WorkerID is the identifier of the worker
	WorkerID int `json:"workerId"`

	// Topics assigned to this worker
	Topics []string `json:"topics,omitempty"`

	// MessagesProcessed by this worker
	MessagesProcessed int64 `json:"messagesProcessed,omitempty"`

	// LastHeartbeat from the worker
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// Status of the worker pod
	Status string `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kb
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Topics",type=integer,JSONPath=`.status.topicsBackedUp`
// +kubebuilder:printcolumn:name="Messages",type=integer,JSONPath=`.status.totalMessages`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Backup is the Schema for the backups API
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}
