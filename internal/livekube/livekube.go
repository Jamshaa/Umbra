// Package livekube is the production wiring the tests never touch: a
// read-only cluster surface (agent.Kube) plus the two cluster gates the
// pipeline needs (dry-run and sandbox apply with rollback), all over the
// default kubeconfig. Anything destructive only ever targets the sandbox
// namespace; production objects are read, never written.
package livekube

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/Jamshaa/Umbra/internal/verify"
)

// jsonUnmarshal parses YAML or JSON into v.
func jsonUnmarshal(doc []byte, v any) error {
	js, err := sigsyaml.YAMLToJSON(doc)
	if err != nil {
		return fmt.Errorf("livekube: not YAML/JSON: %w", err)
	}
	if err := json.Unmarshal(js, v); err != nil {
		return fmt.Errorf("livekube: not an object: %w", err)
	}
	return nil
}

// SandboxNamespace is the only namespace this package ever writes to.
const SandboxNamespace = "umbra-sandbox"

// gvrOf maps the kinds the agent may touch to their resources.
// Unknown kinds are an error: fail closed, never guess a plural.
func gvrOf(kind string) (schema.GroupVersionResource, bool) {
	table := map[string]schema.GroupVersionResource{
		"Deployment":              {Group: "apps", Version: "v1", Resource: "deployments"},
		"StatefulSet":             {Group: "apps", Version: "v1", Resource: "statefulsets"},
		"DaemonSet":               {Group: "apps", Version: "v1", Resource: "daemonsets"},
		"ReplicaSet":              {Group: "apps", Version: "v1", Resource: "replicasets"},
		"Pod":                     {Group: "", Version: "v1", Resource: "pods"},
		"Service":                 {Group: "", Version: "v1", Resource: "services"},
		"ConfigMap":               {Group: "", Version: "v1", Resource: "configmaps"},
		"Secret":                  {Group: "", Version: "v1", Resource: "secrets"},
		"ServiceAccount":          {Group: "", Version: "v1", Resource: "serviceaccounts"},
		"PersistentVolumeClaim":   {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		"Event":                   {Group: "", Version: "v1", Resource: "events"},
		"Namespace":               {Group: "", Version: "v1", Resource: "namespaces"},
		"ResourceQuota":           {Group: "", Version: "v1", Resource: "resourcequotas"},
		"LimitRange":              {Group: "", Version: "v1", Resource: "limitranges"},
		"Job":                     {Group: "batch", Version: "v1", Resource: "jobs"},
		"CronJob":                 {Group: "batch", Version: "v1", Resource: "cronjobs"},
		"HorizontalPodAutoscaler": {Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
		"Ingress":                 {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		"NetworkPolicy":           {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
		"Role":                    {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		"ClusterRole":             {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		"RoleBinding":             {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		"ClusterRoleBinding":      {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
	}
	g, ok := table[kind]
	return g, ok
}

// clusterScoped reports kinds without a namespace.
func clusterScoped(kind string) bool {
	return kind == "Namespace" || kind == "ClusterRole" || kind == "ClusterRoleBinding"
}

// Kube implements agent.Kube over a live cluster (read-only).
type Kube struct {
	Dyn  dynamic.Interface
	Core kubernetes.Interface
}

// New builds clients from kubeconfig ("" = default loading rules:
// KUBECONFIG or ~/.kube/config, current context).
func New(kubeconfig string) (*Kube, error) {
	loading := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loading.ExplicitPath = kubeconfig
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("livekube: kubeconfig: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("livekube: dynamic client: %w", err)
	}
	core, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("livekube: clientset: %w", err)
	}
	return &Kube{Dyn: dyn, Core: core}, nil
}

// Get returns one object as plain maps.
func (k *Kube) Get(ctx context.Context, kind, name, namespace string) (map[string]any, error) {
	gvr, ok := gvrOf(kind)
	if !ok {
		return nil, fmt.Errorf("livekube: unsupported kind %q", kind)
	}
	ns := namespace
	if clusterScoped(kind) {
		ns = ""
	}
	u, err := k.Dyn.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("livekube: get %s %s/%s: %w", kind, namespace, name, err)
	}
	return u.Object, nil
}

// List returns objects of kind; selector matches labels exactly.
func (k *Kube) List(ctx context.Context, kind, namespace string, selector map[string]string) ([]map[string]any, error) {
	gvr, ok := gvrOf(kind)
	if !ok {
		return nil, fmt.Errorf("livekube: unsupported kind %q", kind)
	}
	ns := namespace
	if clusterScoped(kind) {
		ns = ""
	}
	opts := metav1.ListOptions{}
	if len(selector) > 0 {
		opts.LabelSelector = labels.Set(selector).String()
	}
	l, err := k.Dyn.Resource(gvr).Namespace(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("livekube: list %s/%s: %w", kind, namespace, err)
	}
	out := make([]map[string]any, 0, len(l.Items))
	for _, u := range l.Items {
		out = append(out, u.Object)
	}
	return out, nil
}

// Logs returns the last tail lines of a container log.
func (k *Kube) Logs(ctx context.Context, namespace, pod, container string, tail int) (string, error) {
	var lines int64 = int64(tail)
	req := k.Core.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container: container,
		TailLines: &lines,
	})
	data, err := req.Do(ctx).Raw()
	if err != nil {
		return "", fmt.Errorf("livekube: logs %s/%s: %w", namespace, pod, err)
	}
	return string(data), nil
}

// DryRunner validates a document with a server-side dry-run CREATE in the
// sandbox namespace under a generated name: schema and admission are
// checked, nothing is persisted, real objects are never touched.
type DryRunner struct {
	Dyn dynamic.Interface
}

// DryRun implements invariants.DryRunner.
func (d DryRunner) DryRun(ctx context.Context, doc []byte) error {
	var obj map[string]any
	if err := jsonUnmarshal(doc, &obj); err != nil {
		return err
	}
	kind, _ := obj["kind"].(string)
	gvr, ok := gvrOf(kind)
	if !ok {
		return fmt.Errorf("livekube: dry-run of unsupported kind %q", kind)
	}
	u := &unstructured.Unstructured{Object: obj}
	u.SetNamespace(SandboxNamespace)
	u.SetGenerateName("umbra-dryrun-")
	u.SetName("")
	_, err := d.Dyn.Resource(gvr).Namespace(SandboxNamespace).Create(ctx, u, metav1.CreateOptions{DryRun: []string{"All"}})
	if err != nil {
		return fmt.Errorf("livekube: dry-run refused: %w", err)
	}
	return nil
}

// Sandbox applies documents to the sandbox namespace and keeps the
// previous object JSON as the rollback snapshot ("" when absent).
type Sandbox struct {
	Dyn dynamic.Interface
}

// Apply implements invariants.Sandbox.
func (s Sandbox) Apply(ctx context.Context, doc []byte) (string, error) {
	var obj map[string]any
	if err := jsonUnmarshal(doc, &obj); err != nil {
		return "", err
	}
	kind, _ := obj["kind"].(string)
	gvr, ok := gvrOf(kind)
	if !ok {
		return "", fmt.Errorf("livekube: apply of unsupported kind %q", kind)
	}
	if clusterScoped(kind) {
		return "", fmt.Errorf("livekube: refusing cluster-scoped apply of %s (sandbox is namespaced)", kind)
	}
	u := &unstructured.Unstructured{Object: obj}
	u.SetNamespace(SandboxNamespace)
	prev, err := s.Dyn.Resource(gvr).Namespace(SandboxNamespace).Get(ctx, u.GetName(), metav1.GetOptions{})
	snap := ""
	if err == nil {
		raw, _ := json.Marshal(prev.Object)
		snap = string(raw)
		u.SetResourceVersion(prev.GetResourceVersion())
		_, err = s.Dyn.Resource(gvr).Namespace(SandboxNamespace).Update(ctx, u, metav1.UpdateOptions{})
	} else if apierrors.IsNotFound(err) {
		u.SetResourceVersion("")
		_, err = s.Dyn.Resource(gvr).Namespace(SandboxNamespace).Create(ctx, u, metav1.CreateOptions{})
	} else {
		return "", fmt.Errorf("livekube: sandbox lookup: %w", err)
	}
	if err != nil {
		return "", fmt.Errorf("livekube: sandbox apply: %w", err)
	}
	return snap, nil
}

// CleanForPatch returns a copy of a live object with server-managed noise
// removed: managedFields, status, annotations, timestamps. Patch math
// (fixes.Fix, invariants.DiffPaths) runs on this; otherwise field names
// inside managedFields (f:resources, ...) trip naive text checks and the
// model burns tokens on kilobytes of bookkeeping.
func CleanForPatch(obj map[string]any) map[string]any {
	out := map[string]any{}
	if k, ok := obj["kind"].(string); ok {
		out["kind"] = k
	}
	if v, ok := obj["apiVersion"].(string); ok {
		out["apiVersion"] = v
	}
	if meta, ok := obj["metadata"].(map[string]any); ok {
		m := map[string]any{}
		for _, k := range []string{"name", "namespace", "labels"} {
			if v, ok := meta[k]; ok {
				m[k] = v
			}
		}
		if len(m) > 0 {
			out["metadata"] = m
		}
	}
	if spec, ok := obj["spec"]; ok {
		out["spec"] = spec
	}
	return out
}

// PodStatusOf builds a verifier PodStatus from a live pod object: phase,
// all-containers-ready, summed restarts and the last termination reason
// (waiting state first, then lastState).
func PodStatusOf(pod map[string]any) verify.PodStatus {
	var st verify.PodStatus
	status, _ := pod["status"].(map[string]any)
	st.Phase, _ = status["phase"].(string)
	cs, _ := status["containerStatuses"].([]any)
	st.Ready = len(cs) > 0
	for _, c := range cs {
		cm, _ := c.(map[string]any)
		if r, _ := cm["ready"].(bool); !r {
			st.Ready = false
		}
		if n, ok := cm["restartCount"].(float64); ok {
			st.Restarts += int32(n)
		}
		if cur, _ := cm["state"].(map[string]any); cur != nil {
			if tm, _ := cur["terminated"].(map[string]any); tm != nil {
				if r, _ := tm["reason"].(string); r != "" {
					st.LastTerminationReason = r
				}
			}
		}
		if st.LastTerminationReason == "" {
			if ls, _ := cm["lastState"].(map[string]any); ls != nil {
				if tm, _ := ls["terminated"].(map[string]any); tm != nil {
					if r, _ := tm["reason"].(string); r != "" {
						st.LastTerminationReason = r
					}
				}
			}
		}
	}
	return st
}

// TemplateLabels extracts pod-template labels for pod discovery.
func TemplateLabels(obj map[string]any) map[string]string {
	out := map[string]string{}
	spec, _ := obj["spec"].(map[string]any)
	tmpl, _ := spec["template"].(map[string]any)
	meta, _ := tmpl["metadata"].(map[string]any)
	if l, _ := meta["labels"].(map[string]any); l != nil {
		for k, v := range l {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

// Rollback restores snapshot ("" = object did not exist: delete it).
func (s Sandbox) Rollback(ctx context.Context, kind, name, snapshot string) error {
	gvr, ok := gvrOf(kind)
	if !ok {
		return fmt.Errorf("livekube: rollback of unsupported kind %q", kind)
	}
	res := s.Dyn.Resource(gvr).Namespace(SandboxNamespace)
	if snapshot == "" {
		return res.Delete(ctx, name, metav1.DeleteOptions{})
	}
	var obj map[string]any
	if err := jsonUnmarshal([]byte(snapshot), &obj); err != nil {
		return err
	}
	cur, err := res.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	meta, ok := obj["metadata"].(map[string]any)
	if !ok {
		return fmt.Errorf("livekube: rollback snapshot has no metadata")
	}
	meta["resourceVersion"] = cur.GetResourceVersion()
	_, err = res.Update(ctx, &unstructured.Unstructured{Object: obj}, metav1.UpdateOptions{})
	return err
}
