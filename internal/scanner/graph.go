package scanner

import (
	"bytes"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
)

// gDoc is a generic object for cross-resource checks.
type gDoc struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec map[string]any `json:"spec"`
	// RBAC keeps rules/roleRef at top level, not under spec.
	Rules   []any          `json:"rules"`
	RoleRef map[string]any `json:"roleRef"`
}

func decodeAll(docs []byte) []gDoc {
	var out []gDoc
	for _, doc := range bytes.Split(docs, []byte("\n---")) {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		js, err := yaml.ToJSON(doc)
		if err != nil {
			continue
		}
		var g gDoc
		if err := json.Unmarshal(js, &g); err != nil || g.Kind == "" {
			continue
		}
		out = append(out, g)
	}
	return out
}

func gFinding(kind, ns, name string, cat v1alpha1.FindingCategory, sev v1alpha1.FindingSeverity) v1alpha1.Finding {
	return v1alpha1.Finding{Spec: v1alpha1.FindingSpec{
		Severity: sev, Category: cat,
		TargetRef: v1alpha1.TargetRef{Kind: kind, Namespace: ns, Name: name},
		Detector:  DetectorName, State: v1alpha1.FindingStateDetected,
	}}
}

// podLabels extracts template labels (or pod labels) for selector matching.
func podLabels(raw map[string]any) map[string]string {
	out := map[string]string{}
	var walk func(m map[string]any)
	walk = func(m map[string]any) {}
	_ = walk
	if tmpl, _ := raw["template"].(map[string]any); tmpl != nil {
		if meta, _ := tmpl["metadata"].(map[string]any); meta != nil {
			if l, _ := meta["labels"].(map[string]any); l != nil {
				for k, v := range l {
					if s, _ := v.(string); s != "" {
						out[k] = s
					}
				}
				return out
			}
		}
	}
	return out
}

func strMap(m map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if s, _ := v.(string); s != "" {
			out[k] = s
		}
	}
	return out
}

// DetectServiceNoEndpoints: Service selector matches no workload pod labels.
func DetectServiceNoEndpoints(docs []byte) []v1alpha1.Finding {
	all := decodeAll(docs)
	var out []v1alpha1.Finding
	for _, g := range all {
		if g.Kind != "Service" {
			continue
		}
		sel, _ := g.Spec["selector"].(map[string]any)
		if len(sel) == 0 {
			continue // headless/external-name: not our call
		}
		want := strMap(sel)
		matched := false
		for _, w := range all {
			switch w.Kind {
			case "Deployment", "StatefulSet", "ReplicaSet", "DaemonSet":
			default:
				continue
			}
			if w.Metadata.Namespace != g.Metadata.Namespace {
				continue
			}
			got := podLabels(w.Spec)
			ok := true
			for k, v := range want {
				if got[k] != v {
					ok = false
					break
				}
			}
			if ok {
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, gFinding("Service", g.Metadata.Namespace, g.Metadata.Name, v1alpha1.FindingCategoryConfig, v1alpha1.FindingSeverityHigh))
		}
	}
	return out
}

// DetectHPAWithoutRequests: HPA scales on CPU vs requests; no requests
// on the target makes it useless. Needs both docs in the bundle.
func DetectHPAWithoutRequests(docs []byte) []v1alpha1.Finding {
	all := decodeAll(docs)
	// index workload containers' requests by namespace/name+kind
	type key struct{ kind, ns, name string }
	needsReq := map[key]bool{}
	for _, g := range all {
		var conts []container
		switch g.Kind {
		case "Deployment", "StatefulSet":
			var m manifest
			raw, _ := json.Marshal(map[string]any{"kind": g.Kind, "metadata": g.Metadata, "spec": g.Spec})
			_ = json.Unmarshal(raw, &m)
			conts = containersOf(m)
		}
		for _, c := range conts {
			if len(c.Resources.Requests) == 0 {
				needsReq[key{g.Kind, g.Metadata.Namespace, g.Metadata.Name}] = true
			}
		}
	}
	var out []v1alpha1.Finding
	for _, g := range all {
		if g.Kind != "HorizontalPodAutoscaler" {
			continue
		}
		ref, _ := g.Spec["scaleTargetRef"].(map[string]any)
		if ref == nil {
			continue
		}
		kind, _ := ref["kind"].(string)
		name, _ := ref["name"].(string)
		if kind == "" {
			kind = "Deployment"
		}
		if needsReq[key{kind, g.Metadata.Namespace, name}] {
			out = append(out, gFinding("HorizontalPodAutoscaler", g.Metadata.Namespace, g.Metadata.Name, v1alpha1.FindingCategoryResources, v1alpha1.FindingSeverityHigh))
		}
	}
	return out
}

// DetectHPAInvertedBounds: minReplicas above maxReplicas can never be
// admitted (the API server rejects it), so the HPA is stuck forever.
// Static by nature: this object never reaches etcd.
func DetectHPAInvertedBounds(docs []byte) []v1alpha1.Finding {
	all := decodeAll(docs)
	var out []v1alpha1.Finding
	for _, g := range all {
		if g.Kind != "HorizontalPodAutoscaler" {
			continue
		}
		min, minOK := numField(g.Spec, "minReplicas")
		max, maxOK := numField(g.Spec, "maxReplicas")
		if minOK && maxOK && min > max {
			out = append(out, gFinding("HorizontalPodAutoscaler", g.Metadata.Namespace, g.Metadata.Name, v1alpha1.FindingCategoryResources, v1alpha1.FindingSeverityHigh))
		}
	}
	return out
}

