package fixes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jamshaa/Umbra/internal/verify"
)

const f01Manifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: f01
  namespace: demo
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
          ports:
            - containerPort: 8080
`

const f02Manifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: f02
  namespace: demo
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
          resources:
            requests:
              cpu: 10m
              memory: 8Mi
            limits:
              cpu: 100m
              memory: 8Mi
          livenessProbe: {tcpSocket: {port: 8080}}
          readinessProbe: {tcpSocket: {port: 8080}}
          ports:
            - containerPort: 8080
`

const f05Manifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: f05
  namespace: demo
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
          resources:
            requests: {cpu: 10m, memory: 64Mi}
            limits: {cpu: 100m, memory: 128Mi}
          readinessProbe: {tcpSocket: {port: 8080}}
          ports:
            - containerPort: 8080
`

func TestFixF01(t *testing.T) {
	out, ok := Fix("F01", []byte(f01Manifest))
	if !ok {
		t.Fatal("F01 should apply")
	}
	if !stringsContains(string(out), "resources:") {
		t.Fatalf("resources missing: %s", out)
	}
	// original content preserved
	if !stringsContains(string(out), "image: ghcr.io/example/demo-app:v1") {
		t.Fatal("manifest clobbered")
	}
	// unknown fault: no change
	out2, ok2 := Fix("F99", []byte(f01Manifest))
	if ok2 || string(out2) != f01Manifest {
		t.Fatal("unknown fault must be a no-op")
	}
}

func TestFixF02(t *testing.T) {
	out, ok := Fix("F02", []byte(f02Manifest))
	if !ok {
		t.Fatal("F02 should apply")
	}
	if !stringsContains(string(out), "memory: 64Mi") {
		t.Fatalf("limit not raised: %s", out)
	}
	// a manifest already sane is untouched
	sane := stringsReplace(f02Manifest, "8Mi", "128Mi")
	if _, ok := Fix("F02", []byte(sane)); ok {
		t.Fatal("sane manifest should not be patched")
	}
}

func TestFixF05(t *testing.T) {
	out, ok := Fix("F05", []byte(f05Manifest))
	if !ok {
		t.Fatal("F05 should apply")
	}
	if !stringsContains(string(out), "livenessProbe:") {
		t.Fatalf("probe missing: %s", out)
	}
	if stringsContains(string(out), "livenessProbe:\n            tcpSocket:") == false {
		t.Fatal("probe malformed")
	}
}

func TestFixF06(t *testing.T) {
	m := stringsReplace(f05Manifest, "readinessProbe", "livenessProbe")
	// build a manifest with liveness but no readiness
	m = stringsReplace(m, "livenessProbe: {tcpSocket: {port: 8080}}", "livenessProbe: {tcpSocket: {port: 8080}}")
	out, ok := Fix("F06", []byte(m))
	if !ok {
		t.Fatal("F06 should apply")
	}
	if !stringsContains(string(out), "readinessProbe:") {
		t.Fatalf("readiness missing: %s", out)
	}
}

func TestFixUnknown(t *testing.T) {
	for _, id := range []string{"F07", "F13", "F20", ""} {
		out, ok := Fix(id, []byte(f01Manifest))
		if ok || string(out) != f01Manifest {
			t.Fatalf("%s: expected no-op, got ok=%v", id, ok)
		}
	}
}

// TestFixF06DemoManifest ties the demo manifest to the fix and the
// verifier: the shipped F06 yaml must be patched by Fix and the
// result must pass VerifyF06 for a Running+Ready pod.
func TestFixF06DemoManifest(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "demo", "f06_no_readiness.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if stringsContains(string(raw), "readinessProbe:") {
		t.Fatal("demo manifest must ship without readinessProbe")
	}
	out, ok := Fix("F06", raw)
	if !ok {
		t.Fatal("F06 should apply to the demo manifest")
	}
	res := verify.VerifyF06(out, verify.PodStatus{Phase: "Running", Ready: true})
	if !res.Pass {
		t.Fatalf("patched demo manifest fails VerifyF06: %s", res.Reason)
	}
}

// TestFixF01ServerNormalized covers the API-server form: a missing block
// round-trips as "resources: {}" (or bare "resources:"). Both must be
// treated as absent; a block with real content must stay untouched.
func TestFixF01ServerNormalized(t *testing.T) {
	for _, empty := range []string{"          resources: {}\n", "          resources:\n", "          resources: { }\n"} {
		m := stringsReplace(f01Manifest, "          ports:\n", empty+"          ports:\n")
		out, ok := Fix("F01", []byte(m))
		if !ok {
			t.Fatalf("empty form %q should be fixed", empty)
		}
		if !stringsContains(string(out), "memory: \"512Mi\"") {
			t.Fatalf("limits not added for %q", empty)
		}
	}
}

// TestFixF01LiveIndent mirrors kubectl output (8-space indent, empty
// resources map): the fix must apply regardless of indent.
func TestFixF01LiveIndent(t *testing.T) {
	m := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: live
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
          ports:
            - containerPort: 8080
          resources: {}
`
	out, ok := Fix("F01", []byte(m))
	if !ok {
		t.Fatal("F01 should apply to kubectl-style manifest")
	}
	for _, want := range []string{"        resources:\n", "          cpu: \"500m\"", "          memory: \"512Mi\""} {
		if !stringsContains(string(out), want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	if !stringsContains(string(out), "        ports:\n") {
		t.Fatalf("ports anchor disturbed:\n%s", out)
	}
}

func TestFixF03F04(t *testing.T) {
	huge := `apiVersion: apps/v1
kind: Deployment
metadata: {name: f03, namespace: demo}
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
          resources:
            requests: {cpu: "64", memory: "1Ti"}
            limits: {cpu: "64", memory: "1Ti"}
`
	out, ok := Fix("F03", []byte(huge))
	if !ok {
		t.Fatal("F03 should apply")
	}
	if stringsContains(string(out), "64") || stringsContains(string(out), "1Ti") {
		t.Fatalf("huge values survive: %s", out)
	}
	if r := verify.VerifyF03(out, verify.PodStatus{Phase: "Running", Ready: true}); !r.Pass {
		t.Fatalf("patched F03 fails gate: %s", r.Reason)
	}
	out, ok = Fix("F04", []byte(f01Manifest))
	if !ok || !stringsContains(string(out), "resources:") {
		t.Fatalf("F04 should add resources: %s", out)
	}
}

// tiny helpers to keep the test free of import noise.
func stringsContains(s, sub string) bool       { return strings.Contains(s, sub) }
func stringsReplace(s, old, neu string) string { return strings.ReplaceAll(s, old, neu) }
