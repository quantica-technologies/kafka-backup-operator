package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Backup metrics
	BackupMessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_backup_messages_processed_total",
			Help: "Total number of messages processed by backup",
		},
		[]string{"backup_id", "topic", "worker_id"},
	)

	BackupBytesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_backup_bytes_processed_total",
			Help: "Total bytes processed by backup",
		},
		[]string{"backup_id", "topic", "worker_id"},
	)

	BackupDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kafka_backup_duration_seconds",
			Help:    "Duration of backup operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backup_id", "operation"},
	)

	// Restore metrics
	RestoreMessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_restore_messages_processed_total",
			Help: "Total number of messages restored",
		},
		[]string{"restore_id", "topic", "worker_id"},
	)

	RestoreBytesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_restore_bytes_processed_total",
			Help: "Total bytes restored",
		},
		[]string{"restore_id", "topic", "worker_id"},
	)

	RestoreDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kafka_restore_duration_seconds",
			Help:    "Duration of restore operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"restore_id", "operation"},
	)

	// Storage metrics
	StorageOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_backup_storage_operations_total",
			Help: "Total storage operations",
		},
		[]string{"operation", "status"},
	)

	StorageLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kafka_backup_storage_latency_seconds",
			Help:    "Storage operation latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
)
