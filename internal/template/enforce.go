package template

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/evidence"
	"github.com/Jamshaa/Umbra/internal/report"
)

// Action is the drift decision for one assessment. It mirrors
// DriftSpec.OnViolation; "autofix" is granted only when every violation
// is low-risk, otherwise the assessment is downgraded to "pr".
type Action string

const (
	ActionReport  Action = "report"
	ActionPR      Action = "pr"
	ActionAutofix Action = "autofix"
)

// lowRiskFields are the only violation top-level fields autofix may
// touch. Resources and probes are bounded and reversible; image tags
// and scheduling policy can break a rollout, so they always escalate
// to a human-reviewed PR.
var lowRiskFields = map[string]bool{
	"resources":      true,
	"livenessProbe":  true,
	"readinessProbe": true,
}

// AttributedBy records who the drift is attributed to, with the T6
// confidence label (person | client | unknown). Never present a client
// as a person: the label travels with the name.
type AttributedBy struct {
	By         string
	Confidence string
}

// String renders "name (confidence)", or "unknown" when empty.
func (a AttributedBy) String() string {
	if a.By == "" {
		return "unknown"
	}
	if a.Confidence == "" {
		return a.By
	}
	return a.By + " (" + a.Confidence + ")"
}

// Assessment is one enforced object: a Finding carrying the template
// diff plus the drift action. Ts is left zero on purpose so output is
// deterministic; the writer stamps the time.
type Assessment struct {
	Finding v1alpha1.Finding
	Action  Action
}

// topField returns the segment before the first dot ("resources.limits"
// -> "resources").
func topField(field string) string {
	if i := strings.Index(field, "."); i >= 0 {
		return field[:i]
	}
	return field
}

// findingMeta builds a deterministic ObjectMeta for the drift finding:
// "tpl-<template>-<kind>-<name>", lowercased and DNS-safe. Findings are
// cluster-scoped, so no namespace is set.
func findingMeta(tplName, kind, name string) metav1.ObjectMeta {
	raw := strings.ToLower("tpl-" + tplName + "-" + kind + "-" + name)
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 253 {
		out = out[:253]
	}
	if out == "" {
		out = "tpl-drift"
	}
	return metav1.ObjectMeta{Name: out}
}

// Enforce evaluates one object against the template and builds the
// Finding for it. It returns ok=false when nothing violates the
// template. The action follows DriftSpec.OnViolation (default report),
// downgraded from autofix to pr unless every violation is low-risk.
func Enforce(tpl v1alpha1.ComplianceTemplate, obj map[string]any, attr AttributedBy) (Assessment, bool) {
	vs := Evaluate(tpl, []map[string]any{obj})
	if len(vs) == 0 {
		return Assessment{}, false
	}
	meta, _ := obj["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	ns, _ := meta["namespace"].(string)
	kind, _ := obj["kind"].(string)

	var b strings.Builder
	fmt.Fprintf(&b, "template %s: %d violation(s)\n", tpl.Name, len(vs))
	for _, v := range vs {
		fmt.Fprintf(&b, "- %s: %s -> %s (%s)\n", v.Field, v.Actual, v.Expected, v.Message)
	}
	fmt.Fprintf(&b, "Attributed: %s\n", attr.String())
	diag := strings.TrimSuffix(b.String(), "\n")

	args := tpl.Name
	result := fmt.Sprintf("%d violation(s)", len(vs))
	entry := v1alpha1.TimelineEntry{
		Tool:         "template-evaluate",
		Args:         args,
		Result:       result,
		EvidenceHash: evidence.Fingerprint(args + "\n" + result),
	}

	action := Action(tpl.Spec.Drift.OnViolation)
	if action == "" {
		action = ActionReport
	}
	if action != ActionReport && action != ActionPR && action != ActionAutofix {
		action = ActionReport
	}
	if action == ActionAutofix {
		for _, v := range vs {
			if !lowRiskFields[topField(v.Field)] {
				action = ActionPR
				break
			}
		}
	}
	f := v1alpha1.Finding{
		ObjectMeta: findingMeta(tpl.Name, kind, name),
		Spec: v1alpha1.FindingSpec{
			Severity:  v1alpha1.FindingSeverityMedium,
			Category:  v1alpha1.FindingCategoryConfig,
			TargetRef: v1alpha1.TargetRef{Kind: kind, Namespace: ns, Name: name},
			Detector:  "template",
			Diagnosis: diag,
		},
		Status: v1alpha1.FindingStatus{Timeline: []v1alpha1.TimelineEntry{entry}},
	}
	return Assessment{Finding: f, Action: action}, true
}

// ResultsForReport converts violations into PolicyReport results for
// the template's PolicyReport document. Every violation is a fail:
// the template states what must hold, silence means compliance.
func ResultsForReport(policy string, kind, ns, name string, vs []Violation) []report.Result {
	out := make([]report.Result, 0, len(vs))
	for _, v := range vs {
		out = append(out, report.Result{
			Policy:   policy,
			Rule:     v.Field,
			Outcome:  report.Fail,
			Message:  v.Message,
			Resource: kind + "/" + ns + "/" + name,
		})
	}
	return out
}
