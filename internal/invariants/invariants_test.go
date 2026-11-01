package invariants

import (
	"testing"
)

const deployNoResources = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: umbra-sandbox
spec:
  template:
    spec:
      containers:
        - name: app
          image: example/web:1
          ports:
            - containerPort: 8080
`

const deployFixed = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: umbra-sandbox
spec:
  template:
    spec:
      containers:
        - name: app
          image: example/web:1
          resources:
            requests: {cpu: 100m, memory: 64Mi}
            limits: {cpu: 500m, memory: 512Mi}
          ports:
            - containerPort: 8080
`

func mustDoc(t *testing.T, s string) map[string]any {
	t.Helper()
	d, err := decodeOne([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDiffPaths(t *testing.T) {
	before := mustDoc(t, `{"a":1,"b":{"x":[1,2],"y":"k"},"gone":true}`)
	after := mustDoc(t, `{"a":2,"b":{"x":[1,3],"y":"k"},"new":"n"}`)
	got := DiffPaths(before, after)
	want := []string{"/a", "/b/x/1", "/gone", "/new"}
	if len(got) != len(want) {
		t.Fatalf("diff = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("diff = %v, want %v", got, want)
		}
	}
	// Escapes: keys with / and ~ round-trip, sorted output.
	b2 := mustDoc(t, `{"a/b":1,"a~b":2}`)
	a2 := mustDoc(t, `{"a/b":1,"a~b":3}`)
	got = DiffPaths(b2, a2)
	if len(got) != 1 || got[0] != "/a~0b" {
		t.Fatalf("escaped diff = %v", got)
	}
}

func TestAllowlist(t *testing.T) {
	ok := []string{"/spec/template/spec/containers/0/resources/limits/memory", "/spec/containers/2/resources"}
	for _, p := range ok {
		if err := CheckAllowlist("F01", []string{p}); err != nil {
			t.Fatalf("F01 should allow %s: %v", p, err)
		}
	}
	for _, tc := range []struct{ rule, path string }{
		{"F01", "/spec/template/spec/containers/0/image"},
		{"F05", "/spec/template/spec/containers/0/resources"},
		{"F05", "/spec/template/spec/containers/0/readinessProbe"},
		{"NOPE", "/spec/anything"},
	} {
		if err := CheckAllowlist(tc.rule, []string{tc.path}); err == nil {
			t.Fatalf("rule %s must reject %s", tc.rule, tc.path)
		}
	}
}

func TestIdentity(t *testing.T) {
	base := mustDoc(t, deployNoResources)
	if err := checkIdentity(base, mustDoc(t, deployNoResources)); err != nil {
		t.Fatalf("identical docs rejected: %v", err)
	}
	renamed := mustDoc(t, deployNoResources)
	renamed["metadata"].(map[string]any)["name"] = "other"
	if err := checkIdentity(base, renamed); err == nil {
		t.Fatal("name change accepted")
	}
	// Selector is immutable for Deployments.
	selChanged := mustDoc(t, deployNoResources)
	tmpl := selChanged["spec"].(map[string]any)["template"].(map[string]any)
	tmpl["metadata"] = map[string]any{"labels": map[string]any{"app": "other"}}
	_ = tmpl
	selChanged["spec"].(map[string]any)["selector"] = map[string]any{"matchLabels": map[string]any{"app": "other"}}
	if err := checkIdentity(base, selChanged); err == nil {
		t.Fatal("selector change accepted")
	}
	// Anything outside umbra-sandbox is refused.
	prod := mustDoc(t, deployNoResources)
	prod["metadata"].(map[string]any)["namespace"] = "demo"
	if err := checkIdentity(base, prod); err == nil {
		t.Fatal("non-sandbox target accepted")
	}
}

func TestRBAC(t *testing.T) {
	role := `{"kind":"Role","metadata":{"name":"r","namespace":"umbra-sandbox"},"rules":[{"apiGroups":[""],"resources":["pods"],"verbs":["get"]}]}`
	wider := `{"kind":"Role","metadata":{"name":"r","namespace":"umbra-sandbox"},"rules":[{"apiGroups":[""],"resources":["pods"],"verbs":["get","list"]}]}`
	if err := checkRBAC(mustDoc(t, role), mustDoc(t, role)); err != nil {
		t.Fatalf("identical role rejected: %v", err)
	}
	if err := checkRBAC(mustDoc(t, role), mustDoc(t, wider)); err == nil {
		t.Fatal("verb widening accepted")
	}
	bind := `{"kind":"RoleBinding","metadata":{"name":"b","namespace":"umbra-sandbox"},"subjects":[{"kind":"ServiceAccount","name":"a"}],"roleRef":{"kind":"Role","name":"r"}}`
	bind2 := `{"kind":"RoleBinding","metadata":{"name":"b","namespace":"umbra-sandbox"},"subjects":[{"kind":"ServiceAccount","name":"a"},{"kind":"ServiceAccount","name":"evil"}],"roleRef":{"kind":"Role","name":"r"}}`
	if err := checkRBAC(mustDoc(t, bind), mustDoc(t, bind2)); err == nil {
		t.Fatal("added subject accepted")
	}
	if err := checkRBAC(mustDoc(t, deployNoResources), mustDoc(t, deployFixed)); err != nil {
		t.Fatalf("non-RBAC kinds must pass: %v", err)
	}
}

func TestSizeAndSecrets(t *testing.T) {
	if err := checkSize(make([]byte, MaxDocBytes+1)); err == nil {
		t.Fatal("oversize doc accepted")
	}
	if err := checkSecrets([]byte("image: example/web:1\n")); err != nil {
		t.Fatalf("clean doc rejected: %v", err)
	}
	if err := checkSecrets([]byte("token: AKIAIOSFODNN7EXAMPLE\n")); err == nil {
		t.Fatal("AKIA key accepted")
	}
	if err := checkSecrets([]byte("password: hunter2-hunter2\n")); err == nil {
		t.Fatal("secret value accepted")
	}
}

func TestResourcesVsUsage(t *testing.T) {
	after := mustDoc(t, deployFixed)
	usage := map[string]ResourceUsage{"app": {CPU: "10m", Memory: "300Mi"}}
	if _, err := checkResources(mustDoc(t, deployNoResources), after, usage); err == nil {
		t.Fatal("limit below usage accepted")
	}
	usage = map[string]ResourceUsage{"app": {CPU: "10m", Memory: "64Mi"}}
	if skipped, err := checkResources(mustDoc(t, deployNoResources), after, usage); err != nil || len(skipped) != 0 {
		t.Fatalf("sane limits rejected: %v skipped=%v", err, skipped)
	}
	skipped, err := checkResources(mustDoc(t, deployNoResources), after, nil)
	if err != nil || len(skipped) != 1 || skipped[0] != "app" {
		t.Fatalf("unknown usage must skip openly: %v %v", err, skipped)
	}
}
