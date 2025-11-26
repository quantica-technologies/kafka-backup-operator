package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1alpha1 "github.com/quantica-technologies/kafka-backup-operator/api/v1alpha1"
)

const (
	backupFinalizer = "backup.kafka.io/finalizer"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=backup.kafka.io,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.kafka.io,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.kafka.io,resources=backups/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Backup instance
	backup := &backupv1alpha1.Backup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !backup.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, backup)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(backup, backupFinalizer) {
		controllerutil.AddFinalizer(backup, backupFinalizer)
		if err := r.Update(ctx, backup); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get referenced KafkaCluster
	kafkaCluster := &backupv1alpha1.KafkaCluster{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      backup.Spec.Source,
		Namespace: backup.Namespace,
	}, kafkaCluster); err != nil {
		log.Error(err, "Failed to get KafkaCluster", "name", backup.Spec.Source)
		return r.updateStatus(ctx, backup, backupv1alpha1.BackupPhaseFailed, err.Error())
	}

	// Get referenced StorageConfig
	storageConfig := &backupv1alpha1.StorageConfig{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      backup.Spec.Sink,
		Namespace: backup.Namespace,
	}, storageConfig); err != nil {
		log.Error(err, "Failed to get StorageConfig", "name", backup.Spec.Sink)
		return r.updateStatus(ctx, backup, backupv1alpha1.BackupPhaseFailed, err.Error())
	}

	// Handle suspension
	if backup.Spec.Suspend {
		return r.handleSuspension(ctx, backup)
	}

	// Create or update ConfigMap with configuration
	if err := r.reconcileConfigMap(ctx, backup, kafkaCluster, storageConfig); err != nil {
		log.Error(err, "Failed to reconcile ConfigMap")
		return r.updateStatus(ctx, backup, backupv1alpha1.BackupPhaseFailed, err.Error())
	}

	// Determine if this is a scheduled or continuous backup
	if backup.Spec.Schedule != "" {
		return r.reconcileCronJob(ctx, backup)
	} else {
		return r.reconcileJob(ctx, backup)
	}
}

func (r *BackupReconciler) reconcileConfigMap(ctx context.Context, backup *backupv1alpha1.Backup,
	kafkaCluster *backupv1alpha1.KafkaCluster, storage *backupv1alpha1.StorageConfig) error {

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name + "-config",
			Namespace: backup.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}

		// Build configuration
		config := r.buildConfig(backup, kafkaCluster, storage)
		configMap.Data["config.yaml"] = config

		// Set owner reference
		return controllerutil.SetControllerReference(backup, configMap, r.Scheme)
	})

	return err
}

func (r *BackupReconciler) buildConfig(backup *backupv1alpha1.Backup,
	kafkaCluster *backupv1alpha1.KafkaCluster, storage *backupv1alpha1.StorageConfig) string {

	// Build YAML configuration
	config := fmt.Sprintf(`kafka:
  bootstrap_servers: "%s"
  security_protocol: "%s"
  group_id: "kafka-backup-%s"

storage:
  type: "%s"
  path: "%s"
  region: "%s"
  prefix: "%s"

workers: %d
compression: %t
batch_size: %d
flush_interval: "%s"
`,
		kafkaCluster.Spec.BootstrapServers,
		kafkaCluster.Spec.SecurityProtocol,
		backup.Name,
		storage.Spec.Type,
		storage.Spec.Path,
		storage.Spec.Region,
		storage.Spec.Prefix,
		getWorkers(backup),
		backup.Spec.Compression,
		backup.Spec.BatchSize,
		backup.Spec.FlushInterval,
	)

	return config
}

func (r *BackupReconciler) reconcileJob(ctx context.Context, backup *backupv1alpha1.Backup) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// For continuous backup, we use a Deployment with multiple replicas if WorkGroup is specified
	workers := getWorkers(backup)

	for i := 0; i < workers; i++ {
		jobName := fmt.Sprintf("%s-worker-%d", backup.Name, i)
		job := &batchv1.Job{}
		err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: backup.Namespace}, job)

		if err != nil && errors.IsNotFound(err) {
			// Create new job
			job = r.buildJob(backup, i)
			if err := controllerutil.SetControllerReference(backup, job, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}

			log.Info("Creating backup job", "worker", i)
			if err := r.Create(ctx, job); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return r.updateStatus(ctx, backup, backupv1alpha1.BackupPhaseRunning, "")
}

func (r *BackupReconciler) reconcileCronJob(ctx context.Context, backup *backupv1alpha1.Backup) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cronJob, func() error {
		cronJob.Spec.Schedule = backup.Spec.Schedule
		cronJob.Spec.JobTemplate.Spec = r.buildJobSpec(backup, 0)

		return controllerutil.SetControllerReference(backup, cronJob, r.Scheme)
	})

	if err != nil {
		log.Error(err, "Failed to reconcile CronJob")
		return r.updateStatus(ctx, backup, backupv1alpha1.BackupPhaseFailed, err.Error())
	}

	return r.updateStatus(ctx, backup, backupv1alpha1.BackupPhaseRunning, "")
}

