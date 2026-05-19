package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1alpha1 "github.com/chirichexe/cluster-backup-operator/api/v1alpha1"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=operators.com,resources=backups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.com,resources=backups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operators.com,resources=backups/finalizers,verbs=update

// For simplicity, we're adding RBAC permissions to list and get all resources.
// In a production setting, you'd want to be more specific about what your Operator needs to access.
//+kubebuilder:rbac:groups="",resources=*,verbs=get;list
//+kubebuilder:rbac:groups=apps,resources=*,verbs=get;list
//+kubebuilder:rbac:groups=batch,resources=*,verbs=get;list
// ...add other API groups as needed...

func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting reconciliation", "request", req.NamespacedName)
	// Fetch the Backup instance
	backup := &v1alpha1.Backup{}
	err := r.Get(ctx, req.NamespacedName, backup)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if the status is already completed or failed.  If so, don't do anything.
	if backup.Status.Status == v1alpha1.BackupStatusCompleted || backup.Status.Status == v1alpha1.BackupStatusFailed {
		logger.Info("Backup already completed or failed, skipping", "status", backup.Status.Status)
		return ctrl.Result{}, nil
	}

	// Initialize MinIO client (replace with your actual credentials and endpoint)
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin123", ""),
		Secure: false,
	})
	if err != nil {
		// Update status to Failed and set the error message.
		backup.Status.Status = v1alpha1.BackupStatusFailed
		backup.Status.Error = fmt.Sprintf("error initializing MinIO client: %v", err)
		if err := r.Status().Update(ctx, backup); err != nil {
			logger.Error(err, "unable to update Backup status")
		}
		return ctrl.Result{}, fmt.Errorf("error initializing MinIO client: %w", err) // Return the error for potential retry
	}

	// Set status to InProgress and update StartTime
	if backup.Status.Status != v1alpha1.BackupStatusInProgress {
		backup.Status.Status = v1alpha1.BackupStatusInProgress
		now := metav1.Now()
		backup.Status.StartTime = &now
		if err := r.Status().Update(ctx, backup); err != nil {
			logger.Error(err, "unable to update Backup status")
			return ctrl.Result{}, err // Return original error, but try updating status.
		}
	}

	// BACKUP LOGIC
	allItems := make([]unstructured.Unstructured, 0)
	for _, resource := range backup.Spec.Resources {
		// Get resources of the specified type
		resourceList := &unstructured.UnstructuredList{}
		resourceList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   resource.Group,
			Version: resource.Version,
			Kind:    resource.Name + "List",
		})

		err := r.List(ctx, resourceList, client.InNamespace(backup.Namespace))
		if err != nil {
			// Update status to Failed and set the error message.
			backup.Status.Status = v1alpha1.BackupStatusFailed
			backup.Status.Error = fmt.Sprintf("failed to list %s: %v", resource.Name, err)
			if err := r.Status().Update(ctx, backup); err != nil {
				logger.Error(err, "unable to update Backup status")
			}
			return ctrl.Result{}, fmt.Errorf("failed to list %s: %w", resource.Name, err)
		}

		// Serialize each resource to JSON and append to the list
		for _, item := range resourceList.Items {
			// Remove unnecessary data.
			unstructured.RemoveNestedField(item.Object, "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(item.Object, "metadata", "generation")
			unstructured.RemoveNestedField(item.Object, "metadata", "resourceVersion")
			unstructured.RemoveNestedField(item.Object, "metadata", "uid")
			unstructured.RemoveNestedField(item.Object, "status")
			allItems = append(allItems, item)
		}
	}

	data, err := json.Marshal(allItems)
	if err != nil {
		// Update status to Failed and set the error message.
		backup.Status.Status = v1alpha1.BackupStatusFailed
		backup.Status.Error = fmt.Sprintf("failed to marshal resources to JSON: %v", err)
		if err := r.Status().Update(ctx, backup); err != nil {
			logger.Error(err, "unable to update Backup status")
		}
		return ctrl.Result{}, fmt.Errorf("failed to marshal resources to JSON: %w", err)
	}

	objectName := fmt.Sprintf("%s.json", backup.Name)
	_, err = minioClient.PutObject(ctx, backup.Spec.StorageBucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})

	if err != nil {
		// Update status to Failed and set the error message.
		backup.Status.Status = v1alpha1.BackupStatusFailed
		backup.Status.Error = fmt.Sprintf("failed to upload to MinIO: %v", err)

		if updateErr := r.Status().Update(ctx, backup); updateErr != nil {
			logger.Error(updateErr, "unable to update Backup status after MinIO upload failure")
		}
		return ctrl.Result{}, fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	// Update status to Completed, set CompletionTime and BackupFileName.
	backup.Status.Status = v1alpha1.BackupStatusCompleted
	now := metav1.Now()
	backup.Status.CompletionTime = &now
	backup.Status.BackupFileName = objectName
	backup.Status.Error = ""

	if err := r.Status().Update(ctx, backup); err != nil {
		logger.Error(err, "unable to update Backup status")
		return ctrl.Result{}, err
	}

	logger.Info("Backup completed successfully", "backup", backup.Name, "file", objectName)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Backup{}).
		Complete(r)
}
