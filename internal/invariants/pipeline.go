package invariants

import (
	"context"
	"time"

	"github.com/Jamshaa/Umbra/internal/verify"
)

// VerifyTimeout bounds the deterministic check: verification is fast
// CPU work, never a place to hang the loop.
const VerifyTimeout = 30 * time.Second

// verifyFn is a rule's deterministic gate: F01..F06 from internal/verify.
// Rules without a Go verifier are refused by the pipeline (fail closed);
// T5 covers them with diagnosis-only runs.
var verifiers = map[string]func(doc []byte, pod verify.PodStatus) verify.Result{
	"F01": verify.VerifyF01,
	"F02": verify.VerifyF02,
	"F03": verify.VerifyF03,
	"F04": verify.VerifyF04,
	"F05": verify.VerifyF05,
	"F06": verify.VerifyF06,
}

// DryRunner asks the API server to admit a document without persisting
// it. The real implementation lives with the harness (needs a cluster);
// tests use a fake.
type DryRunner interface {
	DryRun(ctx context.Context, doc []byte) error
}

// Sandbox applies to umbra-sandbox only and returns a snapshot id for
// rollback. The real implementation lives with the harness.
type Sandbox interface {
	Apply(ctx context.Context, doc []byte) (snapshotID string, err error)
}

// Verdict is the pipeline outcome. Passed with a Snapshot means every
// gate went green; otherwise Step names the first gate that failed.
type Verdict struct {
	Passed   bool
	Step     string
	Reason   string
	Snapshot string
	Skipped  []string
}

// Inputs carries everything the gates need. Usage maps container name
// to observed consumption; missing entries skip that check openly.
type Inputs struct {
	RuleID  string
	Before  []byte
	After   []byte
	Pod     verify.PodStatus
	Usage   map[string]ResourceUsage
	Timeout time.Duration
}

func (in Inputs) timeout() time.Duration {
	if in.Timeout > 0 {
		return in.Timeout
	}
	return VerifyTimeout
}

// Validate runs the fixed gate order: allowlist+invariants on the docs,
// dry-run, sandbox apply, verifier. First failure stops the pipeline.
func Validate(ctx context.Context, dry DryRunner, box Sandbox, in Inputs) Verdict {
	before, err := decodeOne(in.Before)
	if err != nil {
		return Verdict{Step: "parse", Reason: "before: " + err.Error()}
	}
	afterDoc, err := decodeOne(in.After)
	if err != nil {
		return Verdict{Step: "parse", Reason: "after: " + err.Error()}
	}
	if err := CheckAllowlist(in.RuleID, DiffPaths(before, afterDoc)); err != nil {
		return Verdict{Step: "allowlist", Reason: err.Error()}
	}
	if err := checkIdentity(before, afterDoc); err != nil {
		return Verdict{Step: "invariants", Reason: err.Error()}
	}
	if err := checkSize(in.After); err != nil {
		return Verdict{Step: "invariants", Reason: err.Error()}
	}
	if err := checkSecrets(in.After); err != nil {
		return Verdict{Step: "invariants", Reason: err.Error()}
	}
	if err := checkRBAC(before, afterDoc); err != nil {
		return Verdict{Step: "invariants", Reason: err.Error()}
	}
	skipped, err := checkResources(before, afterDoc, in.Usage)
	if err != nil {
		return Verdict{Step: "invariants", Reason: err.Error()}
	}
	if err := dry.DryRun(ctx, in.After); err != nil {
		return Verdict{Step: "dryrun", Reason: "dry-run refused: " + err.Error()}
	}
	snap, err := box.Apply(ctx, in.After)
	if err != nil {
		return Verdict{Step: "sandbox", Reason: "sandbox apply failed: " + err.Error()}
	}
	vfn, ok := verifiers[in.RuleID]
	if !ok {
		return Verdict{Step: "verify", Reason: "no deterministic verifier for rule " + in.RuleID + " (diagnose only)", Snapshot: snap, Skipped: skipped}
	}
	vctx, cancel := context.WithTimeout(ctx, in.timeout())
	defer cancel()
	done := make(chan verify.Result, 1)
	go func() { done <- vfn(in.After, in.Pod) }()
	select {
	case <-vctx.Done():
		return Verdict{Step: "verify", Reason: "verifier timed out", Snapshot: snap, Skipped: skipped}
	case res := <-done:
		if !res.Pass {
			return Verdict{Step: "verify", Reason: res.Reason, Snapshot: snap, Skipped: skipped}
		}
		return Verdict{Passed: true, Step: "ok", Reason: res.Reason, Snapshot: snap, Skipped: skipped}
	}
}

// ValidateOps is the agent path: apply ops to the base document, then
// run the exact same gates. Deterministic fixes skip straight to
// Validate with their finished YAML. One path, two doors.
func ValidateOps(ctx context.Context, dry DryRunner, box Sandbox, in Inputs, ops []Op) Verdict {
	after, err := ApplyOps(in.Before, ops)
	if err != nil {
		return Verdict{Step: "patch", Reason: err.Error()}
	}
	in.After = after
	return Validate(ctx, dry, box, in)
}
