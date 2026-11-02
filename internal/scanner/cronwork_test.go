package scanner

import (
	"testing"
	"time"

	"github.com/Jamshaa/Umbra/internal/attribution"
)

const healthyCron = `
apiVersion: batch/v1
kind: CronJob
metadata: {name: nightly, namespace: demo}
spec:
  schedule: "0 2 * * *"
  timeZone: UTC
  concurrencyPolicy: Forbid
  startingDeadlineSeconds: 60
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 1
  jobTemplate:
    spec:
      backoffLimit: 6
      activeDeadlineSeconds: 600
      ttlSecondsAfterFinished: 100
      template:
        spec:
          containers:
            - name: job
              image: ghcr.io/example/demo-app:v1
              resources:
                requests: {cpu: "10m", memory: "64Mi"}
                limits: {cpu: "100m", memory: "128Mi"}
`

const bareCron = `
apiVersion: batch/v1
kind: CronJob
metadata: {name: bare, namespace: demo}
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: job
              image: ghcr.io/example/demo-app:v1
`

func TestCronMissingDeadline(t *testing.T) {
	if got := DetectCronMissingDeadline([]byte(bareCron)); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got := DetectCronMissingDeadline([]byte(healthyCron)); len(got) != 0 {
		t.Fatalf("healthy flagged: %+v", got)
	}
}

func TestCronShortDeadline(t *testing.T) {
	short := bareCron + ""
	_ = short
	doc := `
apiVersion: batch/v1
kind: CronJob
metadata: {name: s, namespace: demo}
spec:
  schedule: "* * * * *"
  startingDeadlineSeconds: 5
  jobTemplate: {spec: {template: {spec: {containers: [{name: j, image: x:v1}]}}}}
`
	if got := DetectCronShortDeadline([]byte(doc)); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got := DetectCronShortDeadline([]byte(healthyCron)); len(got) != 0 {
		t.Fatalf("healthy flagged")
	}
}

func TestCronSuspendOverlapTZBounds(t *testing.T) {
	susp := `
apiVersion: batch/v1
kind: CronJob
metadata: {name: s, namespace: demo}
spec:
  schedule: "* * * * *"
  suspend: true
  jobTemplate: {spec: {template: {spec: {containers: [{name: j, image: x:v1}]}}}}
`
	if got := DetectCronSuspended([]byte(susp)); len(got) != 1 {
		t.Fatalf("suspended: want 1, got %d", len(got))
	}
	if got := DetectCronOverlap([]byte(bareCron)); len(got) != 1 {
		t.Fatalf("overlap: want 1, got %d", len(got))
	}
	if got := DetectCronOverlap([]byte(healthyCron)); len(got) != 0 {
		t.Fatalf("healthy overlap flagged")
	}
	if got := DetectCronMissingTimezone([]byte(bareCron)); len(got) != 1 {
		t.Fatalf("tz: want 1, got %d", len(got))
	}
	if got := DetectCronMissingJobBounds([]byte(bareCron)); len(got) != 1 {
		t.Fatalf("bounds: want 1, got %d", len(got))
	}
	if got := DetectCronMissingJobBounds([]byte(healthyCron)); len(got) != 0 {
		t.Fatalf("healthy bounds flagged")
	}
}

func TestCronGPUPlacement(t *testing.T) {
	doc := `
apiVersion: batch/v1
kind: CronJob
metadata: {name: g, namespace: demo}
spec:
  schedule: "* * * * *"
  startingDeadlineSeconds: 60
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: train
              image: x:v1
              resources: {limits: {nvidia.com/gpu: "1"}}
`
	if got := DetectCronGPUWithoutPlacement([]byte(doc)); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got := DetectCronGPUWithoutPlacement([]byte(healthyCron)); len(got) != 0 {
		t.Fatalf("healthy flagged")
	}
}

func TestNextCronRuns(t *testing.T) {
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Unix()
	runs := NextCronRuns("*/30 * * * *", "", from, 4)
	if len(runs) != 4 {
		t.Fatalf("want 4 runs, got %v", runs)
	}
	for i := 1; i < len(runs); i++ {
		if runs[i]-runs[i-1] != 1800 {
			t.Fatalf("want 30m spacing, got %v", runs)
		}
	}
	if got := NextCronRuns("nonsense", "", from, 4); got != nil {
		t.Fatalf("bad schedule must yield nil, got %v", got)
	}
}

