package main

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/controller"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func seedFinding(name string) *v1alpha1.Finding {
	return &v1alpha1.Finding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.FindingSpec{
			Severity: v1alpha1.FindingSeverityHigh,
			Category: v1alpha1.FindingCategoryResources,
			TargetRef: v1alpha1.TargetRef{
				Kind: "Deployment", Namespace: "demo", Name: "web",
			},
			Detector: "test",
			State:    v1alpha1.FindingStateVerified,
		},
	}
}

func TestApproveWritesAnnotation(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(seedFinding("f-ok")).Build()
	if err := approve(context.Background(), c, "f-ok", "alice"); err != nil {
		t.Fatal(err)
	}
	var got v1alpha1.Finding
	if err := c.Get(context.Background(), types.NamespacedName{Name: "f-ok"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Annotations[controller.AnnotationApproved] != "alice" {
		t.Fatalf("annotation = %v", got.Annotations)
	}
}

func TestApproveRejects(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(seedFinding("f-ok")).Build()
	for _, tc := range []struct{ name, user string }{
		{"missing-finding", "alice"}, // unknown finding
		{"f-ok", ""},                 // empty user
		{"f-ok", "alice bob"},        // whitespace user
	} {
		// Seed the missing case separately: fake client errors on Get.
		if err := approve(context.Background(), c, tc.name, tc.user); err == nil {
			t.Errorf("approve(%q, %q) accepted, want error", tc.name, tc.user)
		}
	}
}

func TestApproveCommandName(t *testing.T) {
	// The CLI, the reconciler and the docs must agree on one annotation.
	if controller.AnnotationApproved != "umbra.dev/approved" {
		t.Fatalf("annotation = %q", controller.AnnotationApproved)
	}
}
