package gitops

import (
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name   string
		obj    map[string]any
		system string
	}{
		{"argo instance annotation", map[string]any{"metadata": map[string]any{"labels": map[string]any{"app.kubernetes.io/instance": "shop"}, "annotations": map[string]any{"argocd.argopoint.io/instance": "shop"}}}, "argo"},
		{"argo managed-by", map[string]any{"metadata": map[string]any{"labels": map[string]any{"app.kubernetes.io/managed-by": "argoproj", "app.kubernetes.io/instance": "shop"}}}, "argo"},
		{"flux kustomize", map[string]any{"metadata": map[string]any{"labels": map[string]any{"kustomize.toolkit.fluxcd.io/name": "shop"}}}, "flux"},
		{"flux helm", map[string]any{"metadata": map[string]any{"labels": map[string]any{"helm.toolkit.fluxcd.io/name": "shop"}}}, "flux"},
		{"flux managed-by", map[string]any{"metadata": map[string]any{"labels": map[string]any{"app.kubernetes.io/managed-by": "flux", "app.kubernetes.io/instance": "shop"}}}, "flux"},
		{"none", map[string]any{"metadata": map[string]any{"labels": map[string]any{"app": "shop"}}}, "none"},
		{"empty", map[string]any{}, "none"},
	}
	for _, c := range cases {
		if got := Detect(c.obj).System; got != c.system {
			t.Errorf("%s: got %s, want %s", c.name, got, c.system)
		}
	}
}

func TestManagedFieldsManagers(t *testing.T) {
	obj := map[string]any{"metadata": map[string]any{"managedFields": []any{
		map[string]any{"manager": "kubectl-client-side-apply", "operation": "Update"},
		map[string]any{"manager": "argocd-application-controller", "operation": "Apply"},
		map[string]any{"manager": "kubectl-client-side-apply", "operation": "Update"},
	}}}
	got := ManagedFieldsManagers(obj)
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 managers", got)
	}
	if got[0] != "kubectl-client-side-apply" || got[1] != "argocd-application-controller" {
		t.Fatalf("got %v", got)
	}
	if len(ManagedFieldsManagers(map[string]any{})) != 0 {
		t.Fatal("empty object should yield no managers")
	}
}

func TestIsGitOps(t *testing.T) {
	o := Owner{System: "argo"}
	if !o.IsGitOps() {
		t.Fatal("argo should be gitops")
	}
	o2 := Owner{System: "none"}
	if o2.IsGitOps() {
		t.Fatal("none should not be gitops")
	}
}
