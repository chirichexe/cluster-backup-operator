package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RestoreStatusCompleted  Status = "Completed"
	RestoreStatusFailed     Status = "Failed"
	RestoreStatusInProgress Status = "InProgress"
)

// RestoreSpec defines the desired state of Restore
type RestoreSpec struct {
	BackupName string `json:"backupName"`
	// StorageBucket is the name of the MinIO bucket to store backups.
	StorageBucket string `json:"storageBucket"`
}

// RestoreStatus defines the observed state of Restore
type RestoreStatus struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`

// Restore is the Schema for the restores API
type Restore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreSpec   `json:"spec,omitempty"`
	Status RestoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RestoreList contains a list of Restore
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Restore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Restore{}, &RestoreList{})
}
