// Package controller implements the Finding lifecycle reconciler.
//
// It enforces the Finding state machine
//
//	Detected -> Analyzing -> FixProposed -> Verified
//	                                \-> Failed -> Reverted
//
// persists TimelineEntry values only in status, and mirrors the flow
// state in conditions. Every string written to status passes through
// internal/evidence first. This task wires no LLM and no sandbox: the
// reconciler only guards the status line.
package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/evidence"
)

const (
	// ConditionReady mirrors the Finding flow state in status.conditions.
	ConditionReady = "Ready"
	// AnnotationObservedState records the last accepted spec.state, so the
	// next reconcile can validate the transition. status itself carries
	// no copy of the state (TimelineEntry lives there alone).
	AnnotationObservedState = "umbra.io/observed-state"
)

// IsValidTransition reports whether a Finding may move from state from
// to state to. An empty from means "no recorded state": only fresh
// findings entering Detected are accepted. Staying in the same state
// is valid so re-reconciles are no-ops. Verified and Reverted are
// terminal.
func IsValidTransition(from, to v1alpha1.FindingState) bool {
	if to == "" {
		return false
	}
	if from == "" {
		return to == v1alpha1.FindingStateDetected
	}
	if from == to {
		return true
	}
	switch from {
	case v1alpha1.FindingStateDetected:
		return to == v1alpha1.FindingStateAnalyzing
	case v1alpha1.FindingStateAnalyzing:
		return to == v1alpha1.FindingStateFixProposed || to == v1alpha1.FindingStateFailed
	case v1alpha1.FindingStateFixProposed:
		return to == v1alpha1.FindingStateVerified || to == v1alpha1.FindingStateFailed
	case v1alpha1.FindingStateFailed:
		return to == v1alpha1.FindingStateReverted
	default: // Verified, Reverted: terminal.
		return false
	}
}

// NewTimelineEntry builds an audit-ready TimelineEntry: args and result
// are sanitized before being stored, and EvidenceHash covers exactly
// what is stored ("sanitized args + \n + sanitized result").
func NewTimelineEntry(tool, args, result, modelVersion string, ts int64) v1alpha1.TimelineEntry {
	args = evidence.Sanitize(args)
	result = evidence.Sanitize(result)
	return v1alpha1.TimelineEntry{
		Tool:         tool,
		Args:         args,
		Result:       result,
		ModelVersion: modelVersion,
		EvidenceHash: evidence.Fingerprint(args + "\n" + result),
		Ts:           ts,
	}
}

// sanitizeStatus runs every status TimelineEntry through the evidence
// sanitizer and recomputes EvidenceHash whenever stored content
// changes (or no hash was recorded). It reports whether it modified
// the status.
func sanitizeStatus(status *v1alpha1.FindingStatus) bool {
	changed := false
	for i := range status.Timeline {
		e := &status.Timeline[i]
		args, result := evidence.Sanitize(e.Args), evidence.Sanitize(e.Result)
		if args != e.Args || result != e.Result {
			e.Args, e.Result = args, result
			changed = true
		}
		if want := evidence.Fingerprint(e.Args + "\n" + e.Result); e.EvidenceHash != want {
			e.EvidenceHash = want
			changed = true
		}
	}
	return changed
}

// readyConditionFor mirrors the flow state in the Ready condition.
func readyConditionFor(state v1alpha1.FindingState, generation int64) metav1.Condition {
	switch state {
	case v1alpha1.FindingStateVerified:
		return metav1.Condition{Type: ConditionReady, Status: metav1.ConditionTrue,
			Reason: "Verified", Message: "fix verified against deterministic checks",
			ObservedGeneration: generation}
	case v1alpha1.FindingStateReverted:
		return metav1.Condition{Type: ConditionReady, Status: metav1.ConditionFalse,
			Reason: "Reverted", Message: "rolled back to pre-patch snapshot",
			ObservedGeneration: generation}
	case v1alpha1.FindingStateFailed:
		return metav1.Condition{Type: ConditionReady, Status: metav1.ConditionFalse,
			Reason: "Failed", Message: "fix verification failed; awaiting rollback to pre-patch snapshot",
			ObservedGeneration: generation}
	default: // Detected, Analyzing, FixProposed: in flight.
		return metav1.Condition{Type: ConditionReady, Status: metav1.ConditionUnknown,
			Reason: string(state), Message: fmt.Sprintf("finding in state %s", state),
			ObservedGeneration: generation}
	}
}

// FindingReconciler enforces the Finding state machine on status.
type FindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile validates spec.state against the last accepted state,
// sanitizes status content, and mirrors the flow state in conditions.
// It never mutates spec; TimelineEntry values live only in status.
func (r *FindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var f v1alpha1.Finding
	if err := r.Get(ctx, req.NamespacedName, &f); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	to := f.Spec.State
	if to == "" {
		to = v1alpha1.FindingStateDetected
	}
	from := v1alpha1.FindingState("")
	if f.Annotations != nil {
		from = v1alpha1.FindingState(f.Annotations[AnnotationObservedState])
	}

	if !IsValidTransition(from, to) {
		meta.SetStatusCondition(&f.Status.Conditions, metav1.Condition{
			Type: ConditionReady, Status: metav1.ConditionFalse,
			Reason:             "InvalidTransition",
			Message:            fmt.Sprintf("illegal transition from %q to %q", from, to),
			ObservedGeneration: f.Generation,
		})
		if err := r.Status().Update(ctx, &f); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Defense in depth: anything already in status gets sanitized
	// before we write status back (covers entries written by others).
	applyStatus := func() {
		sanitizeStatus(&f.Status)
		meta.SetStatusCondition(&f.Status.Conditions, readyConditionFor(to, f.Generation))
	}
	applyStatus()

	if f.Annotations == nil {
		f.Annotations = map[string]string{}
	}
	if f.Annotations[AnnotationObservedState] != string(to) {
		f.Annotations[AnnotationObservedState] = string(to)
		if err := r.Update(ctx, &f); err != nil {
			return ctrl.Result{}, err
		}
		// The metadata write returns the stored status, dropping the
		// in-memory conditions: re-apply status intent before the
		// status write below.
		applyStatus()
	}
	if err := r.Status().Update(ctx, &f); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager wires the reconciler to Finding objects.
func (r *FindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.Finding{}).Complete(r)
}
