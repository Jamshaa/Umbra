package agent

import (
	"context"
	"sort"
	"strings"
)

// Kube is the read-only cluster surface tools may touch. No shell, no
// free kubectl, no credentials: list/get/logs through this interface.
// Maps are unstructured objects (see internal/gitops for the shape).
type Kube interface {
	// Get returns one object (Deployment, Pod, ConfigMap, ...).
	Get(ctx context.Context, kind, name, namespace string) (map[string]any, error)
	// List returns objects of kind; selector matches labels exactly
	// (empty selector matches all).
	List(ctx context.Context, kind, namespace string, selector map[string]string) ([]map[string]any, error)
	// Logs returns the last tail lines of a container log.
	Logs(ctx context.Context, namespace, pod, container string, tail int) (string, error)
}

// Related mirrors what get_related returns: linked objects by
// reference, never secret values.
type Related struct {
	Services     []string `json:"services"`
	ConfigMaps   []string `json:"configMaps"`
	Secrets      []string `json:"secrets"`
	PVCs         []string `json:"pvcs"`
	ServiceAcct  string   `json:"serviceAccount"`
	RoleBindings []string `json:"roleBindings"`
	Owners       []string `json:"owners,omitempty"`
}

func str(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func metaOf(obj map[string]any) (ns, name string) {
	meta, _ := obj["metadata"].(map[string]any)
	return str(meta, "namespace"), str(meta, "name")
}

// containersOf returns the pod-template (or inline pod) containers.
func containersOf(obj map[string]any) []any {
	spec, _ := obj["spec"].(map[string]any)
	if tmpl, _ := spec["template"].(map[string]any); tmpl != nil {
		if ps, _ := tmpl["spec"].(map[string]any); ps != nil {
			if cs, _ := ps["containers"].([]any); cs != nil {
				return cs
			}
		}
	}
	if cs, _ := spec["containers"].([]any); cs != nil {
		return cs
	}
	return nil
}

// podSpecOf returns the pod spec (template or inline) for volume and
// service-account reads.
func podSpecOf(obj map[string]any) map[string]any {
	spec, _ := obj["spec"].(map[string]any)
	if tmpl, _ := spec["template"].(map[string]any); tmpl != nil {
		if ps, ok := tmpl["spec"].(map[string]any); ok {
			return ps
		}
	}
	if _, ok := spec["containers"]; ok {
		return spec
	}
	return nil
}

// GetRelated resolves the dependency neighbourhood of an object:
// Services selecting it, referenced ConfigMaps/Secrets/PVCs/service
// account, and RoleBindings pointing at that account. Values are
// names only — never secret data.
func GetRelated(ctx context.Context, k Kube, kind, name, namespace string) (Related, error) {
	obj, err := k.Get(ctx, kind, name, namespace)
	if err != nil {
		return Related{}, err
	}
	var out Related
	// Services whose selector matches this object's pod labels.
	svcs, err := k.List(ctx, "Service", namespace, nil)
	if err != nil {
		return Related{}, err
	}
	for _, s := range svcs {
		spec, _ := s["spec"].(map[string]any)
		sel, _ := spec["selector"].(map[string]any)
		if len(sel) > 0 && matchesLabels(obj, sel) {
			_, n := metaOf(s)
			out.Services = append(out.Services, n)
		}
	}
	// Volume + env references from every container. The seen set is
	// keyed by list so equal names in different lists never collide.
	seen := map[string]bool{}
	add := func(kind, v string, list *[]string) {
		if v == "" || seen[kind+"\x00"+v] {
			return
		}
		seen[kind+"\x00"+v] = true
		*list = append(*list, v)
	}
	ps := podSpecOf(obj)
	if ps != nil {
		for _, v := range volumesOf(ps) {
			switch {
			case v.cm != "":
				add("cm", v.cm, &out.ConfigMaps)
			case v.secret != "":
				add("secret", v.secret, &out.Secrets)
			case v.pvc != "":
				add("pvc", v.pvc, &out.PVCs)
			}
		}
		out.ServiceAcct = str(ps, "serviceAccountName")
		if out.ServiceAcct == "" {
			if sa, _ := ps["serviceAccount"].(string); sa != "" {
				out.ServiceAcct = sa
			}
		}
		// envFrom / valueFrom references.
		for _, c := range containersOf(obj) {
			cm, _ := c.(map[string]any)
			if cm == nil {
				continue
			}
			for _, ef := range asList(cm["envFrom"]) {
				efm, _ := ef.(map[string]any)
				if r, _ := efm["configMapRef"].(map[string]any); r != nil {
					add("cm", str(r, "name"), &out.ConfigMaps)
				}
				if r, _ := efm["secretRef"].(map[string]any); r != nil {
					add("secret", str(r, "name"), &out.Secrets)
				}
			}
			for _, e := range asList(cm["env"]) {
				em, _ := e.(map[string]any)
				if vf, _ := em["valueFrom"].(map[string]any); vf != nil {
					if r, _ := vf["configMapKeyRef"].(map[string]any); r != nil {
						add("cm", str(r, "name"), &out.ConfigMaps)
					}
					if r, _ := vf["secretKeyRef"].(map[string]any); r != nil {
						add("secret", str(r, "name"), &out.Secrets)
					}
				}
			}
		}
	}
	// RoleBindings pointing at this service account (metadata only).
	if out.ServiceAcct != "" {
		rbs, err := k.List(ctx, "RoleBinding", namespace, nil)
		if err != nil {
			return Related{}, err
		}
		crbs, err := k.List(ctx, "ClusterRoleBinding", "", nil)
		if err != nil {
			return Related{}, err
		}
		for _, rb := range append(rbs, crbs...) {
			for _, s := range asList(rb["subjects"]) {
				sm, _ := s.(map[string]any)
				if sm == nil {
					continue
				}
				if str(sm, "kind") == "ServiceAccount" && str(sm, "name") == out.ServiceAcct {
					_, n := metaOf(rb)
					add("rb", n, &out.RoleBindings)
				}
			}
		}
	}
	sort.Strings(out.Services)
	sort.Strings(out.ConfigMaps)
	sort.Strings(out.Secrets)
	sort.Strings(out.PVCs)
	sort.Strings(out.RoleBindings)
	// Owner chain: who created this object (ReplicaSet -> Deployment...).
	if meta, _ := obj["metadata"].(map[string]any); meta != nil {
		for _, o := range asList(meta["ownerReferences"]) {
			om, _ := o.(map[string]any)
			if om == nil {
				continue
			}
			kind, name := str(om, "kind"), str(om, "name")
			if kind != "" && name != "" {
				out.Owners = append(out.Owners, kind+"/"+name)
			}
		}
	}
	sort.Strings(out.Owners)
	return out, nil
}

type volumeRef struct {
	cm, secret, pvc string
}

func volumesOf(ps map[string]any) []volumeRef {
	var out []volumeRef
	for _, v := range asList(ps["volumes"]) {
		vm, _ := v.(map[string]any)
		if vm == nil {
			continue
		}
		var r volumeRef
		if m, _ := vm["configMap"].(map[string]any); m != nil {
			r.cm = str(m, "name")
		}
		if m, _ := vm["secret"].(map[string]any); m != nil {
			if n, _ := m["secretName"].(string); n != "" {
				r.secret = n
			} else {
				r.secret = str(m, "name")
			}
		}
		if m, _ := vm["persistentVolumeClaim"].(map[string]any); m != nil {
			r.pvc = str(m, "claimName")
		}
		out = append(out, r)
	}
	return out
}

func asList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

// matchesLabels reports whether the object's pod-template labels
// satisfy every selector entry.
func matchesLabels(obj map[string]any, sel map[string]any) bool {
	spec, _ := obj["spec"].(map[string]any)
	tmpl, _ := spec["template"].(map[string]any)
	meta, _ := tmpl["metadata"].(map[string]any)
	labels, _ := meta["labels"].(map[string]any)
	for k, v := range sel {
		want, _ := v.(string)
		got, _ := labels[k].(string)
		if got != want {
			return false
		}
	}
	return true
}

// EventsFor returns human-readable event lines for an object,
// newest last, capped to keep prompts small.
func EventsFor(ctx context.Context, k Kube, kind, name, namespace string) ([]string, error) {
	evs, err := k.List(ctx, "Event", namespace, nil)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range evs {
		inv, _ := e["involvedObject"].(map[string]any)
		if inv == nil {
			continue
		}
		if str(inv, "kind") != kind || str(inv, "name") != name {
			continue
		}
		reason := str(e, "reason")
		msg := str(e, "message")
		t, _ := e["lastTimestamp"].(string)
		if t == "" {
			t, _ = e["eventTime"].(string)
		}
		out = append(out, strings.TrimSpace(t+" "+reason+": "+msg))
		if len(out) >= 20 {
			break
		}
	}
	return out, nil
}
