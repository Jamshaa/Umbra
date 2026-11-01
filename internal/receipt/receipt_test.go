package receipt

import (
	"strings"
	"testing"
	"time"

	"github.com/Jamshaa/Umbra/internal/attribution"
)

var fixedTime = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func seedInput() Input {
	return Input{
		FindingID: "F01",
		Severity:  "High",
		Category:  "Resources",
		Kind:      "Deployment", Namespace: "demo", Name: "web",
		Rule:      "F01",
		Template:  "api-standard",
		RootCause: "no resources set",
		Ops: []Op{
			{Op: "add", Path: "/spec/template/spec/containers/0/resources", Value: map[string]any{"limits": map[string]any{"memory": "512Mi"}}},
		},
		Diff:       "--- live\n+++ fixed\n@@ resources added",
		RevertPlan: "original yaml",
		Ladder: []StepResult{
			{Step: "invariants", Pass: true, Detail: "ok", DurationMs: 3},
			{Step: "verify", Pass: true, Detail: "F01 green", DurationMs: 41},
		},
		Attr:     attribution.Attribution{Level: attribution.ConfidencePerson, Identity: "alice", When: fixedTime},
		Model:    ModelUse{Model: "muse-spark", ResponseIDs: []string{"resp_1"}, InputTokens: 100, OutputTokens: 50, PromptHash: Fingerprint("system\nfind F01"), Tools: []ToolCall{{Name: "get_manifest", ArgsHash: Fingerprint(`{"kind":"Deployment"}`)}}},
		Decision: Decision{State: "approved", Approver: "alice", At: fixedTime},
		Snapshot: "snap-1",
		PRURL:    "https://example.com/pr/1",
		PrevHash: "GENESIS",
	}
}

func TestBuildV2Deterministic(t *testing.T) {
	a, b := Build(seedInput()), Build(seedInput())
	if a.JSON != b.JSON || a.SHA256 != b.SHA256 {
		t.Fatal("same input gave different receipts")
	}
	// SHA covers the canonical body including the chain link.
	if a.SHA256 != Fingerprint(a.JSON) {
		t.Fatalf("SHA256 %q does not match body", a.SHA256)
	}
	for _, want := range []string{
		`"finding":"F01"`, `"severity":"High"`, `"category":"Resources"`,
		`"rule":"F01"`, `"template":"api-standard"`,
		`"rootCause":"no resources set"`,
		`"path":"/spec/template/spec/containers/0/resources"`,
		`"durationMs":41`, `"step":"verify"`,
		`"level":"person"`, `"identity":"alice"`, `"when":"2026-11-01T10:00:00Z"`,
		`"model":"muse-spark"`, `"responseIds":["resp_1"]`,
		`"inputTokens":100`, `"outputTokens":50`,
		`"state":"approved"`, `"approver":"alice"`,
		`"snapshot":"snap-1"`, `"prevHash":"GENESIS"`,
		`"kind":"Deployment"`, `"namespace":"demo"`, `"name":"web"`,
	} {
		if !strings.Contains(a.JSON, want) {
			t.Fatalf("receipt missing %s:\n%s", want, a.JSON)
		}
	}
}

func TestBuildV2NoModel(t *testing.T) {
	in := seedInput()
	in.Model = ModelUse{}
	in.Decision = Decision{State: "proposed"}
	r := Build(in)
	for _, want := range []string{`"model":""`, `"responseIds":[]`, `"tools":[]`, `"state":"proposed"`} {
		if !strings.Contains(r.JSON, want) {
			t.Fatalf("no-model receipt must be explicit, missing %s:\n%s", want, r.JSON)
		}
	}
	if strings.Contains(r.JSON, "approver") {
		t.Fatalf("undecided receipt must not name an approver:\n%s", r.JSON)
	}
}

func TestBuildV2AttributionLevels(t *testing.T) {
	for _, tc := range []struct {
		level attribution.Confidence
		want  string
	}{
		{attribution.ConfidencePerson, `"level":"person"`},
		{attribution.ConfidenceClient, `"level":"client"`},
		{attribution.ConfidenceUnknown, `"level":"unknown"`},
	} {
		in := seedInput()
		in.Attr = attribution.Attribution{Level: tc.level, Identity: "x"}
		if got := Build(in); !strings.Contains(got.JSON, tc.want) {
			t.Fatalf("level %v not rendered: %s", tc.level, got.JSON)
		}
	}
}

func TestChainDetectsTamper(t *testing.T) {
	r1 := Build(seedInput())
	in2 := seedInput()
	in2.PrevHash = r1.SHA256
	r2 := Build(in2)
	chain := AppendRegistro(nil, r1.JSON)
	chain = AppendRegistro(chain, r2.JSON)
	chain = AppendRegistro(chain, r2.JSON)
	if err := VerifyChain(chain); err != nil {
		t.Fatalf("intact chain rejected: %v", err)
	}
	if err := VerifyChain(nil); err != nil {
		t.Fatalf("empty chain rejected: %v", err)
	}
	// Altered payload in the middle: the next link stops matching.
	bad := append([]string{}, chain...)
	bad[1] = strings.Replace(bad[1], "verify", "VERIFY", 1)
	if err := VerifyChain(bad); err == nil {
		t.Fatal("tampered chain accepted")
	} else if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error should name line 2: %v", err)
	}
	// Swapped order breaks the link.
	swapped := []string{chain[1], chain[0]}
	if err := VerifyChain(swapped); err == nil {
		t.Fatal("reordered chain accepted")
	}
	// Dropped tail is a prefix: still valid (truncation is visible as
	// missing lines, not corruption).
	if err := VerifyChain(chain[:1]); err != nil {
		t.Fatalf("prefix chain rejected: %v", err)
	}
	// Non-JSON line.
	if err := VerifyChain([]string{"not json"}); err == nil {
		t.Fatal("garbage line accepted")
	}
}

func TestChainLinksPayloadHash(t *testing.T) {
	chain := AppendRegistro(nil, "first")
	if !strings.Contains(chain[0], `"prev":"GENESIS"`) {
		t.Fatalf("first link must reference GENESIS: %s", chain[0])
	}
	chain = AppendRegistro(chain, "second")
	want := `"prev":"` + Fingerprint(chain[0]) + `"`
	if !strings.Contains(chain[1], want) {
		t.Fatalf("second link must reference hash of first: %s", chain[1])
	}
	if len(chain) != 2 {
		t.Fatalf("expected 2 links, got %d", len(chain))
	}
}
