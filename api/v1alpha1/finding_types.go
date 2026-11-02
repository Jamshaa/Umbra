package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FindingSeverity is the severity of a Finding.
// +kubebuilder:validation:Enum=High;Medium;Low
type FindingSeverity string

const (
	// FindingSeverityHigh marks findings that need immediate attention.
	FindingSeverityHigh FindingSeverity = "High"
	// FindingSeverityMedium marks findings that should be fixed soon.
	FindingSeverityMedium FindingSeverity = "Medium"
	// FindingSeverityLow marks findings that are informational or minor.
	FindingSeverityLow FindingSeverity = "Low"
)

// FindingCategory is the fault category of a Finding.
// +kubebuilder:validation:Enum=Resources;Reliability;Security;RBAC;Config
type FindingCategory string

const (
	// FindingCategoryResources groups resource request/limit faults.
	FindingCategoryResources FindingCategory = "Resources"
	// FindingCategoryReliability groups probe and restart faults.
	FindingCategoryReliability FindingCategory = "Reliability"
	// FindingCategorySecurity groups container hardening faults.
	FindingCategorySecurity FindingCategory = "Security"
	// FindingCategoryRBAC groups permission faults.
	FindingCategoryRBAC FindingCategory = "RBAC"
	// FindingCategoryConfig groups image and reference faults.
	FindingCategoryConfig FindingCategory = "Config"
)

// FindingState is the lifecycle state of a Finding.
// +kubebuilder:validation:Enum=Detected;Analyzing;FixProposed;Verified;Failed;Reverted
type FindingState string

const (
	// FindingStateDetected marks a newly detected finding.
	FindingStateDetected FindingState = "Detected"
	// FindingStateAnalyzing marks a finding under diagnosis.
	FindingStateAnalyzing FindingState = "Analyzing"
	// FindingStateFixProposed marks a finding with a proposed patch.
	FindingStateFixProposed FindingState = "FixProposed"
	// FindingStateVerified marks a finding whose fix passed verification.
	FindingStateVerified FindingState = "Verified"
	// FindingStateFailed marks a finding whose fix failed verification.
	FindingStateFailed FindingState = "Failed"
	// FindingStateReverted marks a finding rolled back to its snapshot.
	FindingStateReverted FindingState = "Reverted"
)

// TargetRef identifies the observed resource.
type TargetRef struct {
	// Kind of the observed resource.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[A-Z][A-Za-z0-9]*$`
	Kind string `json:"kind"`

	// Namespace of the observed resource.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Namespace string `json:"namespace,omitempty"`

	// Name of the observed resource.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name"`
}

// TimelineEntry records one tool call applied to a Finding.
type TimelineEntry struct {
	// Tool is the harness-owned tool that was executed.
	Tool string `json:"tool"`

	// Args are the tool arguments.
	// +optional
	Args string `json:"args,omitempty"`

	// Result is the tool observation.
	// +optional
	Result string `json:"result,omitempty"`

	// ModelVersion is the version of the model (or tooling) that produced
	// the entry. Empty for deterministic tools. Audit only.
	// +optional
	ModelVersion string `json:"modelVersion,omitempty"`

	// EvidenceHash is the lowercase hex sha256 over the sanitized
	// "args + \n + result" actually stored in this entry. It lets an
	// auditor prove the entry was not altered after the fact.
	// +optional
	EvidenceHash string `json:"evidenceHash,omitempty"`

	// Ts is the epoch-seconds timestamp of the tool call.
	Ts int64 `json:"ts"`
}

// FindingSpec defines the desired state of Finding.
type FindingSpec struct {
	// Severity of the finding.
	// +kubebuilder:validation:Enum=High;Medium;Low
	Severity FindingSeverity `json:"severity"`

	// Category of the finding.
	// +kubebuilder:validation:Enum=Resources;Reliability;Security;RBAC;Config
	Category FindingCategory `json:"category"`

	// TargetRef identifies the observed resource.
	TargetRef TargetRef `json:"targetRef"`

	// Detector is the short name of the component that detected the finding.
	Detector string `json:"detector"`

	// Diagnosis is the optional LLM diagnostic text.
	// +optional
	// +kubebuilder:validation:MaxLength=262144
	Diagnosis string `json:"diagnosis,omitempty"`

	// ProposedPatch is the optional candidate fix as YAML.
	// +optional
	// +kubebuilder:validation:MaxLength=262144
	ProposedPatch string `json:"proposedPatch,omitempty"`

	// PrePatchManifest is the optional pre-patch snapshot as YAML, for rollback.
	// +optional
	// +kubebuilder:validation:MaxLength=262144
	PrePatchManifest string `json:"prePatchManifest,omitempty"`

	// State is the lifecycle state of the finding.
	// +kubebuilder:validation:Enum=Detected;Analyzing;FixProposed;Verified;Failed;Reverted
	State FindingState `json:"state"`
}

// FindingStatus defines the observed state of Finding.
type FindingStatus struct {
	// Timeline records the tool calls applied to the finding.
	// +optional
	Timeline []TimelineEntry `json:"timeline,omitempty"`

	// Conditions is the kubectl-style observed state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:scope:Cluster
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:rbac:groups=umbra.io,resources=findings,verbs=get;list;watch
// +kubebuilder:rbac:groups=umbra.io,resources=findings/status,verbs=update;patch
// +kubebuilder:rbac:groups=umbra.io,resources=findings/finalizers,verbs=update

// Finding is the Schema for the findings API.
type Finding struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of Finding.
	// +optional
	Spec FindingSpec `json:"spec,omitempty"`

	// status defines the observed state of Finding.
	// +optional
	Status FindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FindingList contains a list of Finding.
type FindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Finding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Finding{}, &FindingList{})
}
