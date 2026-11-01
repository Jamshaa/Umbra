package template

import (
	"strings"
	"testing"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/evidence"
	"github.com/Jamshaa/Umbra/internal/report"
)

func mustObj(t *testing.T, kind, ns, name string, spec map[string]any) map[string]any {
	t.Helper()
	meta := map[string]any{"name": name}
	if ns != "" {
		meta["namespace"] = ns
	}
	return map[string]any{"kind": kind, "metadata": meta, "spec": spec}
}

func podSpec(containers ...map[string]any) map[string]any {
	cs := make([]any, 0, len(containers))
	for _, c := range containers {
		cs = append(cs, c)
	}
	return map[string]any{"template": map[string]any{"spec": map[string]any{"containers": cs}}}
}

func apiTemplate(t *testing.T, drift string) v1alpha1.ComplianceTemplate {
	t.Helper()
	tpl := v1alpha1.ComplianceTemplate{}
	tpl.Name = "api-standard"
	tpl.Spec.AppliesTo = []v1alpha1.TargetKind{{Kind: "Deployment"}}
	tpl.Spec.Required.Resources = &v1alpha1.ResourceRequirements{
		Limits:   map[string]v1alpha1.ResourceRule{"memory": {Min: "128Mi", Required: true}},
		Requests: map[string]v1alpha1.ResourceRule{"cpu": {Min: "100m", Required: true}},
	}
	tpl.Spec.Required.Probes = &v1alpha1.ProbeRequirements{Liveness: v1alpha1.ProbePolicyRequired}
	tpl.Spec.Required.Image = &v1alpha1.ImageRequirements{TagPolicy: v1alpha1.ImageTagPolicyExplicit}
	tpl.Spec.Drift.OnViolation = drift
	return tpl
}

func bareContainer() map[string]any {
	return map[string]any{
		"name":  "app",
		"image": "example/web:1",
		"resources": map[string]any{
			"limits":   map[string]any{"memory": "256Mi"},
			"requests": map[string]any{"cpu": "100m"},
		},
		"livenessProbe": map[string]any{"tcpSocket": map[string]any{"port": 8080}},
	}
}

func TestEnforceClean(t *testing.T) {
	obj := mustObj(t, "Deployment", "demo", "web", podSpec(bareContainer()))
	if _, ok := Enforce(apiTemplate(t, ""), obj, AttributedBy{}); ok {
		t.Fatal("clean object must produce no assessment")
	}
}

func TestEnforceDefaultReport(t *testing.T) {
	bad := bareContainer()
	delete(bad, "livenessProbe")
	obj := mustObj(t, "Deployment", "demo", "web", podSpec(bad))
	a, ok := Enforce(apiTemplate(t, ""), obj, AttributedBy{By: "kubectl", Confidence: "client"})
	if !ok {
		t.Fatal("want assessment")
	}
	if a.Action != ActionReport {
		t.Fatalf("default drift must be report, got %q", a.Action)
	}
	f := a.Finding
	if f.Spec.Detector != "template" || f.Spec.Category != v1alpha1.FindingCategoryConfig {
		t.Fatalf("bad finding identity: %+v", f.Spec)
	}
	if f.Spec.TargetRef.Kind != "Deployment" || f.Spec.TargetRef.Name != "web" {
		t.Fatalf("bad target: %+v", f.Spec.TargetRef)
	}
	if !strings.Contains(f.Spec.Diagnosis, "livenessProbe") || !strings.Contains(f.Spec.Diagnosis, "kubectl (client)") {
		t.Fatalf("diagnosis must carry diff + attribution:\n%s", f.Spec.Diagnosis)
	}
	if len(f.Status.Timeline) != 1 || f.Status.Timeline[0].Tool != "template-evaluate" {
		t.Fatalf("want one template-evaluate timeline entry: %+v", f.Status.Timeline)
	}
	want := evidence.Fingerprint("api-standard\n1 violation(s)")
	if f.Status.Timeline[0].EvidenceHash != want {
		t.Fatalf("bad evidence hash: %q", f.Status.Timeline[0].EvidenceHash)
	}
	if got := f.Name; !strings.HasPrefix(got, "tpl-api-standard-deployment-web") {
		t.Fatalf("non-deterministic name: %q", got)
	}
	// Deterministic: twice yields identical output.
	a2, _ := Enforce(apiTemplate(t, ""), obj, AttributedBy{By: "kubectl", Confidence: "client"})
	if a2.Finding.Spec.Diagnosis != f.Spec.Diagnosis || a2.Finding.Name != f.Name {
		t.Fatal("Enforce must be deterministic")
	}
}

func TestEnforceAutofixGate(t *testing.T) {
	resOnly := bareContainer()
	delete(resOnly, "resources")
	obj := mustObj(t, "Deployment", "demo", "web", podSpec(resOnly))
	a, ok := Enforce(apiTemplate(t, "autofix"), obj, AttributedBy{})
	if !ok || a.Action != ActionAutofix {
		t.Fatalf("resources-only drift must autofix, got %+v", a.Action)
	}
	img := bareContainer()
	img["image"] = "example/web:latest"
	obj2 := mustObj(t, "Deployment", "demo", "web", podSpec(img))
	a2, ok := Enforce(apiTemplate(t, "autofix"), obj2, AttributedBy{})
	if !ok || a2.Action != ActionPR {
		t.Fatalf("image drift must downgrade to pr, got %+v", a2.Action)
	}
}

func TestEnforceExplicitPR(t *testing.T) {
	bad := bareContainer()
	delete(bad, "resources")
	obj := mustObj(t, "Deployment", "demo", "web", podSpec(bad))
	a, ok := Enforce(apiTemplate(t, "pr"), obj, AttributedBy{})
	if !ok || a.Action != ActionPR {
		t.Fatalf("explicit pr must stay pr, got %+v", a.Action)
	}
}

func TestEnforceCronJob(t *testing.T) {
	tpl := v1alpha1.ComplianceTemplate{}
	tpl.Name = "cron-standard"
	tpl.Spec.AppliesTo = []v1alpha1.TargetKind{{Kind: "CronJob"}}
	tpl.Spec.Required.ConcurrencyPolicy = "Forbid"
	obj := mustObj(t, "CronJob", "demo", "nightly", map[string]any{})
	a, ok := Enforce(tpl, obj, AttributedBy{By: "argo", Confidence: "client"})
	if !ok || a.Action != ActionReport {
		t.Fatalf("cronjob drift must report: %+v", a.Action)
	}
	if !strings.Contains(a.Finding.Spec.Diagnosis, "concurrencyPolicy") {
		t.Fatalf("missing concurrency diff:\n%s", a.Finding.Spec.Diagnosis)
	}
}

func TestToReport(t *testing.T) {
	rs := ResultsForReport("api-standard", "Deployment", "demo", "web", []Violation{
		{Kind: "Deployment", Namespace: "demo", Name: "web", Field: "resources.limits.memory", Expected: ">=128Mi", Actual: "absent", Message: "resource required by template"},
	})
	if len(rs) != 1 {
		t.Fatalf("want 1 result, got %v", rs)
	}
	r := rs[0]
	if r.Policy != "api-standard" || r.Rule != "resources.limits.memory" || r.Outcome != report.Fail {
		t.Fatalf("bad result mapping: %+v", r)
	}
	if r.Resource != "Deployment/demo/web" {
		t.Fatalf("bad resource ref: %q", r.Resource)
	}
	out := string(report.Build("demo", "api-standard", rs))
	for _, want := range []string{"fail: 1", "resources.limits.memory", "Deployment/demo/web"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, out)
		}
	}
}
