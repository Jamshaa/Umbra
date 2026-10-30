package template

import (
	"testing"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
)

func TestEvaluate(t *testing.T) {
	tpl := v1alpha1.ComplianceTemplate{Spec: v1alpha1.ComplianceTemplateSpec{
		AppliesTo: []v1alpha1.TargetKind{{Kind: "Deployment", MatchLabels: map[string]string{"app.kubernetes.io/component": "api"}}},
		Required: v1alpha1.RequiredSpec{
			Resources: &v1alpha1.ResourceRequirements{
				Limits:   map[string]v1alpha1.ResourceRule{"cpu": {Min: "500m"}, "memory": {Min: "512Mi"}, "nvidia.com/gpu": {Required: true}},
				Requests: map[string]v1alpha1.ResourceRule{"cpu": {Min: "250m"}, "memory": {Min: "256Mi"}},
			},
			Probes: &v1alpha1.ProbeRequirements{Liveness: v1alpha1.ProbePolicyRequired, Readiness: v1alpha1.ProbePolicyRequired},
			Image:  &v1alpha1.ImageRequirements{TagPolicy: v1alpha1.ImageTagPolicyExplicit},
		},
	}}
	good := map[string]any{
		"kind":     "Deployment",
		"metadata": map[string]any{"name": "api", "namespace": "demo", "labels": map[string]any{"app.kubernetes.io/component": "api"}},
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{
			"name": "app", "image": "ghcr.io/example/demo-app:v1.2.3",
			"resources":     map[string]any{"limits": map[string]any{"cpu": "1", "memory": "1Gi", "nvidia.com/gpu": "1"}, "requests": map[string]any{"cpu": "500m", "memory": "512Mi", "nvidia.com/gpu": "1"}},
			"livenessProbe": map[string]any{}, "readinessProbe": map[string]any{},
		}}}}},
	}
	if vs := Evaluate(tpl, []map[string]any{good}); len(vs) != 0 {
		t.Fatalf("good manifest flagged: %+v", vs)
	}
	bad := map[string]any{
		"kind":     "Deployment",
		"metadata": map[string]any{"name": "api", "namespace": "demo", "labels": map[string]any{"app.kubernetes.io/component": "api"}},
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{
			"name": "app", "image": "ghcr.io/example/demo-app:latest",
			"resources": map[string]any{"limits": map[string]any{"cpu": "100m", "memory": "64Mi"}, "requests": map[string]any{"cpu": "10m", "memory": "8Mi"}},
		}}}}},
	}
	vs := Evaluate(tpl, []map[string]any{bad})
	if len(vs) < 7 {
		t.Fatalf("want >=7 violations, got %d: %+v", len(vs), vs)
	}
	// a non-matching kind is ignored
	ignored := map[string]any{"kind": "Pod", "metadata": map[string]any{"name": "x", "namespace": "demo"}, "spec": map[string]any{"containers": []any{}}}
	if vs := Evaluate(tpl, []map[string]any{ignored}); len(vs) != 0 {
		t.Fatalf("non-matching kind flagged: %+v", vs)
	}
	// GPU requests != limits is a violation
	gpu := map[string]any{
		"kind":     "Deployment",
		"metadata": map[string]any{"name": "g", "namespace": "demo", "labels": map[string]any{"app.kubernetes.io/component": "api"}},
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{
			"name": "app", "image": "ghcr.io/example/demo-app:v1",
			"resources":     map[string]any{"limits": map[string]any{"nvidia.com/gpu": "2"}, "requests": map[string]any{"nvidia.com/gpu": "1"}},
			"livenessProbe": map[string]any{}, "readinessProbe": map[string]any{},
		}}}}},
	}
	if vs := Evaluate(tpl, []map[string]any{gpu}); len(vs) == 0 {
		t.Fatal("gpu mismatch not flagged")
	}
}
