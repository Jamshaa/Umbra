package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/agent"
	"github.com/Jamshaa/Umbra/internal/controller"
	"github.com/Jamshaa/Umbra/internal/evidence"
)

const maxTimelineField = 4096

func truncate(s string) string {
	if len(s) > maxTimelineField {
		return s[:maxTimelineField] + "…[truncated]"
	}
	return s
}

// findingNameFor mirrors cmd/umbra's deterministic publish name:
// <rule>-<namespace>-<name> lowercased, hash-suffixed past 253 chars.
func findingNameFor(rule, namespace, name string) string {
	raw := strings.ToLower(rule + "-" + namespace + "-" + name)
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 253 {
		sum := sha256.Sum256([]byte(s))
		s = strings.Trim(s[:242], "-") + "-" + hex.EncodeToString(sum[:])[:10]
	}
	return s
}

// transitionPath walks stepwise from->target using only valid
// transitions. It returns nil,false when the target is unreachable
// (terminal states, or Failed which may only go to Reverted).
func transitionPath(from, target v1alpha1.FindingState) ([]v1alpha1.FindingState, bool) {
	if from == target {
		return nil, true
	}
	next := map[v1alpha1.FindingState][]v1alpha1.FindingState{
		v1alpha1.FindingStateDetected:    {v1alpha1.FindingStateAnalyzing},
		v1alpha1.FindingStateAnalyzing:   {v1alpha1.FindingStateFixProposed, v1alpha1.FindingStateFailed},
		v1alpha1.FindingStateFixProposed: {v1alpha1.FindingStateVerified, v1alpha1.FindingStateFailed},
		v1alpha1.FindingStateFailed:      {v1alpha1.FindingStateReverted},
	}
	cur := from
	var path []v1alpha1.FindingState
	for cur != target {
		var step v1alpha1.FindingState
		found := false
		for _, cand := range next[cur] {
			if !controller.IsValidTransition(cur, cand) {
				continue
			}
			// Greedy walk is exact for this linear-with-branch machine:
			// prefer the candidate that is the target or can reach it.
			if cand == target || canReach(cand, target, next) {
				step, found = cand, true
				break
			}
		}
		if !found {
			return nil, false
		}
		path = append(path, step)
		cur = step
		if len(path) > 8 {
			return nil, false
		}
	}
	return path, true
}

func canReach(from, target v1alpha1.FindingState, next map[v1alpha1.FindingState][]v1alpha1.FindingState) bool {
	if from == target {
		return true
	}
	for _, n := range next[from] {
		if canReach(n, target, next) {
			return true
		}
	}
	return false
}

type publishInput struct {
	Fault     string
	Kind      string
	Namespace string
	Name      string
	Model     string
	Records   []agent.ToolRecord
	Refused   bool
	Reason    string
	RootCause string
	GatePass  bool
	GateStep  string
	GateWhy   string
	Manifest  string
	Ts        int64
}

