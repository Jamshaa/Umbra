package verify

import (
	"bytes"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// multiDoc splits a bundle into generic maps.
func multiDocs(docs []byte) []map[string]any {
	var out []map[string]any
	for _, doc := range bytes.Split(docs, []byte("\n---")) {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		js, err := yaml.ToJSON(doc)
		if err != nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(js, &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out
}

func kindOf(m map[string]any) string {
	s, _ := m["kind"].(string)
	return s
}

func metaOf(m map[string]any) (ns, name string) {
	meta, _ := m["metadata"].(map[string]any)
	ns, _ = meta["namespace"].(string)
	name, _ = meta["name"].(string)
	return ns, name
}

func specOf(m map[string]any) map[string]any {
	s, _ := m["spec"].(map[string]any)
	if s == nil {
		return map[string]any{}
	}
	return s
}

// VerifyServiceEndpoints gates the selector fix: the Service selector must
// match at least one workload's pod-template labels in the bundle.
func VerifyServiceEndpoints(docs []byte, _ PodStatus) Result {
	all := multiDocs(docs)
	for _, g := range all {
		if kindOf(g) != "Service" {
			continue
		}
		sel, _ := specOf(g)["selector"].(map[string]any)
		if len(sel) == 0 {
			continue
		}
		ns, name := metaOf(g)
		matched := false
		for _, w := range all {
			switch kindOf(w) {
			case "Deployment", "StatefulSet", "ReplicaSet", "DaemonSet":
			default:
				continue
			}
			wns, _ := metaOf(w)
			if wns != ns {
				continue
			}
			tmpl, _ := specOf(w)["template"].(map[string]any)
			meta, _ := tmpl["metadata"].(map[string]any)
			labels, _ := meta["labels"].(map[string]any)
			ok := true
			for k, v := range sel {
				want, _ := v.(string)
				got, _ := labels[k].(string)
				if got != want {
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
			return fail("ServiceEndpoints: Service %s/%s selector matches nothing", ns, name)
		}
	}
	return Result{Pass: true, Reason: "ServiceEndpoints: every Service selector matches a workload"}
}

// VerifyHPARequests gates the HPA fix: every scaled workload has CPU
// requests (HPA computes utilization vs requests, not limits).
func VerifyHPARequests(docs []byte, _ PodStatus) Result {
	all := multiDocs(docs)
	missing := map[string]bool{}
	for _, g := range all {
		switch kindOf(g) {
		case "Deployment", "StatefulSet":
		default:
			continue
		}
		ns, name := metaOf(g)
		tmpl, _ := specOf(g)["template"].(map[string]any)
		ps, _ := tmpl["spec"].(map[string]any)
		conts, _ := ps["containers"].([]any)
		for _, c := range conts {
			cm, _ := c.(map[string]any)
			res, _ := cm["resources"].(map[string]any)
			req, _ := res["requests"].(map[string]any)
			cpu, _ := req["cpu"].(string)
			if strings.TrimSpace(cpu) == "" {
				missing[ns+"/"+name] = true
			}
		}
	}
	sawHPA := false
	for _, g := range all {
		if kindOf(g) != "HorizontalPodAutoscaler" {
			continue
		}
		sawHPA = true
		ref, _ := specOf(g)["scaleTargetRef"].(map[string]any)
		ns, _ := metaOf(g)
		name, _ := ref["name"].(string)
		if missing[ns+"/"+name] {
			return fail("HPARequests: target %s/%s has no CPU requests (HPA cannot compute utilization)", ns, name)
		}
	}
	if !sawHPA {
		return fail("HPARequests: no HorizontalPodAutoscaler in bundle")
	}
	return Result{Pass: true, Reason: "HPARequests: scaled workloads carry CPU requests"}
}

// VerifyHPAInvertedBounds gates the inverted-HPA diagnosis: minReplicas
// must not exceed maxReplicas. The API server refuses to persist the
// inverted object, so this gate is static by nature (the fixture, never
// etcd); the model proposes which bound to move, the gate checks the math.
func VerifyHPAInvertedBounds(docs []byte, _ PodStatus) Result {
	all := multiDocs(docs)
	sawHPA := false
	for _, g := range all {
		if kindOf(g) != "HorizontalPodAutoscaler" {
			continue
		}
		sawHPA = true
		min, minOK := numField(specOf(g), "minReplicas")
		max, maxOK := numField(specOf(g), "maxReplicas")
		if minOK && maxOK && min > max {
			ns, name := metaOf(g)
			return fail("HPAInvertedBounds: HPA %s/%s minReplicas %v above maxReplicas %v (unadmittable)", ns, name, min, max)
		}
	}
	if !sawHPA {
		return fail("HPAInvertedBounds: no HorizontalPodAutoscaler in bundle")
	}
	return Result{Pass: true, Reason: "HPAInvertedBounds: bounds ordered (min <= max)"}
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

// VerifyProbesDistinct gates the identical-probe fix: liveness and
// readiness must differ (a wedged app must restart, not just go unready).
// Comparison ignores tuning knobs (delays, thresholds): two probes on the
// same endpoint with different timings fail together, which is exactly
// the anti-pattern. Mirrors the scanner's normalization on purpose.
func VerifyProbesDistinct(doc []byte, pod PodStatus) Result {
	m, ok := decodeOne(doc)
	if !ok {
		return fail("ProbesDistinct: manifest does not decode")
	}
	for _, c := range containersOf(m) {
		if c.LivenessProbe == nil || c.ReadinessProbe == nil {
			return fail("ProbesDistinct: container %q needs both probes to compare", c.Name)
		}
		lb, _ := json.Marshal(normalizeProbeFields(c.LivenessProbe))
		rb, _ := json.Marshal(normalizeProbeFields(c.ReadinessProbe))
		if string(lb) == string(rb) {
			return fail("ProbesDistinct: container %q liveness == readiness", c.Name)
		}
	}
	if r := runningAndReady(pod, "ProbesDistinct"); r != nil {
		return *r
	}
	return Result{Pass: true, Reason: "ProbesDistinct: probes differ, pod Running and Ready"}
}

// normalizeProbeFields drops tuning knobs so endpoint identity decides.
func normalizeProbeFields(p any) any {
	raw, _ := json.Marshal(p)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return p
	}
	for _, k := range []string{"initialDelaySeconds", "periodSeconds", "timeoutSeconds", "successThreshold", "failureThreshold"} {
		delete(m, k)
	}
	return m
}

// VerifyRBACLeastPrivilege gates the RBAC fix on static ground: no
// wildcards, no cluster-admin binding in the bundle.
func VerifyRBACLeastPrivilege(docs []byte, _ PodStatus) Result {
	all := multiDocs(docs)
	hasStar := func(v any) bool {
		if s, ok := v.(string); ok {
			return s == "*"
		}
		l, _ := v.([]any)
		for _, e := range l {
			if s, _ := e.(string); s == "*" {
				return true
			}
		}
		return false
	}
	hasSecrets := func(v any) bool {
		switch t := v.(type) {
		case []any:
			for _, e := range t {
				if s, _ := e.(string); s == "secrets" || s == "*" {
					return true
				}
			}
		case string:
			return t == "secrets" || t == "*"
		}
		return false
	}
	for _, g := range all {
		switch kindOf(g) {
		case "Role", "ClusterRole":
			rules, _ := g["rules"].([]any)
			if rules == nil {
				rules, _ = specOf(g)["rules"].([]any)
			}
			for _, r := range rules {
				rm, _ := r.(map[string]any)
				if rm == nil {
					continue
				}
				if hasStar(rm["verbs"]) || hasStar(rm["resources"]) || hasStar(rm["apiGroups"]) {
					ns, name := metaOf(g)
					return fail("RBACLeastPrivilege: %s %s/%s still has wildcards", kindOf(g), ns, name)
				}
				if kindOf(g) == "ClusterRole" && hasSecrets(rm["resources"]) {
					ns, name := metaOf(g)
					return fail("RBACLeastPrivilege: ClusterRole %s/%s reads secrets cluster-wide", ns, name)
				}
			}
		case "ClusterRoleBinding":
			rr, _ := specOf(g)["roleRef"].(map[string]any)
			if rr == nil {
				rr, _ = g["roleRef"].(map[string]any)
			}
			if rr != nil {
				if n, _ := rr["name"].(string); n == "cluster-admin" {
					ns, name := metaOf(g)
					return fail("RBACLeastPrivilege: binding %s/%s still grants cluster-admin", ns, name)
				}
			}
		case "RoleBinding":
			rr, _ := specOf(g)["roleRef"].(map[string]any)
			if rr == nil {
				rr, _ = g["roleRef"].(map[string]any)
			}
			if k, _ := rr["kind"].(string); k == "ClusterRole" {
				if n, _ := rr["name"].(string); n == "cluster-admin" {
					ns, name := metaOf(g)
					return fail("RBACLeastPrivilege: binding %s/%s still grants cluster-admin", ns, name)
				}
			}
		}
	}
	return Result{Pass: true, Reason: "RBACLeastPrivilege: no wildcards, no cluster-admin grant"}
}
