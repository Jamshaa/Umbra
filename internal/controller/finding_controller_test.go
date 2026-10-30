package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/evidence"
)

var states = []v1alpha1.FindingState{
	v1alpha1.FindingStateDetected,
	v1alpha1.FindingStateAnalyzing,
	v1alpha1.FindingStateFixProposed,
	v1alpha1.FindingStateVerified,
	v1alpha1.FindingStateFailed,
	v1alpha1.FindingStateReverted,
}

func validMoves() map[v1alpha1.FindingState]map[v1alpha1.FindingState]bool {
	allow := map[v1alpha1.FindingState]map[v1alpha1.FindingState]bool{}
	add := func(from, to v1alpha1.FindingState) {
		if allow[from] == nil {
			allow[from] = map[v1alpha1.FindingState]bool{}
		}
		allow[from][to] = true
	}
	add("", v1alpha1.FindingStateDetected)
	for _, s := range states {
		add(s, s) // re-reconcile is a no-op, never a violation
	}
	add(v1alpha1.FindingStateDetected, v1alpha1.FindingStateAnalyzing)
	add(v1alpha1.FindingStateAnalyzing, v1alpha1.FindingStateFixProposed)
	add(v1alpha1.FindingStateAnalyzing, v1alpha1.FindingStateFailed)
	add(v1alpha1.FindingStateFixProposed, v1alpha1.FindingStateVerified)
	add(v1alpha1.FindingStateFixProposed, v1alpha1.FindingStateFailed)
	add(v1alpha1.FindingStateFailed, v1alpha1.FindingStateReverted)
	return allow
}

// TestIsValidTransition covers every valid and invalid move of the
// state machine, including the empty (no recorded state) origin.
func TestIsValidTransition(t *testing.T) {
	allow := validMoves()
	froms := append([]v1alpha1.FindingState{""}, states...)
	for _, from := range froms {
		for _, to := range states {
			if got, want := IsValidTransition(from, to), allow[from][to]; got != want {
				t.Errorf("IsValidTransition(%q -> %q) = %v, want %v", from, to, got, want)
			}
		}
	}
	if IsValidTransition("", "") {
		t.Error("IsValidTransition(empty -> empty) = true, want false")
	}
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func seedFinding(name string, state v1alpha1.FindingState, observed string) *v1alpha1.Finding {
	f := &v1alpha1.Finding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.FindingSpec{
			Severity: v1alpha1.FindingSeverityHigh,
			Category: v1alpha1.FindingCategoryResources,
			TargetRef: v1alpha1.TargetRef{
				Kind: "Deployment", Namespace: "demo", Name: "web-frontend",
			},
			Detector: "kubelinter",
			State:    state,
		},
	}
	if observed != "" {
		f.Annotations = map[string]string{AnnotationObservedState: observed}
	}
	return f
}

