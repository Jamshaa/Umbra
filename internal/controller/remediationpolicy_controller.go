// Package controller implements the RemediationPolicy sandbox reconciler.
//
// Reconciling a RemediationPolicy guarantees the umbra-sandbox namespace
// with a deny-all NetworkPolicy (no ingress, no egress: the sandbox has
// no network by design, DNS included) and a prudent ResourceQuota,
// all labelled for traceability. Conditions (Ready) expose the result
// via kubectl. The reconciler never touches anything outside the
// sandbox and the policy's own status.
package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
)

const (
	// SandboxNamespace is the only namespace this reconciler manages.
	SandboxNamespace = "umbra-sandbox"
	// SandboxLabelKey marks every object owned by the sandbox reconciler.
	SandboxLabelKey = "umbra.io/managed-by"
	// SandboxLabelValue is the label value for sandbox-owned objects.
	SandboxLabelValue = "remediationpolicy-controller"
)

// RemediationPolicyReconciler guarantees the sandbox for each policy.
//
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch
type RemediationPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func sandboxLabels() map[string]string {
	return map[string]string{SandboxLabelKey: SandboxLabelValue}
}

func desiredNamespace() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"})
	u.SetName(SandboxNamespace)
	u.SetLabels(sandboxLabels())
	return u
}

func desiredDenyAll() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"})
	u.SetName("deny-all")
	u.SetNamespace(SandboxNamespace)
	u.SetLabels(sandboxLabels())
	_ = unstructured.SetNestedMap(u.Object, map[string]interface{}{}, "spec", "podSelector")
	_ = unstructured.SetNestedSlice(u.Object, []interface{}{}, "spec", "ingress")
	_ = unstructured.SetNestedSlice(u.Object, []interface{}{}, "spec", "egress")
	_ = unstructured.SetNestedStringSlice(u.Object, []string{"Ingress", "Egress"}, "spec", "policyTypes")
	return u
}

func desiredQuota() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ResourceQuota"})
	u.SetName("sandbox-quota")
	u.SetNamespace(SandboxNamespace)
	u.SetLabels(sandboxLabels())
	_ = unstructured.SetNestedStringMap(u.Object, map[string]string{
		"pods":            "10",
		"requests.cpu":    "4",
		"requests.memory": "8Gi",
		"limits.cpu":      "8",
		"limits.memory":   "16Gi",
	}, "spec", "hard")
	return u
}

// ensure creates obj or converges a drifted copy back to desired.
// Only the managed fields (labels, spec) are compared.
func (r *RemediationPolicyReconciler) ensure(ctx context.Context, desired *unstructured.Unstructured) error {
	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(desired.GroupVersionKind())
	if err := r.Get(ctx, key, current); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		return r.Create(ctx, desired)
	}
	// Converge labels and spec only; never touch status or other metadata.
	// Skip the write when the managed intent matches (true idempotency).
	// shortcut: absent and empty collections compare equal; revisit if a
	// managed field ever needs explicit emptiness to mean something.
	curSpec, _, _ := unstructured.NestedFieldNoCopy(current.Object, "spec")
	desSpec, _, _ := unstructured.NestedFieldNoCopy(desired.Object, "spec")
	if equality.Semantic.DeepEqual(current.GetLabels(), desired.GetLabels()) &&
		equality.Semantic.DeepEqual(normalizeValue(curSpec), normalizeValue(desSpec)) {
		return nil
	}
	merged := current.DeepCopy()
	merged.SetLabels(desired.GetLabels())
	if v, ok, _ := unstructured.NestedFieldNoCopy(desired.Object, "spec"); ok {
		_ = unstructured.SetNestedField(merged.Object, v, "spec")
	}
	return r.Update(ctx, merged)
}

// normalizeValue collapses empty maps/slices to nil so storage pruning
// of empty collections never looks like drift.
func normalizeValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		if len(t) == 0 {
			return nil
		}
		out := make(map[string]interface{}, len(t))
		for k, e := range t {
			if nv := normalizeValue(e); nv != nil {
				out[k] = nv
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []interface{}:
		if len(t) == 0 {
			return nil
		}
		out := make([]interface{}, len(t))
		for i, e := range t {
			out[i] = normalizeValue(e)
		}
		return out
	default:
		return v
	}
}

// Reconcile guarantees the sandbox and mirrors readiness in Conditions.
// It is idempotent: a second pass with no drift writes nothing new.
func (r *RemediationPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var p v1alpha1.RemediationPolicy
	if err := r.Get(ctx, req.NamespacedName, &p); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	for _, desired := range []*unstructured.Unstructured{desiredNamespace(), desiredDenyAll(), desiredQuota()} {
		if err := r.ensure(ctx, desired); err != nil {
			meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{
				Type: "Ready", Status: metav1.ConditionFalse,
				Reason: "SandboxFailed", Message: err.Error(),
				ObservedGeneration: p.Generation,
			})
			_ = r.Status().Update(ctx, &p)
			return ctrl.Result{}, err
		}
	}
	meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue,
		Reason: "SandboxReady", Message: "sandbox namespace, deny-all policy and quota in place",
		ObservedGeneration: p.Generation,
	})
	if err := r.Status().Update(ctx, &p); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// SetupWithManager wires the reconciler to RemediationPolicy objects.
func (r *RemediationPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.RemediationPolicy{}).Complete(r)
}
