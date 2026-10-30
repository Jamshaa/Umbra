package gitops

import (
	"strings"
)

// Owner identifies who manages a resource, if anyone.
type Owner struct {
	System   string // "argo", "flux", or "none"
	Instance string // instance name (Argo app name, Flux object name)
}

// Detect reports who manages obj by inspecting its labels and annotations.
// Returns Owner{System:"none"} when no GitOps owner is found.
func Detect(obj map[string]any) Owner {
	meta, _ := obj["metadata"].(map[string]any)
	labels, _ := meta["labels"].(map[string]any)
	annotations, _ := meta["annotations"].(map[string]any)
	str := func(m map[string]any, k string) string {
		if v, ok := m[k].(string); ok {
			return v
		}
		return ""
	}
	// Argo CD: instance annotation or managed-by label.
	if v := str(annotations, "argocd.argopoint.io/instance"); v != "" {
		return Owner{System: "argo", Instance: v}
	}
	if str(labels, "app.kubernetes.io/managed-by") == "argoproj" {
		if v := str(labels, "app.kubernetes.io/instance"); v != "" {
			return Owner{System: "argo", Instance: v}
		}
		return Owner{System: "argo", Instance: str(labels, "app.kubernetes.io/name")}
	}
	// Flux: kustomize/helm toolkit labels, or managed-by flux.
	if v := str(labels, "kustomize.toolkit.fluxcd.io/name"); v != "" {
		return Owner{System: "flux", Instance: v}
	}
	if v := str(labels, "helm.toolkit.fluxcd.io/name"); v != "" {
		return Owner{System: "flux", Instance: v}
	}
	if str(labels, "app.kubernetes.io/managed-by") == "flux" {
		if v := str(labels, "app.kubernetes.io/instance"); v != "" {
			return Owner{System: "flux", Instance: v}
		}
		return Owner{System: "flux", Instance: str(labels, "app.kubernetes.io/name")}
	}
	return Owner{System: "none"}
}

// ManagedFieldsManagers returns the list of managers that have touched the
// object's fields (SSA field ownership). Empty when absent.
func ManagedFieldsManagers(obj map[string]any) []string {
	meta, _ := obj["metadata"].(map[string]any)
	mf, _ := meta["managedFields"].([]any)
	seen := map[string]bool{}
	var out []string
	for _, raw := range mf {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		mgr, _ := m["manager"].(string)
		if mgr == "" || seen[mgr] {
			continue
		}
		seen[mgr] = true
		out = append(out, mgr)
	}
	return out
}

// IsGitOps reports whether the resource is managed by Argo or Flux.
func (o Owner) IsGitOps() bool {
	return o.System == "argo" || o.System == "flux"
}

// String renders the owner for logs and receipts.
func (o Owner) String() string {
	if o.System == "none" {
		return "none"
	}
	if o.Instance != "" {
		return o.System + "/" + o.Instance
	}
	return o.System
}

// ensure strings import is used (kept for future label normalisation).
var _ = strings.TrimSpace
