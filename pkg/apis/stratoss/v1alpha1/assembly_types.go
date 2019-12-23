package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "operator-sdk generate k8s" and "operator-sdk generate crds" to regenerate code after modifying this file
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
// Comments above types and field names will be used as "description" values in the generated CRD

type assemblyStates struct {
	Failed    string
	Created   string
	Installed string
	Inactive  string
	Broken    string
	Active    string
	NotFound  string
}

var AssemblyStates = &assemblyStates{
	Failed:    "Failed",
	Created:   "Created",
	Installed: "Installed",
	Inactive:  "Inactive",
	Broken:    "Broken",
	Active:    "Active",
	NotFound:  "NotFound",
}

type syncStates struct {
	Error string
	OK    string
}

var SyncStates = &syncStates{
	Error: "Error",
	OK:    "OK",
}

type processStatus struct {
	Planned    string
	Pending    string
	InProgress string
	Completed  string
	Cancelled  string
	Failed     string
}

var ProcessStatus = &processStatus{
	Planned:    "Planned",
	Pending:    "Pending",
	InProgress: "In Progress",
	Completed:  "Completed",
	Cancelled:  "Cancelled",
	Failed:     "Failed",
}

// AssemblySpec defines the desired state of Assembly
// +k8s:openapi-gen=true
type AssemblySpec struct {
	// The descriptor name from which this Assembly will be modelled (in the form of "assembly::<name>::<version>")
	DescriptorName string `json:"descriptorName"`
	// The final intended state that the Assembly should be in
	IntendedState string `json:"intendedState"`
	// An optional map of name and string value properties supplied to configure the Assembly (valid values are properties defined on the descriptor in use)
	Properties map[string]string `json:"properties"`
}

// AssemblyStatus defines the observed state of Assembly
// +k8s:openapi-gen=true
type AssemblyStatus struct {
	// ID of the Assembly
	ID string `json:"assemblyId"`
	// The current descriptor name from which this Assembly was modelled (in the form of "assembly::<name>::<version>")
	DescriptorName string `json:"descriptorName"`
	// An optional map of name and string value properties supplied to configure the Assembly (valid values are properties defined on the descriptor in use)
	Properties map[string]string `json:"properties"`
	// State of the Assembly at last reconcile
	// +kubebuilder:validation:Enum=Failed;Created;Installed;Inactive;Broken;Active;NotFound;None;
	State string `json:"state"`
	// Details of the last process triggered by the operator on an Assembly
	LastProcess Process `json:"lastProcess,omitempty"`
	// Details the success to synchronize this Assembly with LM
	SyncState SyncState `json:"syncState,omitempty"`
}

// Details the success to synchronize this Assembly with LM
// +k8s:openapi-gen=true
type SyncState struct {
	// Status of synchronize (has there been an error?)
	Status string `json:"status"`
	// Error message
	Error string `json:"error"`
	// Number of times this error has led to a retry
	Attempts int `json:"attempts"`
}

// Details an Assembly process
// +k8s:openapi-gen=true
type Process struct {
	// ID of the process
	ID string `json:"processId"`
	// Type of process
	// +kubebuilder:validation:Enum=Create;ChangeState;Update;Delete;None;
	IntentType string `json:"intentType"`
	// Status of the process
	// +kubebuilder:validation:Enum=Planned;Pending;In Progress;Completed;Cancelled;Failed;None;
	Status string `json:"status"`
	// Describes the reason of the Status, usually only set when Failed
	StatusReason string `json:"statusReason"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Assembly is the Schema for the assemblies API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=assemblies,scope=Namespaced
// +kubebuilder:printcolumn:JSONPath=".status.syncState.status",name=Synchronized,type=string,description=Details if the operator was able to synchronize this Assembly when handling the last event
// +kubebuilder:printcolumn:JSONPath=".status.descriptorName",name=Descriptor,type=string,description=The current observed Descriptor of the Assembly
// +kubebuilder:printcolumn:JSONPath=".status.state",name=State,type=string,description=The current observed State of the Assembly
// +kubebuilder:printcolumn:JSONPath=".status.lastProcess.intentType",name=LastProcess,type=string,description=The last observed Process type
// +kubebuilder:printcolumn:JSONPath=".status.lastProcess.status",name=ProcessStatus,type=string,description=The last observed Process status
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date,description=The amount of time this Assembly has existed for
type Assembly struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AssemblySpec   `json:"spec,omitempty"`
	Status AssemblyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AssemblyList contains a list of Assembly
type AssemblyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Assembly `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Assembly{}, &AssemblyList{})
}
