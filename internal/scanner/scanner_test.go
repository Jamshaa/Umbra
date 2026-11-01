package scanner

import (
	"testing"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
)

const healthyDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shop
  namespace: demo
spec:
  template:
    spec:
      imagePullSecrets:
        - name: registry-creds
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
          resources:
            requests: {cpu: "10m", memory: "64Mi"}
            limits: {cpu: "100m", memory: "128Mi"}
          livenessProbe: {tcpSocket: {port: 8080}}
          readinessProbe: {httpGet: {path: /readyz, port: 8080}}
`

const noResourcesDeployment = `
apiVersion: apps/v1
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
`

const tinyMemoryDeployment = `
apiVersion: apps/v1
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
            requests: {cpu: "10m", memory: "8Mi"}
            limits: {cpu: "100m", memory: "8Mi"}
          livenessProbe: {tcpSocket: {port: 8080}}
          readinessProbe: {tcpSocket: {port: 8080}}
`

const hugeRequestsDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: f03
  namespace: demo
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
          resources:
            requests: {cpu: "64", memory: "1Ti"}
            limits: {cpu: "64", memory: "1Ti"}
          livenessProbe: {tcpSocket: {port: 8080}}
          readinessProbe: {tcpSocket: {port: 8080}}
`

func kinds(fs []v1alpha1.Finding) map[string]bool {
	out := map[string]bool{}
	for _, f := range fs {
		out[string(f.Spec.Category)+"/"+f.Spec.TargetRef.Name] = true
	}
	return out
}

func TestDetectNoResources(t *testing.T) {
	fs := DetectNoResources([]byte(noResourcesDeployment))
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d", len(fs))
	}
	if fs[0].Spec.Detector != DetectorName || fs[0].Spec.State != v1alpha1.FindingStateDetected {
		t.Fatalf("bad finding: %+v", fs[0].Spec)
	}
	if got := DetectNoResources([]byte(healthyDeployment)); len(got) != 0 {
		t.Fatalf("healthy manifest flagged: %+v", got)
	}
}

func TestDetectTinyMemoryLimit(t *testing.T) {
	if got := DetectTinyMemoryLimit([]byte(tinyMemoryDeployment)); len(got) != 1 {
		t.Fatalf("want 1 finding, got %d", len(got))
	}
	for _, doc := range []string{healthyDeployment, noResourcesDeployment, hugeRequestsDeployment} {
		if got := DetectTinyMemoryLimit([]byte(doc)); len(got) != 0 {
			t.Fatalf("false positive on %q: %+v", doc[:40], got)
		}
	}
}

func TestDetectHugeRequests(t *testing.T) {
	if got := DetectHugeRequests([]byte(hugeRequestsDeployment)); len(got) != 1 {
		t.Fatalf("want 1 finding, got %d", len(got))
	}
	if got := DetectHugeRequests([]byte(healthyDeployment)); len(got) != 0 {
		t.Fatalf("healthy manifest flagged: %+v", got)
	}
}

func TestDetectProbes(t *testing.T) {
	// F01 fixture has neither probe: both detectors fire.
	if got := DetectNoLivenessProbe([]byte(noResourcesDeployment)); len(got) != 1 {
		t.Fatalf("want 1 liveness finding, got %d", len(got))
	}
	if got := DetectNoReadinessProbe([]byte(noResourcesDeployment)); len(got) != 1 {
		t.Fatalf("want 1 readiness finding, got %d", len(got))
	}
	// Healthy fixture defines both probes: silence.
	if got := DetectNoLivenessProbe([]byte(healthyDeployment)); len(got) != 0 {
		t.Fatalf("liveness false positive: %+v", got)
	}
	if got := DetectNoReadinessProbe([]byte(healthyDeployment)); len(got) != 0 {
		t.Fatalf("readiness false positive: %+v", got)
	}
}

func TestScanFindsAllFaults(t *testing.T) {
	docs := noResourcesDeployment + "\n---\n" + tinyMemoryDeployment + "\n---\n" + hugeRequestsDeployment
	fs := Scan([]byte(docs))
	if len(fs) == 0 {
		t.Fatal("no findings at all")
	}
	seen := kinds(fs)
	for _, want := range []string{"Resources/f01", "Resources/f02", "Resources/f03"} {
		if !seen[want] {
			t.Fatalf("missing %s in %v", want, seen)
		}
	}
	for _, f := range fs {
		if f.Spec.Detector != DetectorName {
			t.Fatalf("wrong detector: %+v", f.Spec)
		}
	}
}

func TestScanHealthyIsSilent(t *testing.T) {
	if got := Scan([]byte(healthyDeployment)); len(got) != 0 {
		t.Fatalf("healthy manifest produced %d findings", len(got))
	}
}

func TestScanSkipsGarbage(t *testing.T) {
	// Malformed documents and unknown kinds never crash the scanner.
	docs := "kind: [unclosed\n---\nnot yaml at all: : :\n---\n" + healthyDeployment
	if got := Scan([]byte(docs)); len(got) != 0 {
		t.Fatalf("garbage produced %d findings", len(got))
	}
}

func TestDetectMissingImagePullSecrets(t *testing.T) {
	priv := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: f21
  namespace: demo
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
`
	if got := DetectMissingImagePullSecrets([]byte(priv)); len(got) != 1 {
		t.Fatalf("want 1 finding, got %d", len(got))
	} else if got[0].Spec.TargetRef.Name != "f21" || got[0].Spec.Category != v1alpha1.FindingCategorySecurity {
		t.Fatalf("bad finding: %+v", got[0].Spec)
	}
	// Public images and explicit secrets stay silent.
	for _, doc := range []string{
		`kind: Deployment
metadata: {name: x, namespace: demo}
spec: {template: {spec: {containers: [{name: app, image: nginx:1.25}]}}}`,
		`kind: Deployment
metadata: {name: x, namespace: demo}
spec: {template: {spec: {containers: [{name: app, image: user/repo:1}]}}}`,
		`kind: Deployment
metadata: {name: x, namespace: demo}
spec: {template: {spec: {containers: [{name: app, image: docker.io/library/nginx:1.25}]}}}`,
		`kind: Deployment
metadata: {name: x, namespace: demo}
spec: {template: {spec: {imagePullSecrets: [{name: creds}], containers: [{name: app, image: ghcr.io/example/demo-app:v1}]}}}`,
		healthyDeployment,
	} {
		if got := DetectMissingImagePullSecrets([]byte(doc)); len(got) != 0 {
			t.Fatalf("false positive on %q", doc[:60])
		}
	}
}