func numField(spec map[string]any, key string) (float64, bool) {
	switch v := spec[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	}
	return 0, false
}

// DetectNamespaceNoPolicy: a declared Namespace with workloads but no
// NetworkPolicy allows unrestricted pod-to-pod traffic. Only namespaces
// declared in the bundle are evaluated: a lone Deployment cannot speak
// for its cluster's policy.
func DetectNamespaceNoPolicy(docs []byte) []v1alpha1.Finding {
	all := decodeAll(docs)
	declared := map[string]string{}
	withWorkload := map[string]bool{}
	hasPolicy := map[string]bool{}
	for _, g := range all {
		ns := g.Metadata.Namespace
		if ns == "" {
			ns = "default"
		}
		if g.Kind == "Namespace" {
			declared[g.Metadata.Name] = g.Metadata.Name
		}
		switch g.Kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob":
			withWorkload[ns] = true
		case "NetworkPolicy":
			hasPolicy[ns] = true
		}
	}
	var out []v1alpha1.Finding
	for ns := range declared {
		if withWorkload[ns] && !hasPolicy[ns] {
			out = append(out, gFinding("Namespace", "", ns, v1alpha1.FindingCategorySecurity, v1alpha1.FindingSeverityMedium))
		}
	}
	return out
}

// DetectNamespaceNoQuota: workloads without ResourceQuota+LimitRange can
// starve neighbors. Only namespaces declared in the bundle are evaluated.
func DetectNamespaceNoQuota(docs []byte) []v1alpha1.Finding {
	all := decodeAll(docs)
	declared := map[string]string{}
	withWorkload := map[string]bool{}
	hasQuota := map[string]bool{}
	for _, g := range all {
		ns := g.Metadata.Namespace
		if ns == "" {
			ns = "default"
		}
		if g.Kind == "Namespace" {
			declared[g.Metadata.Name] = g.Metadata.Name
		}
		switch g.Kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob":
			withWorkload[ns] = true
		case "ResourceQuota", "LimitRange":
			hasQuota[ns] = true
		}
	}
	var out []v1alpha1.Finding
	for ns := range declared {
		if withWorkload[ns] && !hasQuota[ns] {
			out = append(out, gFinding("Namespace", "", ns, v1alpha1.FindingCategoryResources, v1alpha1.FindingSeverityLow))
		}
	}
	return out
}

// DetectRBACWildcard: wildcard verbs/resources, cluster-admin bindings,
// cluster-wide secret reads.
func DetectRBACWildcard(docs []byte) []v1alpha1.Finding {
	all := decodeAll(docs)
	var out []v1alpha1.Finding
	seen := map[string]bool{}
	emit := func(kind, ns, name string) {
		k := kind + "/" + ns + "/" + name
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, gFinding(kind, ns, name, v1alpha1.FindingCategoryRBAC, v1alpha1.FindingSeverityHigh))
	}
	containsStar := func(v any) bool {
		switch t := v.(type) {
		case []any:
			for _, e := range t {
				if s, _ := e.(string); s == "*" {
					return true
				}
			}
		case string:
			return t == "*"
		}
		return false
	}
	containsSecrets := func(v any) bool {
		switch t := v.(type) {
		case []any:
			for _, e := range t {
				if s, _ := e.(string); s == "secrets" {
					return true
				}
			}
		case string:
			return t == "secrets"
		}
		return false
	}
	for _, g := range all {
		switch g.Kind {
		case "Role", "ClusterRole":
			rules := g.Rules
			if rules == nil {
				rules, _ = g.Spec["rules"].([]any)
			}
			for _, r := range rules {
				rm, _ := r.(map[string]any)
				if rm == nil {
					continue
				}
				if containsStar(rm["verbs"]) || containsStar(rm["resources"]) || containsStar(rm["apiGroups"]) {
					emit(g.Kind, g.Metadata.Namespace, g.Metadata.Name)
				}
				if g.Kind == "ClusterRole" {
					if containsStar(rm["resources"]) || containsSecrets(rm["resources"]) {
						emit(g.Kind, g.Metadata.Namespace, g.Metadata.Name)
					}
				}
			}
		case "ClusterRoleBinding":
			rr := g.RoleRef
			if rr == nil {
				rr, _ = g.Spec["roleRef"].(map[string]any)
			}
			if rr != nil {
				if n, _ := rr["name"].(string); n == "cluster-admin" {
					emit(g.Kind, g.Metadata.Namespace, g.Metadata.Name)
				}
			}
		case "RoleBinding":
			rr := g.RoleRef
			if rr == nil {
				rr, _ = g.Spec["roleRef"].(map[string]any)
			}
			if k, _ := rr["kind"].(string); k == "ClusterRole" {
				if n, _ := rr["name"].(string); n == "cluster-admin" {
					emit(g.Kind, g.Metadata.Namespace, g.Metadata.Name)
				}
			}
		}
		_ = strings.TrimSpace
	}
	return out
}
