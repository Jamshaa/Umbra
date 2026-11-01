package invariants

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/Jamshaa/Umbra/internal/evidence"
)

// MaxDocBytes caps a candidate manifest: patches are surgical, never dumps.
const MaxDocBytes = 256 * 1024

// SandboxNamespace is the only namespace the pipeline ever targets.
const SandboxNamespace = "umbra-sandbox"

// ResourceUsage is observed consumption for one container, as quantities
// (e.g. "120Mi", "250m"). Missing entries mean "unknown": the check is
// skipped and the verdict says so, rather than blocking every fix for
// lack of metrics.
type ResourceUsage struct {
	CPU    string
	Memory string
}

// Rule binds a repair rule to the fields it may touch (path prefixes;
// "#" matches any single array index).
type Rule struct {
	// Allow lists JSON-pointer prefixes, e.g.
	// "/spec/template/spec/containers/#/resources".
	Allow []string
}

// rules is the allowlist per rule id. Unknown rule ids are refused:
// no allowlist, no patch.
var rules = map[string]Rule{
	"F01": {Allow: []string{"/spec/template/spec/containers/#/resources", "/spec/containers/#/resources"}},
	"F02": {Allow: []string{"/spec/template/spec/containers/#/resources", "/spec/containers/#/resources"}},
	"F03": {Allow: []string{"/spec/template/spec/containers/#/resources", "/spec/containers/#/resources"}},
	"F04": {Allow: []string{"/spec/template/spec/containers/#/resources", "/spec/containers/#/resources"}},
	"F05": {Allow: []string{"/spec/template/spec/containers/#/livenessProbe", "/spec/containers/#/livenessProbe"}},
	"F06": {Allow: []string{"/spec/template/spec/containers/#/readinessProbe", "/spec/containers/#/readinessProbe"}},
	// CronJob deterministic repairs: scalar job control fields only.
	"CronDeadline":      {Allow: []string{"/spec/startingDeadlineSeconds"}},
	"CronShortDeadline": {Allow: []string{"/spec/startingDeadlineSeconds"}},
	"CronSuspended":     {Allow: []string{"/spec/suspend"}},
	"CronOverlap":       {Allow: []string{"/spec/concurrencyPolicy"}},
	"CronTimezone":      {Allow: []string{"/spec/timeZone"}},
	"CronBounds": {Allow: []string{
		"/spec/jobTemplate/spec/backoffLimit",
		"/spec/jobTemplate/spec/activeDeadlineSeconds",
		"/spec/jobTemplate/spec/ttlSecondsAfterFinished",
	}},
	"CronHistory": {Allow: []string{"/spec/successfulJobsHistoryLimit", "/spec/failedJobsHistoryLimit"}},
	// Model-proposed, statically gated: selectors, HPA targets, probes, RBAC.
	"ServiceEndpoints":   {Allow: []string{"/spec/selector"}},
	"HPARequests":        {Allow: []string{"/spec/template/spec/containers/#/resources", "/spec/containers/#/resources"}},
	"ProbesDistinct":     {Allow: []string{"/spec/template/spec/containers/#/readinessProbe", "/spec/containers/#/readinessProbe"}},
	"RBACLeastPrivilege": {Allow: []string{"/rules", "/spec/rules"}},
	"HPAInvertedBounds":  {Allow: []string{"/spec/minReplicas", "/spec/maxReplicas"}},
}

