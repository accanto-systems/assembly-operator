package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AssemblySpec defines the desired state of Service
// +k8s:openapi-gen=true
type AssemblySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	AssemblyName   string            `json:"AssemblyName"`
	DescriptorName string            `json:"DescriptorName"`
	IntendedState  string            `json:"IntendedState"`
	Properties     map[string]string `json:"Properties"`
}

// AssemblyStatus defines the observed state of Service
// +k8s:openapi-gen=true
type AssemblyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	ProcessID string `json:"ProcessID"`
	Status    string `json:"Status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Assembly is the Schema for the services API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Assembly struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AssemblySpec   `json:"spec,omitempty"`
	Status AssemblyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AssemblyList contains a list of Service
type AssemblyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Assembly `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Assembly{}, &AssemblyList{})
}
