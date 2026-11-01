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

// ApprovalVerdict says who really approved. The annotation value is a
// free-text claim: anyone allowed to patch the Finding can write any
// name, so a claim without corroboration is self-declared, never proven.
type ApprovalVerdict struct {
	Claimed  string // annotation value ("" when absent/invalid)
	Writer   string // managedFields manager that wrote it ("" unknown)
	Audit    string // audit-log actor ("" when no audit trail)
	Verified bool   // an audit person matching the claim
}

// ResolveApproval crosses the annotation claim with the writer client and
// the audit actor. Verified requires a non-system audit user equal to the
// claim; a person mismatch is recorded, never silently accepted.
func ResolveApproval(annotations map[string]string, writer, auditUser string) ApprovalVerdict {
	var v ApprovalVerdict
	claimed, ok := ParseApproval(annotations)
	if !ok {
		return v
	}
	v.Claimed = claimed
	v.Writer = writer
	person := auditUser != "" && !strings.HasPrefix(auditUser, "system:")
	if person {
		v.Audit = auditUser
		v.Verified = auditUser == claimed
	}
	return v
}

// Label renders the verdict for the Approved condition: verified names the
// corroborating actor, everything else says self-declared with the source.
func (v ApprovalVerdict) Label() string {
	if v.Claimed == "" {
		return "no approval"
	}
	if v.Verified {
		return "verificado: " + v.Audit
	}
	who := v.Writer
	if who == "" {
		who = "unknown client"
	}
	s := "autodeclarado: " + v.Claimed + " (via " + who + ")"
	if v.Audit != "" {
		s += " [discrepancia: audit dice " + v.Audit + "]"
	}
	return s
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
	// The reconciler sees managedFields, not the audit log: without an
	// audit actor every approval is self-declared by construction. The
	// label says so openly; ResolveApproval upgrades to verified only
	// when an audit person corroborates the claim.
	v := ResolveApproval(f.Annotations, via, "")
	meta.SetStatusCondition(&f.Status.Conditions, metav1.Condition{
		Type:   ConditionApproved,
		Status: metav1.ConditionTrue,
		Reason: "HumanApproval",
		Message: fmt.Sprintf("Approved by %s [%s]",
			evidence.Sanitize(user), evidence.Sanitize(v.Label())),
		ObservedGeneration: f.Generation,
	})
}
