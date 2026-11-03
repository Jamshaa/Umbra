package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/agent"
	"github.com/Jamshaa/Umbra/internal/brain"
	"github.com/Jamshaa/Umbra/internal/receipt"
)

func agentScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPublishDiagnosis(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(agentScheme(t)).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()
	in := publishInput{
		Fault: "F36", Kind: "Service", Namespace: "demo", Name: "svc",
		Model: "muse-test", GatePass: true, RootCause: "selector mismatch",
		Records: []agent.ToolRecord{
			{Name: "get_manifest", Args: `{"password":"hunter2-hunter2"}`, Result: "ok"},
		},
		Ts: 123,
	}
	note, err := publishDiagnosis(context.Background(), c, in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(note, "verified") && !strings.Contains(note, "Verified") {
		t.Fatalf("note = %q", note)
	}
	var got v1alpha1.Finding
	if err := c.Get(context.Background(), types.NamespacedName{Name: "f36-demo-svc"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.State != v1alpha1.FindingStateVerified {
		t.Fatalf("state = %q", got.Spec.State)
	}
	if len(got.Status.Timeline) != 1 || strings.Contains(got.Status.Timeline[0].Args, "hunter2") {
		t.Fatalf("timeline not sanitized: %+v", got.Status.Timeline)
	}
	if got.Status.Timeline[0].EvidenceHash == "" {
		t.Fatal("missing evidence hash")
	}
	// Idempotent: same records do not duplicate timeline entries.
	if _, err := publishDiagnosis(context.Background(), c, in); err != nil {
		t.Fatal(err)
	}
	var again v1alpha1.Finding
	if err := c.Get(context.Background(), types.NamespacedName{Name: "f36-demo-svc"}, &again); err != nil {
		t.Fatal(err)
	}
	if len(again.Status.Timeline) != 1 {
		t.Fatalf("timeline duplicated: %d entries", len(again.Status.Timeline))
	}
	// Terminal state: a new attempt cannot move it and writes nothing.
	bad := in
	bad.GatePass = false
	if _, err := publishDiagnosis(context.Background(), c, bad); err == nil {
		t.Fatal("terminal state move accepted")
	}
}

func TestPublishTruncates(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(agentScheme(t)).
		WithStatusSubresource(&v1alpha1.Finding{}).Build()
	big := strings.Repeat("x", maxTimelineField+100)
	_, err := publishDiagnosis(context.Background(), c, publishInput{
		Fault: "F07", Kind: "Deployment", Namespace: "demo", Name: "web",
		Records: []agent.ToolRecord{{Name: "get_logs", Args: big, Result: big}},
		Ts:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got v1alpha1.Finding
	if err := c.Get(context.Background(), types.NamespacedName{Name: "f07-demo-web"}, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Status.Timeline) != 1 || len(got.Status.Timeline[0].Result) > maxTimelineField+20 {
		t.Fatalf("timeline field not truncated: %d", len(got.Status.Timeline[0].Result))
	}
	var _ = metav1.ObjectMeta{}
}

func TestAppendReceiptChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "receipts.chain")
	key := []byte("test-key-please-ignore")
	in := receipt.Input{FindingID: "F01", Rule: "F01", RootCause: "x"}
	if err := appendReceipt(path, in, nil); err == nil {
		t.Fatal("empty key accepted")
	}
	if err := appendReceipt(path, in, key); err != nil {
		t.Fatal(err)
	}
	if err := appendReceipt(path, in, key); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var chain []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		chain = append(chain, line)
	}
	if err := receipt.VerifyChainSigned(chain, key); err != nil {
		t.Fatalf("generated chain does not verify: %v", err)
	}
	// One altered byte in a middle receipt must fail.
	raw := strings.Replace(string(data), "F01", "F02", 1)
	var forged []string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		forged = append(forged, line)
	}
	if err := receipt.VerifyChainSigned(forged, key); err == nil {
		t.Fatal("tampered chain accepted")
	}
}

func TestRunPingFakeServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","output":[{"type":"reasoning"},{"type":"function_call","call_id":"c1","name":"get_manifest","arguments":"{}"}],"usage":{"input_tokens":11,"output_tokens":7}}`))
	}))
	defer srv.Close()
	c := brain.New(brain.Config{APIKey: "test-key", BaseURL: srv.URL, Model: "muse-test"})
	out, err := runPing(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"function_call", "get_manifest", "input_tokens: 11", "output_tokens: 7"} {
		if !strings.Contains(out, want) {
			t.Fatalf("ping output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "test-key") {
		t.Fatal("key leaked in ping output")
	}
}
