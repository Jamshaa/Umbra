// Command umbra-agent runs one live diagnosis: model loop with read-only
// cluster tools, then the deterministic gates on the proposal.
//
//	umbra-agent diagnose --kind Deployment --name web --namespace demo \
//	    --fault F07 [--pod web-abc] [--sandbox] [--kubeconfig PATH]
//
// Without --sandbox nothing is written anywhere: gates run in memory
// (dry-run skipped openly) and the proposal is printed, never applied.
// With --sandbox the pipeline applies to the umbra-sandbox namespace only.
// MODEL_API_KEY is required (see .env.example); without it the command
// fails fast and says so.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	sigsyaml "sigs.k8s.io/yaml"

	"github.com/Jamshaa/Umbra/internal/agent"
	"github.com/Jamshaa/Umbra/internal/brain"
	"github.com/Jamshaa/Umbra/internal/invariants"
	"github.com/Jamshaa/Umbra/internal/livekube"
	"github.com/Jamshaa/Umbra/internal/verify"
)

// nullDry skips the server dry-run openly (printed in the result).
type nullDry struct{}

func (nullDry) DryRun(_ context.Context, _ []byte) error { return nil }

// nullBox skips the sandbox apply openly (printed in the result).
type nullBox struct{}

func (nullBox) Apply(_ context.Context, _ []byte) (string, error) { return "", nil }

// result is the machine-readable outcome, one JSON object on stdout.
type result struct {
	Finding   string `json:"finding"`
	Refused   bool   `json:"refused,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Proposal  any    `json:"proposal,omitempty"`
	GateStep  string `json:"gate_step,omitempty"`
	GatePass  bool   `json:"gate_pass,omitempty"`
	GateWhy   string `json:"gate_why,omitempty"`
	Sandbox   bool   `json:"sandbox"`
	Tokens    int    `json:"tokens"`
	Model     string `json:"model"`
	LatencyMS int64  `json:"latency_ms"`
}

func fail(msg string, args ...any) {
	fmt.Fprintln(os.Stderr, "umbra-agent: "+fmt.Sprintf(msg, args...))
	os.Exit(1)
}

// podStatusOf builds a verifier PodStatus from a live pod object.
func podStatusOf(pod map[string]any) verify.PodStatus {
	return livekube.PodStatusOf(pod)
}

// templateLabels extracts pod-template labels for pod discovery.
func templateLabels(obj map[string]any) map[string]string {
	return livekube.TemplateLabels(obj)
}

func main() {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	kind := fs.String("kind", "", "object kind (required)")
	name := fs.String("name", "", "object name (required)")
	ns := fs.String("namespace", "", "object namespace (required)")
	fault := fs.String("fault", "", "rule id for the gates, e.g. F07 (required)")
	pod := fs.String("pod", "", "pod name for the verifier (default: first pod matching template labels)")
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path (default: standard loading rules)")
	sandbox := fs.Bool("sandbox", false, "apply to umbra-sandbox and dry-run (default: read-only, gates in memory)")
	fs.SetOutput(os.Stderr)
	_ = fs.Parse(os.Args[1:])
	if *kind == "" || *name == "" || *ns == "" || *fault == "" {
		fs.Usage()
		os.Exit(2)
	}

	cfg, err := brain.FromEnv()
	if err != nil {
		fail("%v", err)
	}
	lk, err := livekube.New(*kubeconfig)
	if err != nil {
		fail("%v", err)
	}
	ctx := context.Background()

	obj, err := lk.Get(ctx, *kind, *name, *ns)
	if err != nil {
		fail("%v", err)
	}
	// Patch math and the prompt run on the cleaned object (see livekube).
	base, err := sigsyaml.Marshal(livekube.CleanForPatch(obj))
	if err != nil {
		fail("marshal manifest: %v", err)
	}

	var tools []brain.Tool
	for _, td := range agent.Tools() {
		tools = append(tools, brain.Tool{Name: td.Name, Description: td.Description, Parameters: json.RawMessage(td.Parameters)})
	}
	r := &agent.Runner{Client: brain.New(cfg), Kube: lk}
	userContent := fmt.Sprintf("Diagnose %s %s/%s (rule %s). Find the root cause with read-only tools, then propose_patch. Smallest fix; needs_human when unsure.", *kind, *ns, *name, *fault)

	start := time.Now()
	out, err := r.Run(ctx, "", userContent, tools)
	elapsed := time.Since(start)
	if err != nil {
		fail("model loop: %v", err)
	}
	res := result{Finding: *fault, Tokens: out.Tokens, Model: cfg.Model, LatencyMS: elapsed.Milliseconds(), Sandbox: *sandbox}
	if out.Refused || out.Proposal == nil {
		res.Refused = true
		res.Reason = out.Reason
		printResult(res, 2)
		return
	}
	res.Proposal = map[string]any{
		"root_cause": out.Proposal.RootCause, "ops": len(out.Proposal.Ops),
		"confidence": out.Proposal.Confidence, "needs_human": out.Proposal.NeedsHuman,
	}

	ops := make([]invariants.Op, 0, len(out.Proposal.Ops))
	for _, o := range out.Proposal.Ops {
		ops = append(ops, invariants.Op{Op: o.Op, Path: o.Path, Value: o.Value, HasV: o.HasV})
	}
	var dry invariants.DryRunner = nullDry{}
	var box invariants.Sandbox = nullBox{}
	if *sandbox {
		dry = livekube.DryRunner{Dyn: lk.Dyn}
		box = livekube.Sandbox{Dyn: lk.Dyn}
	}
	podName := *pod
	if podName == "" {
		if sel := templateLabels(obj); len(sel) > 0 {
			if pods, err := lk.List(ctx, "Pod", *ns, sel); err == nil && len(pods) > 0 {
				names := []string{}
				for _, p := range pods {
					m, _ := p["metadata"].(map[string]any)
					if n, _ := m["name"].(string); n != "" {
						names = append(names, n)
					}
				}
				sort.Strings(names)
				if len(names) > 0 {
					podName = names[0]
				}
			}
		}
	}
	var pst verify.PodStatus
	if podName != "" {
		if p, err := lk.Get(ctx, "Pod", podName, *ns); err == nil {
			pst = podStatusOf(p)
		}
	}
	v := invariants.ValidateOps(ctx, dry, box, invariants.Inputs{RuleID: *fault, Before: base, Pod: pst}, ops)
	res.GateStep = v.Step
	res.GatePass = v.Passed
	res.GateWhy = v.Reason
	code := 0
	if !v.Passed {
		code = 2
	}
	printResult(res, code)
}

func printResult(res result, code int) {
	raw, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(raw))
	os.Exit(code)
}
