package report

import (
	"strings"
	"testing"
)

func TestBuildGolden(t *testing.T) {
	in := []Result{
		{Policy: "api-standard", Rule: "resources-limits", Outcome: Fail, Message: "no limits set", Resource: "Deployment/demo/web"},
		{Policy: "api-standard", Rule: "image-tag", Outcome: Pass, Message: "tag pinned", Resource: "Deployment/demo/web"},
	}
	want := `apiVersion: wgpolicyk8s.io/v1alpha2
kind: PolicyReport
metadata:
  name: umbra-demo-api-standard
  labels:
    umbra.io/managed-by: report
scope:
  namespace: demo
results:
- policy: api-standard
  rule: resources-limits
  result: fail
  message: no limits set
  resource: Deployment/demo/web
- policy: api-standard
  rule: image-tag
  result: pass
  message: tag pinned
  resource: Deployment/demo/web
summary:
  pass: 1
  fail: 1
  warn: 0
  error: 0
  skip: 0
`
	if got := string(Build("demo", "api-standard", in)); got != want {
		t.Fatalf("golden mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestNameDeterministic(t *testing.T) {
	in := []Result{{Policy: "p", Rule: "r", Outcome: Skip, Message: "m", Resource: "R"}}
	a, b := string(Build("demo", "api-standard", in)), string(Build("demo", "api-standard", in))
	if a != b {
		t.Fatal("same input gave different bytes")
	}
	other := string(Build("other", "api-standard", in))
	if !strings.Contains(other, "name: umbra-other-api-standard") {
		t.Fatalf("name not derived from scope:\n%s", other)
	}
	if strings.Contains(other, "umbra-demo-api-standard") {
		t.Fatalf("name collision across scopes:\n%s", other)
	}
	weird := string(Build("De Mo!", "API@Std", nil))
	if !strings.Contains(weird, "name: umbra-de-mo--api-std") {
		t.Fatalf("name not sanitized:\n%s", weird)
	}
	if !strings.Contains(weird, "results:\n  []\n") {
		t.Fatalf("empty results not rendered:\n%s", weird)
	}
}

func TestSummaryCounts(t *testing.T) {
	in := []Result{
		{Outcome: Pass}, {Outcome: Pass}, {Outcome: Fail},
		{Outcome: Warn}, {Outcome: Error}, {Outcome: Skip},
		{Outcome: "bogus"},
	}
	out := string(Build("s", "t", in))
	for _, want := range []string{"pass: 2", "fail: 1", "warn: 1", "error: 1", "skip: 1", "result: bogus"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
}

func TestMessageQuoting(t *testing.T) {
	in := []Result{{Policy: "p", Rule: "r", Outcome: Fail, Message: `saw "x: y" # leak`, Resource: "R"}}
	out := string(Build("s", "t", in))
	if !strings.Contains(out, `message: "saw \"x: y\" # leak"`) {
		t.Fatalf("message not quoted:\n%s", out)
	}
}
