package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TargetKind matches a kind of workload a ComplianceTemplate applies to,
// optionally narrowed by labels.
type TargetKind struct {
	// Kind of the matched resource (e.g. Deployment, CronJob, StatefulSet).
	Kind string `json:"kind"`

	// MatchLabels narrows the match to objects carrying all of these labels.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// ResourceRule bounds a single resource quantity in a template.
type ResourceRule struct {
	// Min is the minimum sane value. Absent means "any positive value is fine".
	// +optional
	Min string `json:"min,omitempty"`

	// Required marks the resource as mandatory (presence, not just a value).
	// +optional
	Required bool `json:"required,omitempty"`
}

// ResourceRequirements groups the resource rules of a template.
type ResourceRequirements struct {
	// Limits bounds the upper bound of each resource.
	// +optional
	Limits map[string]ResourceRule `json:"limits,omitempty"`

	// Requests bounds the lower bound of each resource.
	// +optional
	Requests map[string]ResourceRule `json:"requests,omitempty"`
}

// ProbePolicy says whether a probe is mandatory.
type ProbePolicy string

const (
	ProbePolicyRequired ProbePolicy = "required"
	ProbePolicyOptional ProbePolicy = "optional"
)

// ImageTagPolicy says which image tags are acceptable.
type ImageTagPolicy string

const (
	ImageTagPolicyExplicit ImageTagPolicy = "explicit" // no :latest, digest ok
	ImageTagPolicyAny      ImageTagPolicy = "any"
)

// ProbeRequirements groups probe rules.
type ProbeRequirements struct {
	// +kubebuilder:validation:Enum=required;optional
	Liveness ProbePolicy `json:"liveness,omitempty"`
	// +kubebuilder:validation:Enum=required;optional
	Readiness ProbePolicy `json:"readiness,omitempty"`
}

// ImageRequirements governs image tags.
type ImageRequirements struct {
	// +kubebuilder:validation:Enum=explicit;any
	TagPolicy ImageTagPolicy `json:"tagPolicy,omitempty"`
}

// RequiredSpec is what every matching workload must satisfy.
type RequiredSpec struct {
	// Resources bounds CPU/memory/GPU (extended resources included).
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Probes mandate liveness/readiness.
	// +optional
	Probes *ProbeRequirements `json:"probes,omitempty"`

	// Image governs image tags.
	// +optional
	Image *ImageRequirements `json:"image,omitempty"`

	// ConcurrencyPolicy is only meaningful for CronJobs.
	// +optional
	ConcurrencyPolicy string `json:"concurrencyPolicy,omitempty"`
}

// DriftSpec controls what Umbra does on a violation.
type DriftSpec struct {
	// OnViolation: report (inform only), pr (open a PR), autofix (low-risk only).
	// +kubebuilder:validation:Enum=report;pr;autofix
	// +kubebuilder:default=report
	OnViolation string `json:"onViolation,omitempty"`

	// NotifySlack forwards violations to a webhook (P2).
	// +optional
	NotifySlack bool `json:"notifySlack,omitempty"`
}

// ComplianceTemplateSpec defines the desired state of ComplianceTemplate.
type ComplianceTemplateSpec struct {
	// AppliesTo selects the workloads this template governs.
	AppliesTo []TargetKind `json:"appliesTo"`

	// Required is what every matching workload must satisfy.
	// +optional
	Required RequiredSpec `json:"required,omitempty"`

	// Drift controls the reaction to a violation.
	// +optional
	Drift DriftSpec `json:"drift,omitempty"`
}

// ComplianceTemplateStatus defines the observed state.
type ComplianceTemplateStatus struct {
	// Conditions is the kubectl-style observable state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:rbac:groups=umbra.io,resources=compliancetemplates,verbs=get;list;watch
//+kubebuilder:rbac:groups=umbra.io,resources=compliancetemplates/status,verbs=get;update;patch

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// ComplianceTemplate is the Schema for the compliancetemplates API.
type ComplianceTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComplianceTemplateSpec   `json:"spec,omitempty"`
	Status ComplianceTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComplianceTemplateList contains a list of ComplianceTemplate.
type ComplianceTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComplianceTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComplianceTemplate{}, &ComplianceTemplateList{})
}
