package fixes

import (
	"strings"
	"testing"

	"github.com/Jamshaa/Umbra/internal/verify"
)

const bareCronManifest = `apiVersion: batch/v1
kind: CronJob
metadata:
  name: bare
  namespace: demo
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

func TestFixCronDeadline(t *testing.T) {
	out, ok := Fix("CronDeadline", []byte(bareCronManifest))
	if !ok {
		t.Fatal("CronDeadline should apply")
	}
	if !strings.Contains(string(out), "startingDeadlineSeconds: 60") {
		t.Fatalf("deadline missing:\n%s", out)
	}
	if r := verify.VerifyCronDeadline(out, verify.PodStatus{}); !r.Pass {
		t.Fatalf("patched doc fails gate: %s", r.Reason)
	}
}

func TestFixCronOverlapAndTZ(t *testing.T) {
	out, ok := Fix("CronOverlap", []byte(bareCronManifest))
	if !ok || !strings.Contains(string(out), "concurrencyPolicy: Forbid") {
		t.Fatalf("overlap not fixed: %s", out)
	}
	if r := verify.VerifyCronOverlap(out, verify.PodStatus{}); !r.Pass {
		t.Fatalf("gate: %s", r.Reason)
	}
	out, ok = Fix("CronTimezone", []byte(bareCronManifest))
	if !ok || !strings.Contains(string(out), "timeZone: UTC") {
		t.Fatalf("tz not fixed: %s", out)
	}
}

func TestFixCronSuspended(t *testing.T) {
	m := strings.Replace(bareCronManifest, `schedule: "*/5 * * * *"`,
		"schedule: \"*/5 * * * *\"\n  suspend: true", 1)
	out, ok := Fix("CronSuspended", []byte(m))
	if !ok || !strings.Contains(string(out), "suspend: false") {
		t.Fatalf("suspend not flipped: %s", out)
	}
	// Suspended without deadline still fails the suspend gate (burst risk).
	if r := verify.VerifyCronSuspended(out, verify.PodStatus{}); r.Pass {
		t.Fatalf("should fail without deadline: %+v", r)
	}
}

func TestFixCronBounds(t *testing.T) {
	out, ok := Fix("CronBounds", []byte(bareCronManifest))
	if !ok {
		t.Fatal("CronBounds should apply")
	}
	for _, want := range []string{"backoffLimit: 6", "activeDeadlineSeconds: 600", "ttlSecondsAfterFinished: 100"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	if r := verify.VerifyCronBounds(out, verify.PodStatus{}); !r.Pass {
		t.Fatalf("patched doc fails gate: %s", r.Reason)
	}
	// Idempotent: a bounded doc is a no-op.
	if _, ok := Fix("CronBounds", out); ok {
		t.Fatal("bounded doc must not be patched again")
	}
}

func TestFixCronUnknown(t *testing.T) {
	if _, ok := Fix("ServiceEndpoints", []byte(bareCronManifest)); ok {
		t.Fatal("selector fix needs cluster truth: must be model-only")
	}
}
