package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
)

func policyScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := networkingv1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func seedPolicy(name string) *v1alpha1.RemediationPolicy {
	return &v1alpha1.RemediationPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.RemediationPolicySpec{
			TargetSelector: v1alpha1.TargetSelector{Namespace: "demo"},
			Mode:           v1alpha1.RemediationPolicyModeRequireApproval,
		},
	}
}

func policyClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(policyScheme(t)).WithObjects(objs...).
		WithStatusSubresource(&v1alpha1.RemediationPolicy{}).Build()
}

func reconcilePolicy(t *testing.T, c client.Client, name string) {
	t.Helper()
	r := &RemediationPolicyReconciler{Client: c, Scheme: c.Scheme()}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func getUnstructured(t *testing.T, c client.Client, gvk schema.GroupVersionKind, name, ns string) *unstructured.Unstructured {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	if err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, u); err != nil {
		t.Fatal(err)
	}
	return u
}

func sandboxGVKs() map[string]schema.GroupVersionKind {
	return map[string]schema.GroupVersionKind{
		"ns":    {Group: "", Version: "v1", Kind: "Namespace"},
		"deny":  {Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"},
		"quota": {Group: "", Version: "v1", Kind: "ResourceQuota"},
	}
}

// TestSandboxCreation applies one policy on an empty cluster and asserts
// the namespace, the deny-all policy and the quota appear with Ready=True.
func TestSandboxCreation(t *testing.T) {
	c := policyClient(t, seedPolicy("p-create"))
	reconcilePolicy(t, c, "p-create")
	gvks := sandboxGVKs()
	getUnstructured(t, c, gvks["ns"], SandboxNamespace, "")
	deny := getUnstructured(t, c, gvks["deny"], "deny-all", SandboxNamespace)
	if got, _, _ := unstructured.NestedSlice(deny.Object, "spec", "ingress"); len(got) != 0 {
		t.Fatalf("deny-all ingress = %v, want empty", got)
	}
	if got, _, _ := unstructured.NestedSlice(deny.Object, "spec", "egress"); len(got) != 0 {
		t.Fatalf("deny-all egress = %v, want empty", got)
	}
	quota := getUnstructured(t, c, gvks["quota"], "sandbox-quota", SandboxNamespace)
	hard, _, _ := unstructured.NestedStringMap(quota.Object, "spec", "hard")
	if hard["pods"] != "10" || hard["limits.memory"] != "16Gi" {
		t.Fatalf("quota hard = %v, want pods=10 limits.memory=16Gi", hard)
	}
	var p v1alpha1.RemediationPolicy
	if err := c.Get(context.Background(), types.NamespacedName{Name: "p-create"}, &p); err != nil {
		t.Fatal(err)
	}
	ready := false
	for _, cond := range p.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == metav1.ConditionTrue && cond.Reason == "SandboxReady" {
			ready = true
		}
	}
	if !ready {
		t.Fatalf("Ready condition = %+v, want True/SandboxReady", p.Status.Conditions)
	}
}

// TestSandboxIdempotent reconciles twice and asserts the second pass
// changes nothing observable (same resource versions, still ready).
func TestSandboxIdempotent(t *testing.T) {
	c := policyClient(t, seedPolicy("p-idem"))
	reconcilePolicy(t, c, "p-idem")
	gvks := sandboxGVKs()
	versions := map[string]string{}
	for key, gvk := range gvks {
		name, ns := SandboxNamespace, ""
		if key != "ns" {
			name, ns = map[string]string{"deny": "deny-all", "quota": "sandbox-quota"}[key], SandboxNamespace
		}
		versions[key] = getUnstructured(t, c, gvk, name, ns).GetResourceVersion()
	}
	reconcilePolicy(t, c, "p-idem")
	for key, gvk := range gvks {
		name, ns := SandboxNamespace, ""
		if key != "ns" {
			name, ns = map[string]string{"deny": "deny-all", "quota": "sandbox-quota"}[key], SandboxNamespace
		}
		if got := getUnstructured(t, c, gvk, name, ns).GetResourceVersion(); got != versions[key] {
			t.Fatalf("%s resourceVersion changed on no-op reconcile: %q -> %q", key, versions[key], got)
		}
	}
}

// TestSandboxConvergesChanges deletes the NetworkPolicy and widens the
// quota, then asserts one reconcile restores both to the desired state.
func TestSandboxConvergesChanges(t *testing.T) {
	c := policyClient(t, seedPolicy("p-conv"))
	reconcilePolicy(t, c, "p-conv")
	gvks := sandboxGVKs()
	ctx := context.Background()
	deny := getUnstructured(t, c, gvks["deny"], "deny-all", SandboxNamespace)
	if err := c.Delete(ctx, deny); err != nil {
		t.Fatal(err)
	}
	quota := getUnstructured(t, c, gvks["quota"], "sandbox-quota", SandboxNamespace)
	_ = unstructured.SetNestedStringMap(quota.Object, map[string]string{"pods": "100"}, "spec", "hard")
	if err := c.Update(ctx, quota); err != nil {
		t.Fatal(err)
	}
	reconcilePolicy(t, c, "p-conv")
	getUnstructured(t, c, gvks["deny"], "deny-all", SandboxNamespace)
	restored := getUnstructured(t, c, gvks["quota"], "sandbox-quota", SandboxNamespace)
	if hard, _, _ := unstructured.NestedStringMap(restored.Object, "spec", "hard"); hard["pods"] != "10" {
		t.Fatalf("quota not converged: %v", hard)
	}
}
