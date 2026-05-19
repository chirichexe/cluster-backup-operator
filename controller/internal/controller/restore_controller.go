package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/chirichexe/cluster-backup-operator/api/v1alpha1"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// +kubebuilder:rbac:groups=operators.com,resources=restores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.com,resources=restores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operators.com,resources=restores/finalizers,verbs=update
// +kubebuilder:rbac:groups=operators.com,resources=backups,verbs=get;list;watch  // Need to read Backups!
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch // Needed to read secrets!
// +kubebuilder:rbac:groups=*,resources=*,verbs=create;update;patch;delete  // Broad permissions for applying resources
// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *RestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting reconciliation", "request", req.NamespacedName)

	// Fetch the Restore instance
	restore := &v1alpha1.Restore{}
	err := r.Get(ctx, req.NamespacedName, restore)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Skip if already finished — prevents re-running on every reconciliation
	if restore.Status.Status == v1alpha1.RestoreStatusCompleted || restore.Status.Status == v1alpha1.RestoreStatusFailed {
		logger.Info("Restore already finished, skipping", "status", restore.Status.Status)
		return ctrl.Result{}, nil
	}

	// Initialize MinIO client
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin123", ""),
		Secure: false,
	})

	if err != nil {
		restore.Status.Status = v1alpha1.RestoreStatusFailed
		restore.Status.Message = fmt.Sprintf("Failed to connect to MinIO: %v", err)
		r.Status().Update(ctx, restore)
		return ctrl.Result{}, err
	}

	// Set restore to running state
	restore.Status.Status = v1alpha1.RestoreStatusInProgress
	r.Status().Update(ctx, restore)

	// Download and apply from MinIO
	object, err := minioClient.GetObject(ctx, restore.Spec.StorageBucket, fmt.Sprintf("%s.json", restore.Spec.BackupName), minio.GetObjectOptions{})
	if err != nil {
		restore.Status.Status = v1alpha1.RestoreStatusFailed
		restore.Status.Message = "Error getting backup files from MinIO"
		r.Status().Update(ctx, restore)
		return ctrl.Result{}, fmt.Errorf("failed to get object from MinIO: %w", err)
	}
	defer object.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(object); err != nil {
		restore.Status.Status = v1alpha1.RestoreStatusFailed
		restore.Status.Message = fmt.Sprintf("Error reading backup data: %v", err)
		r.Status().Update(ctx, restore)
		return ctrl.Result{}, fmt.Errorf("failed to read backup object: %w", err)
	}

	items := make([]unstructured.Unstructured, 0)
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		restore.Status.Status = v1alpha1.RestoreStatusFailed
		restore.Status.Message = fmt.Sprintf("Error parsing backup data: %v", err)
		r.Status().Update(ctx, restore)
		return ctrl.Result{}, fmt.Errorf("failed to unmarshal backup: %w", err)
	}

	for _, obj := range items {
		//Try to create, if already exists then update.
		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(obj.GroupVersionKind())
		err = r.Client.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)

		if err != nil && errors.IsNotFound(err) {
			err = r.Client.Create(ctx, &obj)
			if err != nil {
				restore.Status.Status = v1alpha1.RestoreStatusFailed
				restore.Status.Message = "Error creating resource"
				r.Status().Update(ctx, restore)
				return ctrl.Result{}, fmt.Errorf("failed to create resource: %w", err)
			}
		} else if err == nil {
			obj.SetResourceVersion(existing.GetResourceVersion())
			err = r.Client.Update(ctx, &obj)

			if err != nil {
				restore.Status.Status = v1alpha1.RestoreStatusFailed
				restore.Status.Message = "Error updating resource"
				r.Status().Update(ctx, restore)
				return ctrl.Result{}, fmt.Errorf("failed to update resource: %w", err)
			}
		} else {
			restore.Status.Status = v1alpha1.RestoreStatusFailed
			restore.Status.Message = "Error getting resource"
			r.Status().Update(ctx, restore)
			return ctrl.Result{}, fmt.Errorf("failed to get resource: %w", err)
		}
	}

	restore.Status.Status = v1alpha1.RestoreStatusCompleted
	restore.Status.Message = "Restore completed successfully."
	r.Status().Update(ctx, restore)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Restore{}).
		Complete(r)
}
