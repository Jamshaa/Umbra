package invariants

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Jamshaa/Umbra/internal/fixes"
	"github.com/Jamshaa/Umbra/internal/verify"
)

type fakeDry struct {
	err   error
	calls int
}

func (f *fakeDry) DryRun(_ context.Context, _ []byte) error {
	f.calls++
	return f.err
}

type fakeBox struct {
	id     string
	err    error
	apples int
	docs   [][]byte
}

func (f *fakeBox) Apply(_ context.Context, doc []byte) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.apples++
	f.docs = append(f.docs, doc)
	if f.id == "" {
		return "snap-test-1", nil
	}
	return f.id, nil
}

var podOK = verify.PodStatus{Phase: "Running", Ready: true}
var podDown = verify.PodStatus{Phase: "Pending", Ready: false}

func fullPass(t *testing.T) (DryRunner, *fakeBox) {
	t.Helper()
	return &fakeDry{}, &fakeBox{}
}

func f01Ops() []Op {
	return []Op{{Op: "add", Path: "/spec/template/spec/containers/0/resources", Value: map[string]any{
		"requests": map[string]any{"cpu": "100m", "memory": "64Mi"},
		"limits":   map[string]any{"cpu": "500m", "memory": "512Mi"},
	}, HasV: true}}
}

const f01NoResources = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: umbra-sandbox
spec:
  template:
    spec:
      containers:
        - name: app
          image: example/web:1
          ports:
            - containerPort: 8080
`

func TestPipelineHappyPath(t *testing.T) {
	dry, box := fullPass(t)
	v := ValidateOps(context.Background(), dry, box, Inputs{
		RuleID: "F01", Before: []byte(f01NoResources), Pod: podOK,
	}, f01Ops())
	if !v.Passed || v.Step != "ok" || v.Snapshot != "snap-test-1" {
		t.Fatalf("happy path failed: %+v", v)
	}
	if len(box.docs) != 1 {
		t.Fatalf("sandbox must receive exactly the candidate, got %d applies", box.apples)
	}
}

func TestPipelineDeterministicSamePath(t *testing.T) {
	// A deterministic fix output goes through the exact same gates.
	fixed, ok := fixes.Fix("F01", []byte(f01NoResources))
	if !ok {
		t.Fatal("deterministic fix refused")
	}
	dry, box := fullPass(t)
	v := Validate(context.Background(), dry, box, Inputs{
		RuleID: "F01", Before: []byte(f01NoResources), After: fixed, Pod: podOK,
	})
	if !v.Passed {
		t.Fatalf("deterministic output rejected: %+v", v)
	}
}

func TestPipelineStopsAtEachGate(t *testing.T) {
	// The RBAC invariant needs a rule whose allowlist covers RBAC
	// paths (no shipped rule does: RBAC fixes have no deterministic
	// verifier, so they are refused earlier by design).
	rules["T-RBAC"] = Rule{Allow: []string{"/rules"}}
	defer delete(rules, "T-RBAC")
	goodOps := f01Ops()
	evilOps := []Op{{Op: "add", Path: "/spec/template/spec/containers/0/imagePullPolicy", Value: "Always", HasV: true}}
	baseDoc := `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web","namespace":"umbra-sandbox"},"spec":{"template":{"spec":{"containers":[{"name":"app","image":"example/web:1","ports":[{"containerPort":8080}],"resources":{"requests":{"cpu":"100m","memory":"64Mi"},"limits":{"cpu":"500m"}}}]}}}}`
	bigMem := "M" + strings.Repeat("x", MaxDocBytes)
	bigDoc := strings.Replace(baseDoc,
		`"limits":{"cpu":"500m"}}`,
		`"limits":{"cpu":"500m","memory":"`+bigMem+`"}}`, 1)
	// A secret-looking value under an allowed path: allowlist passes,
	// the secrets invariant must stop it.
	secretBase := strings.Replace(baseDoc, `"cpu":"500m"}}`,
		`"cpu":"500m","memory":"AKIAIOSFODNN7EXAMPLE"}}`, 1)

	rbacBefore := `{"kind":"Role","metadata":{"name":"r","namespace":"umbra-sandbox"},"rules":[{"apiGroups":[""],"resources":["pods"],"verbs":["get"]}]}`
	rbacAfter := `{"kind":"Role","metadata":{"name":"r","namespace":"umbra-sandbox"},"rules":[{"apiGroups":[""],"resources":["pods"],"verbs":["get","list"]}]}`

	cases := []struct {
		name          string
		rule          string
		before, after string
		ops           []Op
		usage         map[string]ResourceUsage
		dry           DryRunner
		box           *fakeBox
		pod           verify.PodStatus
		want          string
	}{
		{"unknown rule", "F99", f01NoResources, "", goodOps, nil, &fakeDry{}, &fakeBox{}, podOK, "allowlist"},
		{"off-rule field", "F01", f01NoResources, "", evilOps, nil, &fakeDry{}, &fakeBox{}, podOK, "allowlist"},
		{"rbac widened", "T-RBAC", rbacBefore, rbacAfter, nil, nil, &fakeDry{}, &fakeBox{}, podOK, "invariants"},
		{"too big", "F01", f01NoResources, bigDoc, nil, nil, &fakeDry{}, &fakeBox{}, podOK, "invariants"},
		{"secret", "F01", f01NoResources, secretBase, nil, nil, &fakeDry{}, &fakeBox{}, podOK, "invariants"},
		{"below usage", "F02", f01NoResources, "", []Op{{Op: "add", Path: "/spec/template/spec/containers/0/resources", Value: map[string]any{"limits": map[string]any{"memory": "8Mi"}}, HasV: true}}, map[string]ResourceUsage{"app": {Memory: "64Mi"}}, &fakeDry{}, &fakeBox{}, podOK, "invariants"},
		{"dryrun refused", "F01", f01NoResources, "", goodOps, nil, &fakeDry{err: errors.New("quota")}, &fakeBox{}, podOK, "dryrun"},
		{"sandbox fails", "F01", f01NoResources, "", goodOps, nil, &fakeDry{}, &fakeBox{err: errors.New("boom")}, podOK, "sandbox"},
		{"verifier red", "F01", f01NoResources, "", goodOps, nil, &fakeDry{}, &fakeBox{}, podDown, "verify"},
	}
	for _, tc := range cases {
		var v Verdict
		if tc.ops != nil {
			v = ValidateOps(context.Background(), tc.dry, tc.box, Inputs{RuleID: tc.rule, Before: []byte(tc.before), Pod: tc.pod, Usage: tc.usage}, tc.ops)
		} else {
			v = Validate(context.Background(), tc.dry, tc.box, Inputs{RuleID: tc.rule, Before: []byte(tc.before), After: []byte(tc.after), Pod: tc.pod})
		}
		if v.Passed || v.Step != tc.want {
			t.Fatalf("%s: got %+v, want stop at %s", tc.name, v, tc.want)
		}
	}
}

