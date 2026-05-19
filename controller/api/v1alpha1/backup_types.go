package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSpec defines the desired state of Backup
type BackupSpec struct {
    // Message to be logged by the operator
    // +kubebuilder:validation:MinLength=1
    Message string `json:"message"`
}

// BackupStatus defines the observed state of Backup
type BackupStatus struct {
    // Indicates if the backup was processed
    Processed bool `json:"processed"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Backup",type="string",JSONPath=".spec.message"
//+kubebuilder:printcolumn:name="Processed",type="boolean",JSONPath=".status.processed"

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