package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	restoreFinalizer = "restore.kafka.io/finalizer"
)

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=backup.kafka.io,resources=restores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.kafka.io,resources=restores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.kafka.io,resources=restores/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *RestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Restore instance
	restore := &backupv1alpha1.Restore{}
	if err := r.Get(ctx, req.NamespacedName, restore); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !restore.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, restore)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(restore, restoreFinalizer) {
		controllerutil.AddFinalizer(restore, restoreFinalizer)
		if err := r.Update(ctx, restore); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Skip if already completed or failed
	if restore.Status.Phase == backupv1alpha1.RestorePhaseCompleted ||
		restore.Status.Phase == backupv1alpha1.RestorePhaseFailed {
		return ctrl.Result{}, nil
	}

	// Get referenced KafkaCluster (target)
	kafkaCluster := &backupv1alpha1.KafkaCluster{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      restore.Spec.Target,
		Namespace: restore.Namespace,
	}, kafkaCluster); err != nil {
		log.Error(err, "Failed to get target KafkaCluster", "name", restore.Spec.Target)
		return r.updateStatus(ctx, restore, backupv1alpha1.RestorePhaseFailed, err.Error())
	}

	// Get referenced StorageConfig (source)
	storageConfig := &backupv1alpha1.StorageConfig{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      restore.Spec.Source,
		Namespace: restore.Namespace,
	}, storageConfig); err != nil {
		log.Error(err, "Failed to get StorageConfig", "name", restore.Spec.Source)
		return r.updateStatus(ctx, restore, backupv1alpha1.RestorePhaseFailed, err.Error())
	}

	// Set status to validating if this is the first reconciliation
	if restore.Status.Phase == "" {
		return r.updateStatus(ctx, restore, backupv1alpha1.RestorePhaseValidating, "Validating restore configuration")
	}

	// Create or update ConfigMap with configuration
	if err := r.reconcileConfigMap(ctx, restore, kafkaCluster, storageConfig); err != nil {
		log.Error(err, "Failed to reconcile ConfigMap")
		return r.updateStatus(ctx, restore, backupv1alpha1.RestorePhaseFailed, err.Error())
	}

	// Create restore job
	return r.reconcileJob(ctx, restore)
}

func (r *RestoreReconciler) reconcileConfigMap(ctx context.Context, restore *backupv1alpha1.Restore,
	kafkaCluster *backupv1alpha1.KafkaCluster, storage *backupv1alpha1.StorageConfig) error {

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restore.Name + "-config",
			Namespace: restore.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}

		// Build configuration
		config := r.buildConfig(restore, kafkaCluster, storage)
		configMap.Data["config.yaml"] = config

		// Set owner reference
		return controllerutil.SetControllerReference(restore, configMap, r.Scheme)
	})

	return err
}

func (r *RestoreReconciler) buildConfig(restore *backupv1alpha1.Restore,
	kafkaCluster *backupv1alpha1.KafkaCluster, storage *backupv1alpha1.StorageConfig) string {

	// Build YAML configuration
	config := fmt.Sprintf(`kafka:
  bootstrap_servers: "%s"
  security_protocol: "%s"
  group_id: "kafka-restore-%s"

storage:
  type: "%s"
  path: "%s"
  region: "%s"
  prefix: "%s"

workers: %d
`,
		kafkaCluster.Spec.BootstrapServers,
		kafkaCluster.Spec.SecurityProtocol,
		restore.Name,
		storage.Spec.Type,
		storage.Spec.Path,
		storage.Spec.Region,
		storage.Spec.Prefix,
		restore.Spec.Workers,
	)

	return config
}

func (r *RestoreReconciler) reconcileJob(ctx context.Context, restore *backupv1alpha1.Restore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Check if job already exists
	job := &batchv1.Job{}
	jobName := restore.Name + "-job"
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: restore.Namespace}, job)

	if err != nil && errors.IsNotFound(err) {
		// Create new job
		job = r.buildJob(restore)
		if err := controllerutil.SetControllerReference(restore, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("Creating restore job")
		if err := r.Create(ctx, job); err != nil {
			return ctrl.Result{}, err
		}

		now := metav1.Now()
		restore.Status.StartTime = &now
		return r.updateStatus(ctx, restore, backupv1alpha1.RestorePhaseRunning, "Restore job started")
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Check job status
	return r.updateStatusFromJob(ctx, restore, job)
}

func (r *RestoreReconciler) buildJob(restore *backupv1alpha1.Restore) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restore.Name + "-job",
			Namespace: restore.Namespace,
			Labels: map[string]string{
				"app":     "kafka-restore",
				"restore": restore.Name,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "kafka-restore",
						"restore": restore.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "restore",
							Image: "kafka-backup:latest", // TODO: make configurable
							Args: []string{
								"--mode", "restore",
								"--config", "/config/config.yaml",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "BACKUP_NAME",
									Value: restore.Spec.BackupName,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/config",
								},
							},
							Resources: r.buildResourceRequirements(restore),
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: restore.Name + "-config",
									},
								},
							},
						},
					},
				},
			},
			BackoffLimit: func() *int32 { i := int32(3); return &i }(),
		},
	}
}

func (r *RestoreReconciler) buildResourceRequirements(restore *backupv1alpha1.Restore) corev1.ResourceRequirements {
	req := corev1.ResourceRequirements{}

	if restore.Spec.Resources != nil {
		req.Requests = corev1.ResourceList{}
		req.Limits = corev1.ResourceList{}

		// Similar to backup controller
	}

	return req
}

func (r *RestoreReconciler) updateStatusFromJob(ctx context.Context, restore *backupv1alpha1.Restore,
	job *batchv1.Job) (ctrl.Result, error) {

	// Check if job is completed
	if job.Status.Succeeded > 0 {
		now := metav1.Now()
		restore.Status.CompletionTime = &now
		return r.updateStatus(ctx, restore, backupv1alpha1.RestorePhaseCompleted, "Restore completed successfully")
	}

	// Check if job failed
	if job.Status.Failed > 0 {
		return r.updateStatus(ctx, restore, backupv1alpha1.RestorePhaseFailed, "Restore job failed")
	}

	// Job is still running
	if restore.Status.Phase != backupv1alpha1.RestorePhaseRunning {
		return r.updateStatus(ctx, restore, backupv1alpha1.RestorePhaseRunning, "Restore in progress")
	}

	// Requeue to check status again
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *RestoreReconciler) handleDeletion(ctx context.Context, restore *backupv1alpha1.Restore) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(restore, restoreFinalizer) {
		// Cleanup resources
		job := &batchv1.Job{}
		jobName := restore.Name + "-job"
		if err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: restore.Namespace}, job); err == nil {
			r.Delete(ctx, job)
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(restore, restoreFinalizer)
		if err := r.Update(ctx, restore); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *RestoreReconciler) updateStatus(ctx context.Context, restore *backupv1alpha1.Restore,
	phase backupv1alpha1.RestorePhase, message string) (ctrl.Result, error) {

	restore.Status.Phase = phase
	restore.Status.ObservedGeneration = restore.Generation

	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             string(phase),
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	if phase == backupv1alpha1.RestorePhaseFailed {
		condition.Status = metav1.ConditionFalse
	}

	restore.Status.Conditions = append(restore.Status.Conditions, condition)

	if err := r.Status().Update(ctx, restore); err != nil {
		return ctrl.Result{}, err
	}

	// Don't requeue if completed or failed
	if phase == backupv1alpha1.RestorePhaseCompleted || phase == backupv1alpha1.RestorePhaseFailed {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.Restore{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
