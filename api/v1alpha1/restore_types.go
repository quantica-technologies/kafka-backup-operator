package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RestoreSpec defines the desired state of Restore
type RestoreSpec struct {
	// Source is the name of the storage configuration to restore from
	Source string `json:"source"`

	// Target is the name of the Kafka cluster to restore to
	Target string `json:"target"`

	// BackupName references a specific backup to restore
	// If empty, restores the latest backup
	// +optional
	BackupName string `json:"backupName,omitempty"`

	// BackupTimestamp specifies a point-in-time to restore to
	// +optional
	BackupTimestamp *metav1.Time `json:"backupTimestamp,omitempty"`

	// TopicSelectors define which topics to restore
	// If empty, restores all topics from backup
	// +optional
	TopicSelectors *TopicSelectors `json:"topicSelectors,omitempty"`

	// TopicMapping allows renaming topics during restore
	// +optional
	TopicMapping map[string]string `json:"topicMapping,omitempty"`

	// Workers is the number of parallel restore workers
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	Workers int `json:"workers,omitempty"`

	// CreateTopics specifies whether to create topics if they don't exist
	// +optional
	// +kubebuilder:default=true
	CreateTopics bool `json:"createTopics,omitempty"`

	// OverwriteExisting specifies whether to overwrite existing data
	// +optional
	OverwriteExisting bool `json:"overwriteExisting,omitempty"`

	// PartitionFilter allows restoring specific partitions only
	// +optional
	PartitionFilter map[string][]int32 `json:"partitionFilter,omitempty"`

	// Resources defines resource requirements for restore pods
	// +optional
	Resources *Resources `json:"resources,omitempty"`
}

// RestoreStatus defines the observed state of Restore
type RestoreStatus struct {
	// Phase represents the current phase of the restore
	Phase RestorePhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// StartTime is when the restore started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the restore completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// TopicsRestored is the number of topics restored
	TopicsRestored int `json:"topicsRestored,omitempty"`

	// TotalMessages is the total number of messages restored
	TotalMessages int64 `json:"totalMessages,omitempty"`

	// TotalBytes is the total size of restored data
	TotalBytes int64 `json:"totalBytes,omitempty"`

	// Progress shows restore progress per topic
	// +optional
	Progress []TopicProgress `json:"progress,omitempty"`

	// Errors contains any errors encountered during restore
	// +optional
	Errors []string `json:"errors,omitempty"`

	// ObservedGeneration reflects the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// RestorePhase represents the phase of a restore
// +kubebuilder:validation:Enum=Pending;Validating;Running;Completed;Failed;PartiallyCompleted
type RestorePhase string

const (
	RestorePhasePending            RestorePhase = "Pending"
	RestorePhaseValidating         RestorePhase = "Validating"
	RestorePhaseRunning            RestorePhase = "Running"
	RestorePhaseCompleted          RestorePhase = "Completed"
	RestorePhaseFailed             RestorePhase = "Failed"
	RestorePhasePartiallyCompleted RestorePhase = "PartiallyCompleted"
)

// TopicProgress represents restore progress for a topic
type TopicProgress struct {
	// TopicName is the name of the topic
	TopicName string `json:"topicName"`

	// PartitionsTotal is the total number of partitions
	PartitionsTotal int `json:"partitionsTotal"`

	// PartitionsCompleted is the number of completed partitions
	PartitionsCompleted int `json:"partitionsCompleted"`

	// MessagesRestored is the number of messages restored
	MessagesRestored int64 `json:"messagesRestored"`

	// Status is the current status of the topic restore
	Status string `json:"status"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kr
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Topics",type=integer,JSONPath=`.status.topicsRestored`
// +kubebuilder:printcolumn:name="Messages",type=integer,JSONPath=`.status.totalMessages`
// +kubebuilder:printcolumn:name="Duration",type=string,JSONPath=`.status.completionTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Restore is the Schema for the restores API
type Restore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreSpec   `json:"spec,omitempty"`
	Status RestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RestoreList contains a list of Restore
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Restore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Restore{}, &RestoreList{})
}
