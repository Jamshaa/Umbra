package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RemediationPolicyMode selects whether fixes apply automatically or wait
// for explicit human approval.
// +kubebuilder:validation:Enum=Auto;RequireApproval
type RemediationPolicyMode string

const (
	// RemediationPolicyModeAuto allows verified fixes to proceed without human input.
	RemediationPolicyModeAuto RemediationPolicyMode = "Auto"
	// RemediationPolicyModeRequireApproval always waits for human approval.
	RemediationPolicyModeRequireApproval RemediationPolicyMode = "RequireApproval"
)

// TargetSelector matches the attended object by namespace and workload identity.
type TargetSelector struct {
	// Namespace of the attended object. Empty matches all namespaces.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Kind of the attended object (e.g. Deployment). Empty matches all kinds.
	// +optional
	Kind string `json:"kind,omitempty"`

	// Name of the attended object. Empty matches all names.
	// +optional
	Name string `json:"name,omitempty"`
}

// EscalationSpec escalates a policy to human approval after N failed auto-fixes.
type EscalationSpec struct {
	// MaxAutoFixes is the number of failed auto-fixes after which the policy
	// automatically starts requiring human approval.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxAutoFixes int `json:"maxAutoFixes,omitempty"`
}

// RemediationPolicySpec defines the desired state of RemediationPolicy.
type RemediationPolicySpec struct {
	// TargetSelector matches the attended object by namespace/workload.
	TargetSelector TargetSelector `json:"targetSelector"`

	// Mode selects automatic fixing or mandatory human approval.
	// +kubebuilder:validation:Enum=Auto;RequireApproval
	Mode RemediationPolicyMode `json:"mode"`

	// Escalation configures automatic escalation to human approval.
	// +optional
	Escalation EscalationSpec `json:"escalation,omitempty"`
}

// RemediationPolicyStatus defines the observed state of RemediationPolicy.
type RemediationPolicyStatus struct {
	// Conditions is the kubectl-style observed state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:scope:Cluster
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:rbac:groups=umbra.io,resources=remediationpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=umbra.io,resources=remediationpolicies/status,verbs=update;patch
// +kubebuilder:rbac:groups=umbra.io,resources=remediationpolicies/finalizers,verbs=update

// RemediationPolicy is the Schema for the remediationpolicies API.
type RemediationPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of RemediationPolicy.
	// +optional
	Spec RemediationPolicySpec `json:"spec,omitempty"`

	// status defines the observed state of RemediationPolicy.
	// +optional
	Status RemediationPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RemediationPolicyList contains a list of RemediationPolicy.
type RemediationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemediationPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemediationPolicy{}, &RemediationPolicyList{})
}