func (r *BackupReconciler) buildJob(backup *backupv1alpha1.Backup, workerID int) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-worker-%d", backup.Name, workerID),
			Namespace: backup.Namespace,
			Labels: map[string]string{
				"app":       "kafka-backup",
				"backup":    backup.Name,
				"worker-id": fmt.Sprintf("%d", workerID),
			},
		},
		Spec: r.buildJobSpec(backup, workerID),
	}
}

func (r *BackupReconciler) buildJobSpec(backup *backupv1alpha1.Backup, workerID int) batchv1.JobSpec {
	workers := getWorkers(backup)

	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app":       "kafka-backup",
					"backup":    backup.Name,
					"worker-id": fmt.Sprintf("%d", workerID),
				},
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyOnFailure,
				Containers: []corev1.Container{
					{
						Name:  "backup",
						Image: "kafka-backup:latest", // TODO: make configurable
						Args: []string{
							"--mode", "backup",
							"--config", "/config/config.yaml",
						},
						Env: []corev1.EnvVar{
							{
								Name:  "WORKER_ID",
								Value: fmt.Sprintf("%d", workerID),
							},
							{
								Name:  "TOTAL_WORKERS",
								Value: fmt.Sprintf("%d", workers),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "config",
								MountPath: "/config",
							},
						},
						Resources: r.buildResourceRequirements(backup),
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: backup.Name + "-config",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *BackupReconciler) buildResourceRequirements(backup *backupv1alpha1.Backup) corev1.ResourceRequirements {
	req := corev1.ResourceRequirements{}

	if backup.Spec.Resources != nil {
		req.Requests = corev1.ResourceList{}
		req.Limits = corev1.ResourceList{}

		if backup.Spec.Resources.Requests.CPU != "" {
			req.Requests[corev1.ResourceCPU] = resource.MustParse(backup.Spec.Resources.Requests.CPU)
		}
		if backup.Spec.Resources.Requests.Memory != "" {
			req.Requests[corev1.ResourceMemory] = resource.MustParse(backup.Spec.Resources.Requests.Memory)
		}
		if backup.Spec.Resources.Limits.CPU != "" {
			req.Limits[corev1.ResourceCPU] = resource.MustParse(backup.Spec.Resources.Limits.CPU)
		}
		if backup.Spec.Resources.Limits.Memory != "" {
			req.Limits[corev1.ResourceMemory] = resource.MustParse(backup.Spec.Resources.Limits.Memory)
		}
	}

	return req
}

func (r *BackupReconciler) handleSuspension(ctx context.Context, backup *backupv1alpha1.Backup) (ctrl.Result, error) {
	// Delete all running jobs
	jobList := &batchv1.JobList{}
	if err := r.List(ctx, jobList, client.InNamespace(backup.Namespace),
		client.MatchingLabels{"backup": backup.Name}); err != nil {
		return ctrl.Result{}, err
	}

	for _, job := range jobList.Items {
		if err := r.Delete(ctx, &job); err != nil {
			return ctrl.Result{}, err
		}
	}

	return r.updateStatus(ctx, backup, backupv1alpha1.BackupPhaseSuspended, "")
}

func (r *BackupReconciler) handleDeletion(ctx context.Context, backup *backupv1alpha1.Backup) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(backup, backupFinalizer) {
		// Cleanup resources
		// Delete all jobs
		jobList := &batchv1.JobList{}
		if err := r.List(ctx, jobList, client.InNamespace(backup.Namespace),
			client.MatchingLabels{"backup": backup.Name}); err == nil {
			for _, job := range jobList.Items {
				r.Delete(ctx, &job)
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(backup, backupFinalizer)
		if err := r.Update(ctx, backup); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *BackupReconciler) updateStatus(ctx context.Context, backup *backupv1alpha1.Backup,
	phase backupv1alpha1.BackupPhase, message string) (ctrl.Result, error) {

	backup.Status.Phase = phase
	backup.Status.ObservedGeneration = backup.Generation

	if message != "" {
		condition := metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             string(phase),
			Message:            message,
			LastTransitionTime: metav1.Now(),
		}
		backup.Status.Conditions = append(backup.Status.Conditions, condition)
	}

	if phase == backupv1alpha1.BackupPhaseRunning {
		now := metav1.Now()
		backup.Status.LastBackupTime = &now
	}

	if err := r.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue after 30 seconds to update status
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func getWorkers(backup *backupv1alpha1.Backup) int {
	if backup.Spec.WorkGroup != nil {
		return backup.Spec.WorkGroup.Workers
	}
	return 1
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.Backup{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