func reconcile(t *testing.T, c client.Client, name string) {
	t.Helper()
	r := &FindingReconciler{Client: c, Scheme: c.Scheme()}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func getFinding(t *testing.T, c client.Client, name string) *v1alpha1.Finding {
	t.Helper()
	var f v1alpha1.Finding
	if err := c.Get(context.Background(), types.NamespacedName{Name: name}, &f); err != nil {
		t.Fatal(err)
	}
	return &f
}

func readyCond(t *testing.T, f *v1alpha1.Finding) *metav1.Condition {
	t.Helper()
	for i := range f.Status.Conditions {
		if f.Status.Conditions[i].Type == ConditionReady {
			return &f.Status.Conditions[i]
		}
	}
	t.Fatal("no Ready condition")
	return nil
}

// TestReconcileHappyPath walks Detected->Analyzing->FixProposed->
// Verified on a single object with the fake client.
func TestReconcileHappyPath(t *testing.T) {
	s := testScheme(t)
	f := seedFinding("f-happy", v1alpha1.FindingStateDetected, "")
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(f).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()

	path := []v1alpha1.FindingState{
		v1alpha1.FindingStateDetected,
		v1alpha1.FindingStateAnalyzing,
		v1alpha1.FindingStateFixProposed,
		v1alpha1.FindingStateVerified,
	}
	for _, want := range path {
		cur := getFinding(t, c, "f-happy")
		cur.Spec.State = want
		if err := c.Update(context.Background(), cur); err != nil {
			t.Fatal(err)
		}
		reconcile(t, c, "f-happy")
		got := getFinding(t, c, "f-happy")
		if got.Annotations[AnnotationObservedState] != string(want) {
			t.Fatalf("observed state = %q, want %q", got.Annotations[AnnotationObservedState], want)
		}
		cond := readyCond(t, got)
		if want == v1alpha1.FindingStateVerified {
			if cond.Status != metav1.ConditionTrue || cond.Reason != "Verified" {
				t.Fatalf("Verified condition = %+v", cond)
			}
		} else if cond.Status != metav1.ConditionUnknown || cond.Reason != string(want) {
			t.Fatalf("condition for %q = %+v", want, cond)
		}
	}

	// Terminal state rejects further moves: Verified -> Failed is illegal.
	cur := getFinding(t, c, "f-happy")
	cur.Spec.State = v1alpha1.FindingStateFailed
	if err := c.Update(context.Background(), cur); err != nil {
		t.Fatal(err)
	}
	reconcile(t, c, "f-happy")
	got := getFinding(t, c, "f-happy")
	if got.Annotations[AnnotationObservedState] != string(v1alpha1.FindingStateVerified) {
		t.Fatalf("terminal state advanced: %q", got.Annotations[AnnotationObservedState])
	}
	if cond := readyCond(t, got); cond.Reason != "InvalidTransition" {
		t.Fatalf("reason = %q, want InvalidTransition", cond.Reason)
	}
}

// TestReconcileFailureBranch walks Analyzing->Failed->Reverted.
func TestReconcileFailureBranch(t *testing.T) {
	s := testScheme(t)
	f := seedFinding("f-fail", v1alpha1.FindingStateDetected, "")
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(f).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()

	for _, want := range []v1alpha1.FindingState{
		v1alpha1.FindingStateDetected,
		v1alpha1.FindingStateAnalyzing,
		v1alpha1.FindingStateFailed,
		v1alpha1.FindingStateReverted,
	} {
		cur := getFinding(t, c, "f-fail")
		cur.Spec.State = want
		if err := c.Update(context.Background(), cur); err != nil {
			t.Fatal(err)
		}
		reconcile(t, c, "f-fail")
		if got := getFinding(t, c, "f-fail"); got.Annotations[AnnotationObservedState] != string(want) {
			t.Fatalf("observed state = %q, want %q", got.Annotations[AnnotationObservedState], want)
		}
	}
	got := getFinding(t, c, "f-fail")
	if cond := readyCond(t, got); cond.Status != metav1.ConditionFalse || cond.Reason != "Reverted" {
		t.Fatalf("Reverted condition = %+v", cond)
	}
}

// TestReconcileInvalidMoves asserts every illegal move is rejected:
// condition flips to InvalidTransition and the recorded state stands.
func TestReconcileInvalidMoves(t *testing.T) {
	allow := validMoves()
	froms := append([]v1alpha1.FindingState{""}, states...)
	for _, from := range froms {
		for _, to := range states {
			if allow[from][to] {
				continue
			}
			s := testScheme(t)
			name := "f-invalid"
			c := fake.NewClientBuilder().WithScheme(s).
				WithObjects(seedFinding(name, to, string(from))).
				WithStatusSubresource(&v1alpha1.Finding{}).Build()
			reconcile(t, c, name)
			got := getFinding(t, c, name)
			if got.Annotations[AnnotationObservedState] != string(from) {
				t.Errorf("move %q -> %q advanced annotation to %q",
					from, to, got.Annotations[AnnotationObservedState])
			}
			cond := readyCond(t, got)
			if cond.Status != metav1.ConditionFalse || cond.Reason != "InvalidTransition" {
				t.Errorf("move %q -> %q condition = %+v, want InvalidTransition", from, to, cond)
			}
		}
	}
}

// TestReconcileSanitizesStatus plants a raw secret-bearing entry in
// status (as another writer could) and asserts the reconcile scrubs it
// and backfills the audit hash.
func TestReconcileSanitizesStatus(t *testing.T) {
	s := testScheme(t)
	f := seedFinding("f-scrub", v1alpha1.FindingStateAnalyzing, "Analyzing")
	f.Status.Timeline = []v1alpha1.TimelineEntry{{
		Tool:   "get_logs",
		Args:   "ns=demo token=hunter2-hunter2",
		Result: "saw AKIAIOSFODNN7EXAMPLE in output",
		Ts:     1,
	}}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(f).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()

	reconcile(t, c, "f-scrub")
	got := getFinding(t, c, "f-scrub")
	if len(got.Status.Timeline) != 1 {
		t.Fatalf("timeline = %+v", got.Status.Timeline)
	}
	e := got.Status.Timeline[0]
	for _, leak := range []string{"hunter2", "AKIAIOSFODNN7EXAMPLE"} {
		if contains(e.Args, leak) || contains(e.Result, leak) {
			t.Fatalf("secret survived in status: %+v", e)
		}
	}
	if want := evidence.Fingerprint(e.Args + "\n" + e.Result); e.EvidenceHash != want {
		t.Fatalf("EvidenceHash = %q, want recomputed %q", e.EvidenceHash, want)
	}
}

// TestTimelineEntryRoundTrip proves a TimelineEntry built by
// NewTimelineEntry persists intact through status update AND patch,
// sanitized and audit-hashed.
func TestTimelineEntryRoundTrip(t *testing.T) {
	s := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(seedFinding("f-tl", v1alpha1.FindingStateDetected, "")).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()

	want := NewTimelineEntry("get_manifest", "ns=demo password=hunter2-hunter2",
		"42 lines loaded", "example-model-v1", 1700000000)
	if contains(want.Args, "hunter2") {
		t.Fatalf("constructor leaked secret: %+v", want)
	}
	if want.ModelVersion != "example-model-v1" {
		t.Fatalf("ModelVersion lost: %+v", want)
	}
	if want.EvidenceHash != evidence.Fingerprint(want.Args+"\n"+want.Result) {
		t.Fatalf("EvidenceHash mismatch: %+v", want)
	}

	ctx := context.Background()
	cur := getFinding(t, c, "f-tl")
	cur.Status.Timeline = append(cur.Status.Timeline, want)
	if err := c.Status().Update(ctx, cur); err != nil {
		t.Fatal(err)
	}
	got := getFinding(t, c, "f-tl")
	if len(got.Status.Timeline) != 1 || got.Status.Timeline[0] != want {
		t.Fatalf("update round-trip altered entry:\n got %+v\nwant %+v", got.Status.Timeline, want)
	}

	// Same entry must survive a merge patch untouched.
	before := getFinding(t, c, "f-tl")
	patch := client.MergeFrom(before.DeepCopy())
	before.Status.Timeline[0].Result = want.Result // no-op edit keeps patch focused
	if err := c.Status().Patch(ctx, before, patch); err != nil {
		t.Fatal(err)
	}
	after := getFinding(t, c, "f-tl")
	if len(after.Status.Timeline) != 1 || after.Status.Timeline[0] != want {
		t.Fatalf("patch round-trip altered entry:\n got %+v\nwant %+v", after.Status.Timeline, want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
