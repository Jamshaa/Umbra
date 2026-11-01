package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
)

func TestParseApproval(t *testing.T) {
	if u, ok := ParseApproval(map[string]string{AnnotationApproved: "alice"}); !ok || u != "alice" {
		t.Fatalf("valid approval rejected: %q %v", u, ok)
	}
	for _, tc := range []map[string]string{
		nil,
		{},
		{AnnotationApproved: ""},
		{AnnotationApproved: "   "},
		{AnnotationApproved: "alice bob"},
		{"other.io/key": "alice"},
	} {
		if u, ok := ParseApproval(tc); ok {
			t.Errorf("invalid approval accepted: %q -> %q", tc, u)
		}
	}
}

func TestApprovalAllowedStates(t *testing.T) {
	allowed := map[v1alpha1.FindingState]bool{
		v1alpha1.FindingStateDetected:    false,
		v1alpha1.FindingStateAnalyzing:   false,
		v1alpha1.FindingStateFixProposed: true,
		v1alpha1.FindingStateVerified:    true,
		v1alpha1.FindingStateFailed:      false,
		v1alpha1.FindingStateReverted:    false,
		v1alpha1.FindingState(""):        false,
	}
	for state, want := range allowed {
		if got := approvalAllowed(state); got != want {
			t.Errorf("approvalAllowed(%q) = %v, want %v", state, got, want)
		}
	}
}

// approvalManagedFields crafts an Update entry claiming manager wrote the
// approval annotation, exactly as the API server records kubectl apply.
func approvalManagedFields(manager string) []metav1.ManagedFieldsEntry {
	ts := metav1.NewTime(time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC))
	return []metav1.ManagedFieldsEntry{{
		Manager:    manager,
		Operation:  metav1.ManagedFieldsOperationUpdate,
		APIVersion: "v1",
		Time:       &ts,
		FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(
			`{"f:metadata":{"f:annotations":{"f:umbra.dev/approved":{}}}}`)},
	}}
}

func TestApprovalWriter(t *testing.T) {
	mk := func(mf []metav1.ManagedFieldsEntry) *v1alpha1.Finding {
		f := seedFinding("f-attr", v1alpha1.FindingStateVerified, "Verified")
		f.Annotations[AnnotationApproved] = "alice"
		f.ManagedFields = mf
		return f
	}
	if got := approvalWriter(mk(approvalManagedFields("kubectl"))); got != "kubectl" {
		t.Fatalf("writer = %q, want kubectl", got)
	}
	if got := approvalWriter(mk(nil)); got != "" {
		t.Fatalf("no managedFields must yield no writer, got %q", got)
	}
	other := approvalManagedFields("argocd")
	other[0].FieldsV1 = &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:annotations":{"f:other.io/key":{}}}}`)}
	if got := approvalWriter(mk(other)); got != "" {
		t.Fatalf("unrelated annotation must not attribute, got %q", got)
	}
	statusOnly := approvalManagedFields("manager")
	statusOnly[0].Subresource = "status"
	if got := approvalWriter(mk(statusOnly)); got != "" {
		t.Fatalf("status subresource must be ignored, got %q", got)
	}
}

func approveCond(t *testing.T, f *v1alpha1.Finding) *metav1.Condition {
	t.Helper()
	for i := range f.Status.Conditions {
		if f.Status.Conditions[i].Type == ConditionApproved {
			return &f.Status.Conditions[i]
		}
	}
	return nil
}

func TestReconcileRecordsApproval(t *testing.T) {
	s := testScheme(t)
	f := seedFinding("f-ok", v1alpha1.FindingStateVerified, "Verified")
	f.Annotations[AnnotationApproved] = "alice"
	f.ManagedFields = approvalManagedFields("kubectl")
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(f).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()

	reconcile(t, c, "f-ok")
	got := getFinding(t, c, "f-ok")
	cond := approveCond(t, got)
	if cond == nil {
		t.Fatal("no Approved condition after human approval")
	}
	if cond.Status != metav1.ConditionTrue || cond.Reason != "HumanApproval" {
		t.Fatalf("condition = %+v", cond)
	}
	wantPrefix := "Approved by alice [autodeclarado: alice (via "
	if cond.Message != wantPrefix+"kubectl)]" && cond.Message != wantPrefix+"unknown client)]" {
		t.Fatalf("message = %q, want %q<manager>)]", cond.Message, wantPrefix)
	}

	// Unknown writer: recorded honestly, never invented.
	f2 := seedFinding("f-anon", v1alpha1.FindingStateFixProposed, "FixProposed")
	f2.Annotations[AnnotationApproved] = "bob"
	c2 := fake.NewClientBuilder().WithScheme(s).WithObjects(f2).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()
	reconcile(t, c2, "f-anon")
	got2 := getFinding(t, c2, "f-anon")
	cond2 := approveCond(t, got2)
	if cond2 == nil || cond2.Status != metav1.ConditionTrue {
		t.Fatalf("expected approval recorded, got %+v", cond2)
	}
	if want := "Approved by bob [autodeclarado: bob (via unknown client)]"; cond2.Message != want {
		t.Fatalf("message = %q, want %q", cond2.Message, want)
	}
}

