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

// tiny helpers to keep the test free of import noise.
func stringsContains(s, sub string) bool       { return strings.Contains(s, sub) }
func stringsReplace(s, old, neu string) string { return strings.ReplaceAll(s, old, neu) }