func TestPipelineTimeout(t *testing.T) {
	// Swap in a stuck verifier, restore afterwards (no parallel tests here).
	orig, ok := verifiers["F01"]
	if !ok {
		t.Fatal("F01 verifier missing")
	}
	unblock := make(chan struct{})
	defer func() { verifiers["F01"] = orig; close(unblock) }()
	verifiers["F01"] = func(_ []byte, _ verify.PodStatus) verify.Result {
		<-unblock
		return verify.Result{Pass: false, Reason: "stuck"}
	}
	dry, box := fullPass(t)
	v := ValidateOps(context.Background(), dry, box, Inputs{
		RuleID: "F01", Before: []byte(f01NoResources), Pod: podOK, Timeout: time.Millisecond,
	}, f01Ops())
	if v.Passed || v.Step != "verify" || v.Reason != "verifier timed out" {
		t.Fatalf("stuck verifier must time out: %+v", v)
	}
	if v.Snapshot == "" {
		t.Fatal("timeout keeps the sandbox snapshot for rollback")
	}
	// A cancelled context must also abort fast, before any apply.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v = ValidateOps(ctx, &fakeDry{}, &fakeBox{}, Inputs{RuleID: "F01", Before: []byte(f01NoResources), Pod: podOK}, f01Ops())
	if v.Passed {
		t.Fatalf("cancelled ctx must fail: %+v", v)
	}
}

func TestPipelineNoDeterministicVerifier(t *testing.T) {
	// F13 has an allowlist entry? No: unknown rules are refused outright.
	dry, box := fullPass(t)
	v := ValidateOps(context.Background(), dry, box, Inputs{
		RuleID: "F99", Before: []byte(f01NoResources), Pod: podOK,
	}, f01Ops())
	if v.Passed || v.Step != "allowlist" {
		t.Fatalf("unknown rule must stop at allowlist: %+v", v)
	}
}
