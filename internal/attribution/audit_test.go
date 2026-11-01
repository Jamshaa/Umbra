package attribution

import (
	"strings"
	"testing"
	"time"
)

// Realistic kind apiserver line: human patches a Deployment.
const auditPatch = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","level":"Metadata","auditID":"7c3b1e9a-9c9a-4f1a-9b1a-9c9a4f1a9b1a","stage":"ResponseComplete","requestURI":"/apis/apps/v1/namespaces/demo/deployments/web-frontend","verb":"patch","user":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"sourceIPs":["127.0.0.1"],"userAgent":"kubectl/v1.37.1 (linux/amd64) kubernetes/1234567","objectRef":{"resource":"deployments","namespace":"demo","name":"web-frontend","apiVersion":"apps/v1","apiGroup":"apps"},"responseStatus":{"metadata":{},"code":200},"requestReceivedTimestamp":"2026-10-30T19:00:00.123456789Z","stageTimestamp":"2026-10-30T19:00:00.234567890Z","annotations":{"authorization.k8s.io/decision":"allow"}}`

// Controller writing status: system account, must never read as a person.
const auditController = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","level":"Metadata","stage":"ResponseComplete","verb":"update","user":{"username":"system:serviceaccount:umbra-system:umbra-manager","groups":["system:serviceaccounts","system:authenticated"]},"userAgent":"manager/v0.0.0 (linux/amd64) kubernetes/$Format","objectRef":{"resource":"remediationpolicies","name":"demo-require-approval","apiVersion":"umbra.io/v1alpha1"},"responseStatus":{"metadata":{},"code":200},"requestReceivedTimestamp":"2026-10-30T19:05:00Z","stageTimestamp":"2026-10-30T19:05:00Z"}`

func TestParseAuditLine(t *testing.T) {
	e, err := ParseAuditLine([]byte(auditPatch))
	if err != nil {
		t.Fatal(err)
	}
	if e.User != "kubernetes-admin" {
		t.Fatalf("user = %q", e.User)
	}
	if len(e.Groups) != 2 || e.Groups[0] != "system:masters" {
		t.Fatalf("groups = %v", e.Groups)
	}
	if !strings.HasPrefix(e.UserAgent, "kubectl/v1.37") {
		t.Fatalf("userAgent = %q", e.UserAgent)
	}
	if e.Verb != "patch" || e.Resource != "deployments" || e.Namespace != "demo" || e.Name != "web-frontend" || e.APIGroup != "apps" {
		t.Fatalf("what/where wrong: %+v", e)
	}
	if e.Stage != "ResponseComplete" {
		t.Fatalf("stage = %q", e.Stage)
	}
	want, _ := time.Parse(time.RFC3339Nano, "2026-10-30T19:00:00.123456789Z")
	if !e.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want requestReceived, not stage: %v", e.Timestamp, want)
	}
}

func TestParseAuditLineFallbacks(t *testing.T) {
	// No requestReceivedTimestamp: stageTimestamp is used.
	e, err := ParseAuditLine([]byte(`{"verb":"get","user":{"username":"alice"},"stageTimestamp":"2026-10-30T19:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	if e.Timestamp.IsZero() || e.User != "alice" {
		t.Fatalf("fallback broken: %+v", e)
	}
	// Well-formed JSON with unknown shape: zero values, no error.
	e, err = ParseAuditLine([]byte(`{"kind":"Event","unrelated":[1,2,3]}`))
	if err != nil {
		t.Fatal(err)
	}
	if e.User != "" || e.Verb != "" || !e.Timestamp.IsZero() {
		t.Fatalf("expected zero value, got %+v", e)
	}
	// Malformed JSON: hard error, never a half-identity.
	for _, bad := range []string{"", "{", `{"verb":`, "not json at all"} {
		if _, err := ParseAuditLine([]byte(bad)); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestPresentImpersonation(t *testing.T) {
	// kubectl --as: both identities recorded, effective actor presented labeled.
	imp, err := ParseAuditLine([]byte(`{"verb":"patch","user":{"username":"admin"},"impersonatedUser":{"username":"alice@example.com"}}`))
	if err != nil {
		t.Fatal(err)
	}
	id, q := imp.Present()
	if id != "alice@example.com" || q != "impersonated, requested by admin" {
		t.Fatalf("got (%q, %q)", id, q)
	}
	// Normal request: direct, no label games.
	direct, err := ParseAuditLine([]byte(`{"verb":"get","user":{"username":"bob"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if id, q := direct.Present(); id != "bob" || q != "direct" {
		t.Fatalf("got (%q, %q)", id, q)
	}
}

func TestResolveConfidence(t *testing.T) {
	person, err := ParseAuditLine([]byte(auditPatch))
	if err != nil {
		t.Fatal(err)
	}
	if a := Resolve(&person, []string{"kubectl", "manager"}); a.Level != ConfidencePerson || a.Identity != "kubernetes-admin" {
		t.Fatalf("audit person must win: %+v", a)
	}
	ctrl, err := ParseAuditLine([]byte(auditController))
	if err != nil {
		t.Fatal(err)
	}
	if a := Resolve(&ctrl, nil); a.Level != ConfidenceClient || a.Identity != "system:serviceaccount:umbra-system:umbra-manager" {
		t.Fatalf("system account is a client, not a person: %+v", a)
	}
	// No audit: managedFields manager names the client; deterministic pick.
	if a := Resolve(nil, []string{"manager", "kubectl", "manager"}); a.Level != ConfidenceClient || a.Identity != "kubectl" {
		t.Fatalf("want first sorted manager: %+v", a)
	}
	if a := Resolve(nil, nil); a.Level != ConfidenceUnknown || a.Identity != "" {
		t.Fatalf("want unknown: %+v", a)
	}
	// Empty audit struct behaves like no audit.
	if a := Resolve(&AuditEvent{}, []string{"argo"}); a.Level != ConfidenceClient || a.Identity != "argo" {
		t.Fatalf("want managedFields fallback: %+v", a)
	}
}
