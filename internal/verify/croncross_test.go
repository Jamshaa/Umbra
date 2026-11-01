package verify

import (
	"strings"
	"testing"
)

const healthyCronDoc = `
apiVersion: batch/v1
kind: CronJob
metadata: {name: nightly, namespace: demo}
spec:
  schedule: "0 2 * * *"
  timeZone: UTC
  concurrencyPolicy: Forbid
  startingDeadlineSeconds: 60
  successfulJobsHistoryLimit: 3
  jobTemplate:
    spec:
      backoffLimit: 6
      activeDeadlineSeconds: 600
      ttlSecondsAfterFinished: 100
      template: {spec: {containers: [{name: j, image: x:v1}]}}
`

const bareCronDoc = `
apiVersion: batch/v1
kind: CronJob
metadata: {name: bare, namespace: demo}
spec:
  schedule: "*/5 * * * *"
  concurrencyPolicy: Allow
  jobTemplate: {spec: {template: {spec: {containers: [{name: j, image: x:v1}]}}}}
`

func TestVerifyCronDeadline(t *testing.T) {
	if r := VerifyCronDeadline([]byte(healthyCronDoc), PodStatus{}); !r.Pass {
		t.Fatalf("healthy should pass: %s", r.Reason)
	}
	if r := VerifyCronDeadline([]byte(bareCronDoc), PodStatus{}); r.Pass {
		t.Fatalf("bare should fail: %+v", r)
	}
	short := strings.Replace(bareCronDoc, "concurrencyPolicy: Allow",
		"startingDeadlineSeconds: 5", 1)
	if r := VerifyCronDeadline([]byte(short), PodStatus{}); r.Pass || !strings.Contains(r.Reason, "5") {
		t.Fatalf("short deadline should fail naming it: %+v", r)
	}
}

func TestVerifyCronSuspendedOverlapBoundsTZ(t *testing.T) {
	susp := strings.Replace(bareCronDoc, "concurrencyPolicy: Allow", "suspend: true", 1)
	if r := VerifyCronSuspended([]byte(susp), PodStatus{}); r.Pass {
		t.Fatalf("suspended should fail: %+v", r)
	}
	if r := VerifyCronSuspended([]byte(healthyCronDoc), PodStatus{}); !r.Pass {
		t.Fatalf("healthy should pass: %s", r.Reason)
	}
	if r := VerifyCronOverlap([]byte(bareCronDoc), PodStatus{}); r.Pass {
		t.Fatalf("allow should fail: %+v", r)
	}
	if r := VerifyCronOverlap([]byte(healthyCronDoc), PodStatus{}); !r.Pass {
		t.Fatalf("forbid should pass: %s", r.Reason)
	}
	if r := VerifyCronBounds([]byte(bareCronDoc), PodStatus{}); r.Pass {
		t.Fatalf("bare bounds should fail: %+v", r)
	}
	if r := VerifyCronBounds([]byte(healthyCronDoc), PodStatus{}); !r.Pass {
		t.Fatalf("healthy bounds should pass: %s", r.Reason)
	}
	if r := VerifyCronTimezone([]byte(bareCronDoc), PodStatus{}); r.Pass {
		t.Fatalf("no tz should fail: %+v", r)
	}
	if r := VerifyCronTimezone([]byte(healthyCronDoc), PodStatus{}); !r.Pass {
		t.Fatalf("tz should pass: %s", r.Reason)
	}
}

const cronStatusDoc = `
apiVersion: batch/v1
kind: CronJob
metadata: {name: s, namespace: demo}
spec:
  schedule: "*/5 * * * *"
  startingDeadlineSeconds: 60
  jobTemplate: {spec: {template: {spec: {containers: [{name: j, image: x:v1}]}}}}
status:
  lastScheduleTime: "2026-11-01T12:00:00Z"
  lastSuccessfulTime: "2026-11-01T10:00:00Z"
`

func TestVerifyCronSilentFail(t *testing.T) {
	if r := VerifyCronSilentFail([]byte(cronStatusDoc), PodStatus{}); r.Pass {
		t.Fatalf("stale success should fail: %+v", r)
	}
	if r := VerifyCronSilentFail([]byte(healthyCronDoc), PodStatus{}); r.Pass {
		t.Fatalf("no status must fail closed: %+v", r)
	}
}

const svcBundle = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: demo}
spec:
  template:
    metadata: {labels: {app: web}}
    spec: {containers: [{name: app, image: x:v1}]}
