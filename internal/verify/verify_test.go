package verify

import (
	"strings"
	"testing"
)

var runningReady = PodStatus{Phase: "Running", Ready: true}

const healthyManifest = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: shop, namespace: demo}
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
          resources:
            requests: {cpu: "10m", memory: "64Mi"}
            limits: {cpu: "100m", memory: "128Mi"}
          livenessProbe: {tcpSocket: {port: 8080}}
          readinessProbe: {httpGet: {path: /readyz, port: 8080}}
`

const noLimitsManifest = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: f01, namespace: demo}
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
`

const tinyLimitManifest = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: f02, namespace: demo}
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
          readinessProbe: {httpGet: {path: /readyz, port: 8080}}
`

const hugeRequestsManifest = `
apiVersion: apps/v1
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

const noLivenessManifest = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: f05, namespace: demo}
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/demo-app:v1
          resources:
            requests: {cpu: "10m", memory: "64Mi"}
            limits: {cpu: "100m", memory: "128Mi"}
          readinessProbe: {tcpSocket: {port: 8080}}
`

func TestVerifyF01(t *testing.T) {
	if r := VerifyF01([]byte(healthyManifest), runningReady); !r.Pass {
		t.Fatalf("healthy should pass: %s", r.Reason)
	}
	r := VerifyF01([]byte(noLimitsManifest), runningReady)
	if r.Pass || r.Reason == "" {
		t.Fatalf("no-limits should fail with reason, got %+v", r)
	}
	r = VerifyF01([]byte(healthyManifest), PodStatus{Phase: "Pending"})
	if r.Pass {
		t.Fatalf("pending pod should fail: %+v", r)
	}
	if r := VerifyF01([]byte("kind: ["), runningReady); r.Pass || r.Reason == "" {
		t.Fatalf("garbage must fail with reason, got %+v", r)
	}
}

func TestVerifyF02(t *testing.T) {
	if r := VerifyF02([]byte(healthyManifest), runningReady); !r.Pass {
		t.Fatalf("healthy should pass: %s", r.Reason)
	}
	r := VerifyF02([]byte(tinyLimitManifest), runningReady)
	if r.Pass || !strings.Contains(r.Reason, "8Mi") {
		t.Fatalf("tiny limit should fail naming the value, got %+v", r)
	}
	oom := runningReady
	oom.LastTerminationReason = "OOMKilled"
	if r := VerifyF02([]byte(healthyManifest), oom); r.Pass {
		t.Fatalf("recent OOMKill should fail: %+v", r)
	}
}

func TestVerifyF03(t *testing.T) {
	if r := VerifyF03([]byte(healthyManifest), runningReady); !r.Pass {
		t.Fatalf("healthy should pass: %s", r.Reason)
	}
	if r := VerifyF03([]byte(hugeRequestsManifest), runningReady); r.Pass || r.Reason == "" {
		t.Fatalf("huge requests should fail with reason, got %+v", r)
	}
}

func TestVerifyF04(t *testing.T) {
	if r := VerifyF04([]byte(healthyManifest), runningReady); !r.Pass {
		t.Fatalf("healthy should pass: %s", r.Reason)
	}
	if r := VerifyF04([]byte(noLimitsManifest), runningReady); r.Pass {
		t.Fatalf("no-limits should fail: %+v", r)
	}
	looping := runningReady
	looping.Restarts = 7
	if r := VerifyF04([]byte(healthyManifest), looping); r.Pass || !strings.Contains(r.Reason, "7") {
		t.Fatalf("escalating restarts should fail naming the count, got %+v", r)
	}
}

func TestVerifyF05(t *testing.T) {
	if r := VerifyF05([]byte(healthyManifest), runningReady); !r.Pass {
		t.Fatalf("healthy should pass: %s", r.Reason)
	}
	if r := VerifyF05([]byte(noLivenessManifest), runningReady); r.Pass || r.Reason == "" {
		t.Fatalf("missing liveness should fail with reason, got %+v", r)
	}
}

func TestVerifyF06(t *testing.T) {
	if r := VerifyF06([]byte(healthyManifest), runningReady); !r.Pass {
		t.Fatalf("healthy should pass: %s", r.Reason)
	}
	notReady := runningReady
	notReady.Ready = false
	if r := VerifyF06([]byte(healthyManifest), notReady); r.Pass {
		t.Fatalf("not-ready pod should fail: %+v", r)
	}
}