func clamp(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// gateVerdictEntry is the one timeline entry that records the gate
// outcome of a published diagnosis (status only, never spec). Sanitized
// and clamped before the hash is taken, like every other entry.
func gateVerdictEntry(in publishInput) v1alpha1.TimelineEntry {
	step := in.GateStep
	if step == "" {
		step = "diagnose"
	}
	args := clamp(evidence.Sanitize(fmt.Sprintf("rule=%s step=%s", in.Fault, step)), 200)
	result := "PASS"
	if in.Refused || !in.GatePass {
		why := in.GateWhy
		if why == "" {
			why = in.Reason
		}
		// Sanitize the reason on its own first: embedded after
		// "FAIL at <step>: " a leading key=value could be consumed
		// as the value of the step label and escape redaction.
		result = fmt.Sprintf("FAIL at %s: %s", step, evidence.Sanitize(why))
	}
	result = clamp(evidence.Sanitize(result), 500)
	return controller.NewTimelineEntry("gate_verdict", args, result, in.Model, in.Ts)
}

// publishDiagnosis creates or updates the Finding for one diagnosis:
// state walks only valid transitions toward Verified (gate pass),
// Failed (refusal or gate failure); timeline entries are sanitized,
// truncated and deduplicated by evidence hash. When the target state
// is unreachable it reports why and writes nothing.
func publishDiagnosis(ctx context.Context, c client.Client, in publishInput) (string, error) {
	name := findingNameFor(in.Fault, in.Namespace, in.Name)
	target := v1alpha1.FindingStateFailed
	if !in.Refused && in.GatePass {
		target = v1alpha1.FindingStateVerified
	}
	var f v1alpha1.Finding
	err := c.Get(ctx, client.ObjectKey{Name: name}, &f)
	created := false
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return "", fmt.Errorf("get finding %s: %w", name, err)
		}
		f = v1alpha1.Finding{Spec: v1alpha1.FindingSpec{
			Severity:  v1alpha1.FindingSeverityMedium,
			Category:  v1alpha1.FindingCategoryReliability,
			TargetRef: v1alpha1.TargetRef{Kind: in.Kind, Namespace: in.Namespace, Name: in.Name},
			Detector:  "umbra-agent",
			State:     v1alpha1.FindingStateDetected,
		}}
		f.Name = name
		f.Labels = map[string]string{"umbra.io/source": "diagnose"}
		created = true
	}
	from := f.Spec.State
	if from == "" {
		from = v1alpha1.FindingStateDetected
	}
	path, ok := transitionPath(from, target)
	if !ok {
		return "", fmt.Errorf("cannot move finding %s from %s to %s: invalid transition (left untouched)", name, from, target)
	}
	seen := map[string]bool{}
	for _, e := range f.Status.Timeline {
		seen[e.EvidenceHash] = true
	}
	for _, r := range in.Records {
		e := controller.NewTimelineEntry(r.Name, truncate(evidence.Sanitize(r.Args)), truncate(evidence.Sanitize(r.Result)), in.Model, in.Ts)
		if seen[e.EvidenceHash] {
			continue
		}
		seen[e.EvidenceHash] = true
		f.Status.Timeline = append(f.Status.Timeline, e)
	}
	if e := gateVerdictEntry(in); !seen[e.EvidenceHash] {
		seen[e.EvidenceHash] = true
		f.Status.Timeline = append(f.Status.Timeline, e)
	}
	if in.RootCause != "" {
		f.Spec.Diagnosis = truncate(evidence.Sanitize(in.RootCause))
	} else if in.Reason != "" {
		f.Spec.Diagnosis = truncate(evidence.Sanitize(in.Reason))
	}
	if in.Manifest != "" {
		f.Spec.PrePatchManifest = truncate(evidence.Sanitize(in.Manifest))
		if len(f.Spec.PrePatchManifest) > 262144 {
			f.Spec.PrePatchManifest = f.Spec.PrePatchManifest[:262144]
		}
	}
	if created {
		if err := c.Create(ctx, &f); err != nil {
			return "", fmt.Errorf("create finding %s: %w", name, err)
		}
	}
	// Walk the state machine stepwise; each intermediate state is a
	// legal write, the last one is the target.
	for _, step := range path {
		f.Spec.State = step
		if err := c.Update(ctx, &f); err != nil {
			return "", fmt.Errorf("move finding %s to %s: %w", name, step, err)
		}
	}
	if !created && len(path) == 0 {
		if err := c.Update(ctx, &f); err != nil {
			return "", fmt.Errorf("update finding %s: %w", name, err)
		}
	}
	if err := c.Status().Update(ctx, &f); err != nil {
		return "", fmt.Errorf("update finding status %s: %w", name, err)
	}
	return fmt.Sprintf("published finding %s (%s -> %s)", name, from, target), nil
}

// requireKindUmbraCtx refuses any other kubeconfig context (no override).
func requireKindUmbraCtx(kubeconfig string) error {
	loading := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loading.ExplicitPath = kubeconfig
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return fmt.Errorf("kubeconfig: %w", err)
	}
	if cfg.CurrentContext != "kind-umbra" {
		return fmt.Errorf("current context is %q, want kind-umbra", cfg.CurrentContext)
	}
	return nil
}

func kubeClientFor(cfg *rest.Config, scheme *runtime.Scheme) (client.Client, error) {
	return client.New(cfg, client.Options{Scheme: scheme})
}