// decodeOne parses a single YAML/JSON document into plain maps.
func decodeOne(doc []byte) (map[string]any, error) {
	js, err := yaml.ToJSON(bytesTrimSpace(doc))
	if err != nil {
		return nil, fmt.Errorf("invariants: not YAML/JSON: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(js, &out); err != nil {
		return nil, fmt.Errorf("invariants: not an object: %w", err)
	}
	return out, nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// DiffPaths lists every changed leaf as a JSON-pointer-ish path,
// array indices as numbers, map keys escaped per RFC 6901 (~0, ~1).
// Output is sorted for determinism.
func DiffPaths(before, after map[string]any) []string {
	var out []string
	diffInto("", before, after, &out)
	sort.Strings(out)
	return out
}

func diffInto(path string, before, after any, out *[]string) {
	bm, bok := before.(map[string]any)
	am, aok := after.(map[string]any)
	if bok && aok {
		keys := map[string]bool{}
		for k := range bm {
			keys[k] = true
		}
		for k := range am {
			keys[k] = true
		}
		ordered := make([]string, 0, len(keys))
		for k := range keys {
			ordered = append(ordered, k)
		}
		sort.Strings(ordered)
		for _, k := range ordered {
			bv, bok := bm[k]
			av, aok := am[k]
			switch {
			case !bok || !aok:
				*out = append(*out, path+"/"+escape(k))
			case !reflect.DeepEqual(bv, av):
				diffInto(path+"/"+escape(k), bv, av, out)
			}
		}
		return
	}
	bl, blok := before.([]any)
	al, alok := after.([]any)
	if blok && alok {
		n := len(bl)
		if len(al) > n {
			n = len(al)
		}
		for i := 0; i < n; i++ {
			switch {
			case i >= len(bl) || i >= len(al):
				*out = append(*out, path+"/"+strconv.Itoa(i))
			case !reflect.DeepEqual(bl[i], al[i]):
				diffInto(path+"/"+strconv.Itoa(i), bl[i], al[i], out)
			}
		}
		return
	}
	// Scalar changed (or type changed): the node itself is the change.
	*out = append(*out, path)
}

func escape(seg string) string {
	seg = strings.ReplaceAll(seg, "~", "~0")
	return strings.ReplaceAll(seg, "/", "~1")
}

// allowed reports whether changed is covered by one of the rule's path
// prefixes. "#" in a pattern matches any single array index.
func allowed(patterns []string, changed string) bool {
	for _, p := range patterns {
		if matchPrefix(splitPath(p), splitPath(changed)) {
			return true
		}
	}
	return false
}

func splitPath(p string) []string {
	if p == "" || p == "/" {
		return nil
	}
	return strings.Split(strings.TrimPrefix(p, "/"), "/")
}

func matchPrefix(pattern, path []string) bool {
	if len(pattern) > len(path) {
		return false
	}
	for i, seg := range pattern {
		if seg != "#" && seg != path[i] {
			return false
		}
	}
	return true
}

// CheckAllowlist verifies every changed path against the rule's fields.
// Unknown rules and uncovered paths are refused.
func CheckAllowlist(ruleID string, changed []string) error {
	rule, ok := rules[ruleID]
	if !ok {
		return fmt.Errorf("no allowlist for rule %q", ruleID)
	}
	for _, c := range changed {
		if !allowed(rule.Allow, c) {
			return fmt.Errorf("rule %s must not touch %s", ruleID, c)
		}
	}
	return nil
}

func strOf(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func metaOf(obj map[string]any) map[string]any {
	meta, _ := obj["metadata"].(map[string]any)
	if meta == nil {
		return map[string]any{}
	}
	return meta
}

// checkIdentity enforces same object, same namespace, sandbox-only.
// Cluster-scoped kinds have no namespace to match, so they are refused:
// the pipeline only touches namespaced workloads in the sandbox.
func checkIdentity(before, after map[string]any) error {
	if strOf(before, "kind") != strOf(after, "kind") {
		return fmt.Errorf("kind changed %q -> %q", strOf(before, "kind"), strOf(after, "kind"))
	}
	bm, am := metaOf(before), metaOf(after)
	if strOf(bm, "name") != strOf(am, "name") {
		return fmt.Errorf("name changed %q -> %q", strOf(bm, "name"), strOf(am, "name"))
	}
	if strOf(bm, "namespace") != strOf(am, "namespace") {
		return fmt.Errorf("namespace changed %q -> %q", strOf(bm, "namespace"), strOf(am, "namespace"))
	}
	if ns := strOf(am, "namespace"); ns != SandboxNamespace {
		return fmt.Errorf("must target the sandbox %q, got %q", SandboxNamespace, ns)
	}
	if uidB, uidA := strOf(bm, "uid"), strOf(am, "uid"); uidB != "" && uidA != "" && uidB != uidA {
		return fmt.Errorf("uid changed (immutable)")
	}
	bSel, bOk := selectorOf(before)
	aSel, aOk := selectorOf(after)
	if bOk || aOk {
		if !bOk || !aOk || !reflect.DeepEqual(bSel, aSel) {
			return fmt.Errorf("spec.selector changed (immutable)")
		}
	}
	return nil
}

// selectorOf returns the pod selector when both sides define one.
func selectorOf(obj map[string]any) (map[string]any, bool) {
	spec, _ := obj["spec"].(map[string]any)
	sel, _ := spec["selector"].(map[string]any)
	if sel == nil {
		return nil, false
	}
	return sel, true
}

// checkRBAC refuses any privilege widening: new RBAC objects, new
// verbs/resources/apiGroups, changed roleRefs or new subjects.
func checkRBAC(before, after map[string]any) error {
	kind, _ := before["kind"].(string)
	switch kind {
	case "Role", "ClusterRole":
		return checkRulesSubset(ruleSet(before), ruleSet(after))
	case "RoleBinding", "ClusterRoleBinding":
		return checkBindingSubset(before, after)
	}
	return nil
}

type triple struct{ groups, resources, verbs string }

func ruleSet(obj map[string]any) map[triple]bool {
	out := map[triple]bool{}
	spec, _ := obj["spec"].(map[string]any)
	if spec == nil {
		spec = obj
	}
	rules, _ := spec["rules"].([]any)
	if rules == nil {
		rules, _ = obj["rules"].([]any)
	}
	for _, r := range rules {
		rm, _ := r.(map[string]any)
		if rm == nil {
			continue
		}
		for _, g := range strList(rm["apiGroups"]) {
			for _, res := range strList(rm["resources"]) {
				for _, v := range strList(rm["verbs"]) {
					out[triple{g, res, v}] = true
				}
			}
		}
	}
	return out
}

func strList(v any) []string {
	var out []string
	for _, e := range toAnySlice(v) {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func toAnySlice(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

func checkRulesSubset(before, after map[triple]bool) error {
	var added []string
	for t := range after {
		if !before[t] {
			added = append(added, t.groups+"/"+t.resources+"/"+t.verbs)
		}
	}
	sort.Strings(added)
	if len(added) > 0 {
		return fmt.Errorf("RBAC widened with %s", strings.Join(added, ", "))
	}
	return nil
}

func refsOf(obj map[string]any, key string) map[string]bool {
	out := map[string]bool{}
	// bindings present subjects at top level under spec.subjects.
	for _, root := range []map[string]any{obj, nested(obj, "spec")} {
		subs, _ := root["subjects"].([]any)
		for _, s := range subs {
			sm, _ := s.(map[string]any)
			if sm == nil {
				continue
			}
			if strOf(sm, key) != "" {
				out[strOf(sm, "kind")+"/"+strOf(sm, "name")+"/"+strOf(sm, "namespace")] = true
			}
		}
	}
	return out
}

func nested(obj map[string]any, keys ...string) map[string]any {
	cur := obj
	for _, k := range keys {
		next, _ := cur[k].(map[string]any)
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

func checkBindingSubset(before, after map[string]any) error {
	getRef := func(obj map[string]any) string {
		rr, _ := obj["roleRef"].(map[string]any)
		if rr == nil {
			rr = nested(obj, "spec", "roleRef")
		}
		if rr == nil {
			return ""
		}
		return strOf(rr, "kind") + "/" + strOf(rr, "name")
	}
	if getRef(before) != getRef(after) {
		return fmt.Errorf("binding roleRef changed %q -> %q", getRef(before), getRef(after))
	}
	bSubs := refsOf(before, "name")
	for k := range refsOf(after, "name") {
		if !bSubs[k] {
			return fmt.Errorf("binding gains subject %s", k)
		}
	}
	return nil
}

// checkSize caps the candidate document.
func checkSize(after []byte) error {
	if len(after) > MaxDocBytes {
		return fmt.Errorf("candidate too large: %d bytes (max %d)", len(after), MaxDocBytes)
	}
	return nil
}

// checkSecrets rejects candidates carrying secret-looking values,
// reusing the evidence sanitizer as detector: anything it would
// redact must not travel in a patch.
func checkSecrets(after []byte) error {
	if evidence.Sanitize(string(after)) != string(after) {
		return fmt.Errorf("candidate carries secret-looking material")
	}
	return nil
}

// containersOf returns name + resources per container of a pod template
// or inline pod spec.
func containersOf(obj map[string]any) map[string]map[string]string {
	out := map[string]map[string]string{}
	var podSpec map[string]any
	if spec, _ := obj["spec"].(map[string]any); spec != nil {
		if tmpl, _ := spec["template"].(map[string]any); tmpl != nil {
			podSpec, _ = tmpl["spec"].(map[string]any)
		} else {
			podSpec = spec
		}
	}
	if podSpec == nil {
		return out
	}
	raw, _ := podSpec["containers"].([]any)
	for _, c := range raw {
		cm, _ := c.(map[string]any)
		if cm == nil {
			continue
		}
		name, _ := cm["name"].(string)
		got := map[string]string{}
		if res, _ := cm["resources"].(map[string]any); res != nil {
			for _, bucket := range []string{"limits", "requests"} {
				if b, ok := res[bucket].(map[string]any); ok {
					for k, v := range b {
						if s, ok := v.(string); ok {
							got[bucket+"."+k] = s
						}
					}
				}
			}
		}
		out[name] = got
	}
	return out
}

// checkResources refuses limits/requests below observed use. Containers
// without usage data are skipped and reported, not blocked.
func checkResources(before, after map[string]any, usage map[string]ResourceUsage) (skipped []string, err error) {
	_ = before
	for name, res := range containersOf(after) {
		u, ok := usage[name]
		if !ok || (u.CPU == "" && u.Memory == "") {
			skipped = append(skipped, name)
			continue
		}
		for _, ref := range []struct {
			key  string
			used string
		}{{"limits.memory", u.Memory}, {"requests.memory", u.Memory}, {"limits.cpu", u.CPU}, {"requests.cpu", u.CPU}} {
			if ref.used == "" {
				continue
			}
			raw, ok := res[ref.key]
			if !ok || raw == "" {
				continue
			}
			have, err1 := resource.ParseQuantity(raw)
			want, err2 := resource.ParseQuantity(ref.used)
			if err1 != nil || err2 != nil {
				continue
			}
			if have.Cmp(want) < 0 {
				return skipped, fmt.Errorf("container %q %s %s below observed use %s", name, ref.key, raw, ref.used)
			}
		}
	}
	sort.Strings(skipped)
	return skipped, nil
}