func TestIdenticalProbes(t *testing.T) {
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
          livenessProbe: {tcpSocket: {port: 8080}}
          readinessProbe: {tcpSocket: {port: 8080}}
`
	if got := DetectIdenticalProbes([]byte(same)); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got := DetectIdenticalProbes([]byte(healthyDeployment)); len(got) != 0 {
		t.Fatalf("healthy flagged")
	}
}

func TestServiceNoEndpoints(t *testing.T) {
	docs := `
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
metadata: {name: lonely, namespace: demo}
spec: {selector: {app: missing}}
`
	if got := DetectServiceNoEndpoints([]byte(docs)); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	ok := `
apiVersion: v1
kind: Service
metadata: {name: s, namespace: demo}
spec: {selector: {app: web}}
`
	if got := DetectServiceNoEndpoints([]byte(docs + "\n---\n" + ok)); len(got) != 1 {
		t.Fatalf("matching service must not add findings, got %d", len(got))
	}
}

func TestHPAWithoutRequests(t *testing.T) {
	docs := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: demo}
spec:
  template: {spec: {containers: [{name: app, image: x:v1}]}}
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: web, namespace: demo}
spec: {scaleTargetRef: {kind: Deployment, name: web}}
`
	if got := DetectHPAWithoutRequests([]byte(docs)); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
}

func TestRBACWildcard(t *testing.T) {
	docs := `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: {name: bad}
rules: [{apiGroups: ["*"], resources: ["*"], verbs: ["*"]}]
`
	if got := DetectRBACWildcard([]byte(docs)); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	// Scalar (non-list) wildcards and RoleBinding->cluster-admin count too.
	scalar := `
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: {name: badscalar, namespace: demo}
rules: [{apiGroups: "", resources: secrets, verbs: "*"}]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata: {name: evilbind, namespace: demo}
subjects: [{kind: ServiceAccount, name: app}]
roleRef: {kind: ClusterRole, name: cluster-admin}
`
	if got := DetectRBACWildcard([]byte(scalar)); len(got) != 2 {
		t.Fatalf("want 2 (scalar secrets read, namespaced cluster-admin grant), got %d: %+v", len(got), got)
	}
}

func TestHPAInvertedBounds(t *testing.T) {
	bad := `
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: h, namespace: demo}
spec: {scaleTargetRef: {kind: Deployment, name: web}, minReplicas: 8, maxReplicas: 2}
`
	if got := DetectHPAInvertedBounds([]byte(bad)); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	ok := `
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: {name: h, namespace: demo}
spec: {scaleTargetRef: {kind: Deployment, name: web}, minReplicas: 1, maxReplicas: 4}
`
	if got := DetectHPAInvertedBounds([]byte(ok)); len(got) != 0 {
		t.Fatalf("ordered bounds must be silent, got %v", got)
	}
}

func TestMineTemplateAndImpact(t *testing.T) {
	stats, tpl := MineTemplate([]byte(healthyDeployment))
	if stats.Count != 1 {
		t.Fatalf("want count 1, got %+v", stats)
	}
	if tpl.Spec.Required.Resources == nil {
		t.Fatalf("mined template must require resources: %+v", tpl.Spec)
	}
	if got := ImpactScore(2, 1, 1, 1, 1, "Deployment"); got <= 10 || got > 100 {
		t.Fatalf("score out of range: %d", got)
	}
	if ImpactScore(0, 0, 0, 0, 0, "Deployment") != 10 {
		t.Fatalf("empty neighbourhood must score 10")
	}
	if ImpactScore(0, 0, 0, 0, 0, "ClusterRole") != 40 {
		t.Fatalf("cluster kinds must score higher")
	}
}

func TestCorrelateChanges(t *testing.T) {
	now := time.Now()
	fields := []attribution.FieldAttribution{
		{Field: "spec.replicas", Manager: "kubectl", UpdatedAt: now.Add(-30 * time.Minute)},
		{Field: "spec.old", Manager: "old", UpdatedAt: now.Add(-5 * time.Hour)},
	}
	lines := CorrelateChanges(fields, []string{"boom"}, now)
	if len(lines) == 0 {
		t.Fatal("want correlated lines")
	}
}