---
apiVersion: v1
kind: Service
metadata: {name: s, namespace: demo}
spec: {selector: {app: nope}}
`

const svcBundleFixed = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: demo}
spec:
  template:
    metadata: {labels: {app: web}}
    spec: {containers: [{name: app, image: x:v1, resources: {requests: {cpu: "10m"}}}]}
---
apiVersion: v1
kind: Service
metadata: {name: s, namespace: demo}
spec: {selector: {app: web}}
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: web, namespace: demo}
spec: {scaleTargetRef: {kind: Deployment, name: web}}
`

func TestVerifyServiceAndHPA(t *testing.T) {
	if r := VerifyServiceEndpoints([]byte(svcBundle), PodStatus{}); r.Pass {
		t.Fatalf("dangling selector should fail: %+v", r)
	}
	if r := VerifyServiceEndpoints([]byte(svcBundleFixed), PodStatus{}); !r.Pass {
		t.Fatalf("fixed bundle should pass: %s", r.Reason)
	}
	if r := VerifyHPARequests([]byte(svcBundle), PodStatus{}); r.Pass {
		t.Fatalf("no HPA must fail closed: %+v", r)
	}
	if r := VerifyHPARequests([]byte(svcBundleFixed), PodStatus{}); !r.Pass {
		t.Fatalf("fixed HPA should pass: %s", r.Reason)
	}
}

func TestVerifyProbesDistinct(t *testing.T) {
	same := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: p, namespace: demo}
spec:
  template:
    spec:
      containers:
        - name: app
          image: x:v1
          resources:
            requests: {cpu: "10m", memory: "64Mi"}
            limits: {cpu: "100m", memory: "128Mi"}
          livenessProbe: {tcpSocket: {port: 8080}}
          readinessProbe: {tcpSocket: {port: 8080}}
`
	if r := VerifyProbesDistinct([]byte(same), PodStatus{Phase: "Running", Ready: true}); r.Pass {
		t.Fatalf("identical probes should fail: %+v", r)
	}
	if r := VerifyProbesDistinct([]byte(healthyManifest), PodStatus{Phase: "Running", Ready: true}); !r.Pass {
		t.Fatalf("distinct probes should pass: %s", r.Reason)
	}
	delayOnly := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: p, namespace: demo}
spec:
  template:
    spec:
      containers:
        - name: app
          image: x:v1
          resources:
            requests: {cpu: "10m", memory: "64Mi"}
            limits: {cpu: "100m", memory: "128Mi"}
          livenessProbe: {tcpSocket: {port: 8080}}
          readinessProbe: {tcpSocket: {port: 8080}, initialDelaySeconds: 30}
`
	if r := VerifyProbesDistinct([]byte(delayOnly), PodStatus{Phase: "Running", Ready: true}); r.Pass {
		t.Fatalf("delay-only difference must still fail: %+v", r)
	}
}

func TestVerifyRBACLeastPrivilege(t *testing.T) {
	bad := `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: {name: bad}
rules: [{apiGroups: ["*"], resources: ["*"], verbs: ["*"]}]
`
	if r := VerifyRBACLeastPrivilege([]byte(bad), PodStatus{}); r.Pass {
		t.Fatalf("wildcard should fail: %+v", r)
	}
	good := `
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: {name: ok, namespace: demo}
rules: [{apiGroups: [""], resources: ["configmaps"], verbs: ["get", "list"]}]
`
	if r := VerifyRBACLeastPrivilege([]byte(good), PodStatus{}); !r.Pass {
		t.Fatalf("least-privilege should pass: %s", r.Reason)
	}
}

func TestVerifyHPAInvertedBounds(t *testing.T) {
	bad := `
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: h, namespace: demo}
spec: {scaleTargetRef: {kind: Deployment, name: web}, minReplicas: 8, maxReplicas: 2}
`
	if r := VerifyHPAInvertedBounds([]byte(bad), PodStatus{}); r.Pass {
		t.Fatalf("inverted bounds should fail: %+v", r)
	}
	good := `
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: h, namespace: demo}
spec: {scaleTargetRef: {kind: Deployment, name: web}, minReplicas: 1, maxReplicas: 4}
`
	if r := VerifyHPAInvertedBounds([]byte(good), PodStatus{}); !r.Pass {
		t.Fatalf("ordered bounds should pass: %s", r.Reason)
	}
}
