package controller

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/attribution"
	"github.com/Jamshaa/Umbra/internal/evidence"
)

const (
	// AnnotationApproved is the human approval gate. A human writes it,
	// never the operator:
	//
	//   kubectl annotate finding <id> umbra.dev/approved=<usuario>
	//
	// The value names who approves. The reconciler records the approval
	// (who + when) in the Approved condition; it never acts on it alone.
	AnnotationApproved = "umbra.dev/approved"
	// ConditionApproved mirrors the human gate in status.conditions.
	ConditionApproved = "Approved"
)

// ParseApproval extracts the approver from annotations. Empty or
// whitespace-containing values are rejected: usernames never contain
// spaces, so those are mistakes, not identities.
func ParseApproval(annotations map[string]string) (string, bool) {
	v, ok := annotations[AnnotationApproved]
	if !ok {
		return "", false
	}
	v = strings.TrimSpace(v)
	if v == "" || strings.ContainsAny(v, " \t\n\r") {
		return "", false
	}
	return v, true
}

// approvalAllowed reports whether the finding's state may carry an
// approval. Approval expresses intent to ship; it is only meaningful
// once a patch exists (FixProposed) or is verified (Verified).
func approvalAllowed(state v1alpha1.FindingState) bool {
	return state == v1alpha1.FindingStateFixProposed ||
		state == v1alpha1.FindingStateVerified
}

// approvalWriter names the client that last wrote the approval
// annotation, from managedFields. Empty when unknown: callers must say
// "unknown", never invent a writer, and never present a client as a person.
func approvalWriter(f *v1alpha1.Finding) string {
	raw, err := json.Marshal(f)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	for _, a := range attribution.Attribute(m) {
		if a.Field == "metadata.annotations."+AnnotationApproved {
			return a.Manager
		}
	}
	return ""
}

// applyApprovalCondition mirrors the human gate in status.conditions.
// It only reads the annotation; the controller never writes annotations
// itself, so approving never triggers a metadata write loop.
// Who: the annotation value. When: the condition's LastTransitionTime.
func applyApprovalCondition(f *v1alpha1.Finding) {
	user, ok := ParseApproval(f.Annotations)
	if !ok {
		// No (valid) approval: revoke any stale record.
		meta.RemoveStatusCondition(&f.Status.Conditions, ConditionApproved)
		return
	}
	if !approvalAllowed(f.Spec.State) {
		meta.SetStatusCondition(&f.Status.Conditions, metav1.Condition{
			Type:   ConditionApproved,
			Status: metav1.ConditionFalse,
			Reason: "WaitingOnVerification",
			Message: fmt.Sprintf("approval by %s recorded; awaiting FixProposed or Verified state",
				evidence.Sanitize(user)),
			ObservedGeneration: f.Generation,
		})
		return
	}
	via := approvalWriter(f)
	if via == "" {
		via = "unknown client"
	}
	meta.SetStatusCondition(&f.Status.Conditions, metav1.Condition{
		Type:   ConditionApproved,
		Status: metav1.ConditionTrue,
		Reason: "HumanApproval",
		Message: fmt.Sprintf("Approved by %s (via %s)",
			evidence.Sanitize(user), evidence.Sanitize(via)),
		ObservedGeneration: f.Generation,
	})
}