func TestResolveApprovalVerdict(t *testing.T) {
	ann := map[string]string{AnnotationApproved: "alice"}
	// Audit person matching the claim: verified.
	if v := ResolveApproval(ann, "kubectl", "alice"); !v.Verified || v.Label() != "verificado: alice" {
		t.Fatalf("verified case: %+v", v)
	}
	// No audit: self-declared, writer named, never invented.
	if v := ResolveApproval(ann, "kubectl", ""); v.Verified || v.Label() != "autodeclarado: alice (via kubectl)" {
		t.Fatalf("no-audit case: %+v", v)
	}
	if v := ResolveApproval(ann, "", ""); v.Label() != "autodeclarado: alice (via unknown client)" {
		t.Fatalf("unknown writer case: %+v", v)
	}
	// Audit names another person: mismatch recorded, not verified.
	v := ResolveApproval(ann, "kubectl", "mallory")
	if v.Verified || v.Audit != "mallory" {
		t.Fatalf("mismatch must not verify: %+v", v)
	}
	if want := "autodeclarado: alice (via kubectl) [discrepancia: audit dice mallory]"; v.Label() != want {
		t.Fatalf("label = %q, want %q", v.Label(), want)
	}
	// System accounts are machinery, not people: never verify.
	if v := ResolveApproval(ann, "kubectl", "system:serviceaccount:kube-system:x"); v.Verified || v.Audit != "" {
		t.Fatalf("system account must not verify: %+v", v)
	}
	// Invalid annotation: no verdict at all.
	if v := ResolveApproval(map[string]string{AnnotationApproved: "a b"}, "kubectl", "alice"); v.Claimed != "" {
		t.Fatalf("invalid claim must yield nothing: %+v", v)
	}
}

func TestReconcileApprovalWaitsForEligibleState(t *testing.T) {
	s := testScheme(t)
	for _, state := range []v1alpha1.FindingState{
		v1alpha1.FindingStateDetected,
		v1alpha1.FindingStateAnalyzing,
		v1alpha1.FindingStateFailed,
		v1alpha1.FindingStateReverted,
	} {
		f := seedFinding("f-wait", state, string(state))
		f.Annotations[AnnotationApproved] = "alice"
		c := fake.NewClientBuilder().WithScheme(s).WithObjects(f).
			WithStatusSubresource(&v1alpha1.Finding{}).Build()
		reconcile(t, c, "f-wait")
		got := getFinding(t, c, "f-wait")
		cond := approveCond(t, got)
		if cond == nil {
			t.Fatalf("state %q: approval annotation silently dropped, want recorded refusal", state)
		}
		if cond.Status != metav1.ConditionFalse || cond.Reason != "WaitingOnVerification" {
			t.Fatalf("state %q: condition = %+v, want False/WaitingOnVerification", state, cond)
		}
	}
}

func TestReconcileApprovalRevoked(t *testing.T) {
	s := testScheme(t)
	f := seedFinding("f-revoke", v1alpha1.FindingStateVerified, "Verified")
	f.Annotations[AnnotationApproved] = "alice"
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(f).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()
	reconcile(t, c, "f-revoke")
	if approveCond(t, getFinding(t, c, "f-revoke")) == nil {
		t.Fatal("setup: expected Approved condition")
	}

	// Human removes the annotation: the record must go, not linger.
	cur := getFinding(t, c, "f-revoke")
	delete(cur.Annotations, AnnotationApproved)
	if err := c.Update(context.Background(), cur); err != nil {
		t.Fatal(err)
	}
	reconcile(t, c, "f-revoke")
	if cond := approveCond(t, getFinding(t, c, "f-revoke")); cond != nil {
		t.Fatalf("revoked approval still recorded: %+v", cond)
	}
}

func TestReconcileApprovalIdempotent(t *testing.T) {
	s := testScheme(t)
	f := seedFinding("f-idem", v1alpha1.FindingStateVerified, "Verified")
	f.Annotations[AnnotationApproved] = "alice"
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(f).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()
	reconcile(t, c, "f-idem")
	first := getFinding(t, c, "f-idem")
	reconcile(t, c, "f-idem")
	second := getFinding(t, c, "f-idem")
	c1, c2 := approveCond(t, first), approveCond(t, second)
	if c1 == nil || c2 == nil {
		t.Fatal("Approved condition missing after reconcile")
	}
	if *c1 != *c2 {
		t.Fatalf("second reconcile changed the record:\n%+v\n%+v", c1, c2)
	}
}
