package livekube

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestGvrTable(t *testing.T) {
	for _, kind := range []string{"Deployment", "Pod", "Service", "CronJob", "Job",
		"HorizontalPodAutoscaler", "Ingress", "Role", "ClusterRoleBinding", "Event"} {
		if _, ok := gvrOf(kind); !ok {
			t.Fatalf("kind %q must be mapped", kind)
		}
	}
	if _, ok := gvrOf("ClusterRole"); ok {
		_ = ok
	}
	if _, ok := gvrOf("Exec"); ok {
		t.Fatal("arbitrary kinds must fail closed")
	}
	if !clusterScoped("ClusterRoleBinding") || clusterScoped("Deployment") {
		t.Fatal("cluster scope wrong")
	}
	if SandboxNamespace != "umbra-sandbox" {
		t.Fatalf("sandbox namespace changed: %q", SandboxNamespace)
	}
}

func TestPodStatusOf(t *testing.T) {
	pod := map[string]any{"status": map[string]any{
		"phase": "Running",
		"containerStatuses": []any{map[string]any{
			"ready": true, "restartCount": float64(3),
			"lastState": map[string]any{"terminated": map[string]any{"reason": "OOMKilled"}},
		}},
	}}
	st := PodStatusOf(pod)
	if !st.Ready || st.Phase != "Running" || st.Restarts != 3 || st.LastTerminationReason != "OOMKilled" {
		t.Fatalf("bad status: %+v", st)
	}
	if got := PodStatusOf(map[string]any{}); got.Ready || got.Phase != "" {
		t.Fatalf("empty pod must yield zero status: %+v", got)
	}
}

func TestTemplateLabels(t *testing.T) {
	obj := map[string]any{"spec": map[string]any{"template": map[string]any{
		"metadata": map[string]any{"labels": map[string]any{"app": "web"}},
	}}}
	if got := TemplateLabels(obj); got["app"] != "web" {
		t.Fatalf("labels = %v", got)
	}
	if got := TemplateLabels(map[string]any{}); len(got) != 0 {
		t.Fatalf("empty obj must yield no labels: %v", got)
	}
}

func TestDryRunRefusesClusterScoped(t *testing.T) {
	// The refusal happens before any client call (nil Dyn would panic
	// only if reached), so no fake cluster is needed.
	d := DryRunner{}
	for _, kind := range []string{"Namespace", "ClusterRole", "ClusterRoleBinding"} {
		doc := []byte("apiVersion: v1\nkind: " + kind + "\nmetadata:\n  name: x\n")
		if err := d.DryRun(context.Background(), doc); err == nil {
			t.Fatalf("dry-run of %s must be refused", kind)
		} else if !strings.Contains(err.Error(), "cluster-scoped") {
			t.Fatalf("wrong refusal for %s: %v", kind, err)
		}
	}
}

func TestCleanForPatch(t *testing.T) {
	obj := map[string]any{
		"kind": "Deployment", "apiVersion": "apps/v1",
		"metadata": map[string]any{
			"name": "w", "namespace": "demo",
			"uid": "x", "resourceVersion": "1",
			"annotations":   map[string]any{"kubectl.kubernetes.io/last-applied-configuration": "{}"},
			"managedFields": []any{map[string]any{"manager": "kubectl"}},
		},
		"spec":   map[string]any{"replicas": 1},
		"status": map[string]any{"readyReplicas": 1},
	}
	got := CleanForPatch(obj)
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "managedFields") || strings.Contains(string(raw), "f:resources") ||
		strings.Contains(string(raw), "status") || strings.Contains(string(raw), "last-applied") {
		t.Fatalf("noise survives: %s", raw)
	}
	// The text check behind fixes.Fix must not trip on server noise.
	if got["kind"] != "Deployment" || got["spec"] == nil {
		t.Fatalf("identity/spec lost: %v", got)
	}
}
