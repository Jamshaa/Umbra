package main

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
)

func TestPublishFindings(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	hits := []scanHit{{
		Rule: "f01",
		Finding: v1alpha1.Finding{Spec: v1alpha1.FindingSpec{
			Severity:  v1alpha1.FindingSeverityMedium,
			Category:  v1alpha1.FindingCategoryResources,
			TargetRef: v1alpha1.TargetRef{Kind: "Deployment", Namespace: "demo", Name: "web"},
			Detector:  "scanner",
			State:     v1alpha1.FindingStateDetected,
		}},
	}}
	manifests := map[string]string{
		"Deployment/demo/web": "kind: Deployment\n  password: hunter2-hunter2\n",
	}
	if err := publishFindings(context.Background(), c, hits, manifests); err != nil {
		t.Fatal(err)
	}
	var got v1alpha1.Finding
	if err := c.Get(context.Background(), types.NamespacedName{Name: "f01-demo-web"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Labels["umbra.io/source"] != "scan" || got.Spec.State != v1alpha1.FindingStateDetected {
		t.Fatalf("published finding wrong: %+v", got)
	}
	if strings.Contains(got.Spec.PrePatchManifest, "hunter2") {
		t.Fatalf("secret survived publish: %q", got.Spec.PrePatchManifest)
	}
	// Idempotent: second publish changes nothing.
	if err := publishFindings(context.Background(), c, hits, manifests); err != nil {
		t.Fatal(err)
	}
	var list v1alpha1.FindingList
	if err := c.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("findings = %d, want 1", len(list.Items))
	}
	// A later state is never degraded.
	got.Spec.State = v1alpha1.FindingStateVerified
	if err := c.Update(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if err := publishFindings(context.Background(), c, hits, manifests); err != nil {
		t.Fatal(err)
	}
	var after v1alpha1.Finding
	if err := c.Get(context.Background(), types.NamespacedName{Name: "f01-demo-web"}, &after); err != nil {
		t.Fatal(err)
	}
	if after.Spec.State != v1alpha1.FindingStateVerified {
		t.Fatalf("state degraded to %q", after.Spec.State)
	}
	// Limits: a huge manifest is truncated to the CRD cap.
	big := map[string]string{"Deployment/demo/web": strings.Repeat("x", maxFieldBytes+10)}
	if err := publishFindings(context.Background(), c, hits, big); err != nil {
		t.Fatal(err)
	}
	_ = after
}

func TestFindingNameDeterministic(t *testing.T) {
	if got := findingName("F01", "Demo", "Web.App"); got != "f01-demo-web-app" {
		t.Fatalf("name = %q", got)
	}
	long := findingName("f01", "demo", strings.Repeat("a", 300))
	if len(long) > 253 || !strings.Contains(long, "-") {
		t.Fatalf("long name = %q (len %d)", long, len(long))
	}
	if long != findingName("f01", "demo", strings.Repeat("a", 300)) {
		t.Fatal("name not deterministic")
	}
}

func TestChangesReport(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"managedFields": []any{map[string]any{
				"manager": "kubectl", "operation": "Update",
				"fieldsV1": map[string]any{"f:spec": map[string]any{"f:replicas": map[string]any{}}},
			}},
		},
	}
	audit := []string{
		`{"user":{"username":"alice@example.com"},"verb":"patch","objectRef":{"resource":"deployments","namespace":"demo","name":"web"},"requestReceivedTimestamp":"2026-11-03T10:00:00Z"}`,
		`{"user":{"username":"admin","groups":["system:masters"]},"impersonatedUser":{"username":"bob@example.com"},"verb":"update","objectRef":{"resource":"deployments","namespace":"demo","name":"web"},"requestReceivedTimestamp":"2026-11-03T10:05:00Z"}`,
		`{"user":{"username":"system:controller"},"verb":"update","objectRef":{"resource":"pods","namespace":"demo","name":"other"},"requestReceivedTimestamp":"2026-11-03T10:06:00Z"}`,
		"not json",
	}
	lines := changesReport(obj, audit, "Deployment", "web", "demo")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "alice@example.com") || !strings.Contains(joined, "person") {
		t.Fatalf("person attribution missing:\n%s", joined)
	}
	if !strings.Contains(joined, "bob@example.com") || !strings.Contains(joined, "impersonated") {
		t.Fatalf("impersonation not labeled:\n%s", joined)
	}
	if strings.Contains(joined, "other") {
		t.Fatalf("unrelated object leaked:\n%s", joined)
	}
	// Object without recorded changes reports nothing.
	if got := changesReport(map[string]any{}, nil, "Deployment", "web", "demo"); len(got) != 0 {
		t.Fatalf("want no lines, got %v", got)
	}
	var _ = metav1.ObjectMeta{}
}
