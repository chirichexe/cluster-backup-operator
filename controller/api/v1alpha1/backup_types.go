package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Status string

const (
	BackupStatusCompleted  Status = "Completed"
	BackupStatusFailed     Status = "Failed"
	BackupStatusInProgress Status = "InProgress"
)

// Resource defines a Kubernetes resource with its name, group and version
type Resource struct {
	// Name is the name of the Kubernetes resource
	Name string `json:"name"`
	// Group is the API group the resource belongs to.
	// For resources in the core API group (e.g., "pods", "services"), leave this empty.
	Group string `json:"group,omitempty"`
	// Version is the API version of the resource.
	Version string `json:"version,omitempty"`
}

// BackupSpec defines the desired state of Backup
type BackupSpec struct {
	// Resources is a list of Kubernetes resources to include in the backup
	Resources []Resource `json:"resources"`
	// StorageBucket is the name of the MinIO bucket to store backups.
	StorageBucket string `json:"storageBucket"`
}

// BackupStatus defines the observed state of Backup
type BackupStatus struct {
	// BackupFileName is the name of the backup file in MinIO.
	BackupFileName string `json:"backupFileName,omitempty"`
	// CompletionTime is the time at which the backup was completed.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// +kubebuilder:validation:Enum=Completed;Failed;InProgress
	// Status of the backup.  Can be "Completed", "Failed", or "InProgress".
	Status Status `json:"status,omitempty"`
	// Error message if the backup failed.
	Error string `json:"error,omitempty"`
	// StartTime is the time at which the backup was started.
	StartTime *metav1.Time `json:"startTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Backup is the Schema for the backups API
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}
