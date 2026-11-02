package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestGetChangesAndImpact(t *testing.T) {
	k := newFakeKube()
	k.objs[kkey("Deployment", "demo", "web")] = depObj("demo", "web")
	k.objs[kkey("Service", "demo", "web-svc")] = map[string]any{
		"kind": "Service", "metadata": map[string]any{"name": "web-svc", "namespace": "demo"},
		"spec": map[string]any{"selector": map[string]any{"app": "web"}},
	}

	out, prop, err := dispatch(context.Background(), k, Scope{}, "get_changes",
		map[string]any{"kind": "Deployment", "name": "web", "namespace": "demo"})
	if err != nil || prop != nil {
		t.Fatalf("get_changes: %v %v", out, err)
	}
	if out != "no changes in the last 2h" {
		t.Fatalf("no managedFields/events expected, got %q", out)
	}

	out, prop, err = dispatch(context.Background(), k, Scope{}, "get_impact",
		map[string]any{"kind": "Deployment", "name": "web", "namespace": "demo"})
	if err != nil || prop != nil {
		t.Fatalf("get_impact: %v %v", out, err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("impact is not JSON: %v", err)
	}
	if _, ok := v["score"]; !ok {
		t.Fatalf("impact missing score: %s", out)
	}
	if !strings.Contains(out, "web-svc") {
		t.Fatalf("impact must name the selecting service: %s", out)
	}
	// Secret names stay, but values never appear (fake has none anyway).
	if strings.Contains(out, "secretValue") {
		t.Fatalf("impact leaks secret values: %s", out)
	}
}

func TestNewToolsAreListed(t *testing.T) {
	seen := map[string]bool{}
	for _, td := range Tools() {
		seen[td.Name] = true
	}
	for _, want := range []string{"get_changes", "get_impact"} {
		if !seen[want] {
			t.Fatalf("tool %q missing from Tools()", want)
		}
	}
}
