package receipt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Jamshaa/Umbra/internal/attribution"
)

// Op is one validated patch operation, as proposed (agent) or applied
// (deterministic fix expressed in the same shape).
type Op struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// StepResult is the outcome of one verification ladder step. DurationMs
// records wall time so slow steps are visible, not just pass/fail.
type StepResult struct {
	Step       string
	Pass       bool
	Detail     string
	DurationMs int64
}

// ToolCall is one model tool invocation: the name plus the sha256 of
// the exact arguments JSON that was sent. Arguments themselves are
// never stored: they may embed untrusted cluster output.
type ToolCall struct {
	Name     string `json:"name"`
	ArgsHash string `json:"argsHash"`
}

// ModelUse records what the model did. Zero value means no model was
// involved (deterministic-only run); the section is still emitted so
// "no model" is explicit, not ambiguous.
type ModelUse struct {
	Model        string     `json:"model"`
	ResponseIDs  []string   `json:"responseIds"`
	InputTokens  int        `json:"inputTokens"`
	OutputTokens int        `json:"outputTokens"`
	PromptHash   string     `json:"promptHash"`
	Tools        []ToolCall `json:"tools"`
}

// Decision is the human gate outcome. Empty Approver means undecided:
// proposed, not approved.
type Decision struct {
	State    string    `json:"state"`
	Approver string    `json:"approver,omitempty"`
	At       time.Time `json:"at,omitempty"`
}

// Receipt is the evidence packet: the canonical JSON document plus
// its SHA256. The same inputs always yield the same bytes.
type Receipt struct {
	JSON   string
	SHA256 string
}

// Input carries everything the spec requires in one receipt.
// Callers pass sanitized content; the receipt stores it verbatim so
// audits see exactly what was decided on.
type Input struct {
	FindingID  string
	Severity   string
	Category   string
	Kind       string
	Namespace  string
	Name       string
	Rule       string // e.g. "F01"
	Template   string // e.g. "api-standard"; "" when no template applies
	RootCause  string
	Ops        []Op
	Diff       string // unified diff text, for humans; "" when none
	RevertPlan string // pre-patch snapshot YAML
	Ladder     []StepResult
	Attr       attribution.Attribution
	Model      ModelUse
	Decision   Decision
	Snapshot   string // sandbox snapshot reference
	PRURL      string
	PrevHash   string // previous chain hash, or "GENESIS"
}

type attributionOut struct {
	Level    string `json:"level"`
	Identity string `json:"identity"`
	When     string `json:"when"`
}

type stepOut struct {
	Step       string `json:"step"`
	Pass       bool   `json:"pass"`
	Detail     string `json:"detail"`
	DurationMs int64  `json:"durationMs"`
}

type opOut struct {
	// Value is always present (null when absent): omitempty would
	// silently drop legitimate false/0/"" values and falsify history.
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type receiptBody struct {
	Finding     string         `json:"finding"`
	Severity    string         `json:"severity"`
	Category    string         `json:"category"`
	Target      targetOut      `json:"target"`
	Rule        string         `json:"rule"`
	Template    string         `json:"template,omitempty"`
	RootCause   string         `json:"rootCause"`
	Patch       []opOut        `json:"patch"`
	Diff        string         `json:"diff,omitempty"`
	RevertPlan  string         `json:"revertPlan"`
	Ladder      []stepOut      `json:"ladder"`
	Attribution attributionOut `json:"attribution"`
	Model       modelOut       `json:"model"`
	Decision    decisionOut    `json:"decision"`
	Snapshot    string         `json:"snapshot"`
	PRURL       string         `json:"prURL,omitempty"`
	PrevHash    string         `json:"prevHash"`
}

type targetOut struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type modelOut struct {
	Model        string    `json:"model"`
	ResponseIDs  []string  `json:"responseIds"`
	InputTokens  int       `json:"inputTokens"`
	OutputTokens int       `json:"outputTokens"`
	PromptHash   string    `json:"promptHash"`
	Tools        []toolOut `json:"tools"`
}

type toolOut struct {
	Name     string `json:"name"`
	ArgsHash string `json:"argsHash"`
}

type decisionOut struct {
	State    string `json:"state"`
	Approver string `json:"approver,omitempty"`
	At       string `json:"at,omitempty"`
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Fingerprint is the audit hash primitive: lowercase hex sha256.
func Fingerprint(s string) string {
	return hash(s)
}

// Build binds a full action into one hashed evidence packet. The hash
// covers the canonical JSON including prevHash, chaining this receipt
// to the previous one. Use AppendRegistro to store it.
func Build(in Input) Receipt {
	body := receiptBody{
		Finding:    in.FindingID,
		Severity:   in.Severity,
		Category:   in.Category,
		Target:     targetOut{Kind: in.Kind, Namespace: in.Namespace, Name: in.Name},
		Rule:       in.Rule,
		Template:   in.Template,
		RootCause:  in.RootCause,
		Diff:       in.Diff,
		RevertPlan: in.RevertPlan,
		Model: modelOut{
			Model:        in.Model.Model,
			ResponseIDs:  append([]string{}, in.Model.ResponseIDs...),
			InputTokens:  in.Model.InputTokens,
			OutputTokens: in.Model.OutputTokens,
			PromptHash:   in.Model.PromptHash,
		},
		Decision: decisionOut{
			State:    in.Decision.State,
			Approver: in.Decision.Approver,
		},
		Snapshot: in.Snapshot,
		PRURL:    in.PRURL,
		PrevHash: in.PrevHash,
	}
	if body.Model.ResponseIDs == nil {
		body.Model.ResponseIDs = []string{}
	}
	for _, o := range in.Ops {
		// A JSON null value is equivalent to absent here: setting a
		// field to null carries no patch meaning the verifier could
		// act on, so it is dropped like a missing value.
		oo := opOut{Op: o.Op, Path: o.Path, Value: o.Value}
		body.Patch = append(body.Patch, oo)
	}
	if body.Patch == nil {
		body.Patch = []opOut{}
	}
	for _, s := range in.Ladder {
		body.Ladder = append(body.Ladder, stepOut{Step: s.Step, Pass: s.Pass, Detail: s.Detail, DurationMs: s.DurationMs})
	}
	if body.Ladder == nil {
		body.Ladder = []stepOut{}
	}
	when := ""
	if !in.Attr.When.IsZero() {
		when = in.Attr.When.UTC().Format(time.RFC3339)
	}
	body.Attribution = attributionOut{Level: in.Attr.Level.String(), Identity: in.Attr.Identity, When: when}
	for _, t := range in.Model.Tools {
		body.Model.Tools = append(body.Model.Tools, toolOut{Name: t.Name, ArgsHash: t.ArgsHash})
	}
	if body.Model.Tools == nil {
		body.Model.Tools = []toolOut{}
	}
	if !in.Decision.At.IsZero() {
		body.Decision.At = in.Decision.At.UTC().Format(time.RFC3339)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		// The struct graph is JSON-safe by construction.
		panic("receipt: marshal: " + err.Error())
	}
	return Receipt{JSON: string(raw), SHA256: hash(string(raw))}
}

// chainLine is one stored registry line: payload plus previous hash.
type chainLine struct {
	Payload string `json:"payload"`
	Prev    string `json:"prev"`
	// Sig is the optional hex HMAC-SHA256 over prev + "\n" + payload under
	// an operator-held key. Unsigned lines predate signatures and verify
	// with the legacy VerifyChain only.
	Sig string `json:"sig,omitempty"`
}

// Sign returns hex HMAC-SHA256 of prev + "\n" + payload under key. The key
// lives outside the chain (e.g. UMBRA_RECEIPT_HMAC_KEY): whoever can
// rewrite the file but lacks the key cannot forge a signed chain.
func Sign(prev, payload string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(prev + "\n" + payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// AppendRegistro appends payload to the registry chain, linking it to
// the hash of the previous line (or GENESIS for the first line).
// Lines are append-only: never edit or reorder them.
func AppendRegistro(chain []string, payload string) []string {
	prev := "GENESIS"
	if len(chain) > 0 {
		prev = hash(chain[len(chain)-1])
	}
	raw, err := json.Marshal(chainLine{Payload: payload, Prev: prev})
	if err != nil {
		panic("receipt: marshal chain line: " + err.Error())
	}
	return append(chain, string(raw))
}

// AppendRegistroSigned appends a keyed line: same link plus an HMAC over
// prev + payload, so wholesale rewrites without the key do not verify.
func AppendRegistroSigned(chain []string, payload string, key []byte) []string {
	prev := "GENESIS"
	if len(chain) > 0 {
		prev = hash(chain[len(chain)-1])
	}
	raw, err := json.Marshal(chainLine{Payload: payload, Prev: prev, Sig: Sign(prev, payload, key)})
	if err != nil {
		panic("receipt: marshal chain line: " + err.Error())
	}
	return append(chain, string(raw))
}

// VerifyChain checks every link of the registry chain. It returns nil
// when the chain is intact, else an error naming the first broken index.
func VerifyChain(chain []string) error {
	prev := "GENESIS"
	for i, line := range chain {
		var l chainLine
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			return fmt.Errorf("receipt: line %d not valid JSON: %w", i, err)
		}
		if l.Prev != prev {
			return fmt.Errorf("receipt: line %d broken link: want prev %q, got %q", i, prev, l.Prev)
		}
		prev = hash(line)
	}
	return nil
}

// VerifyChainSigned checks links and HMACs. Every line must carry a valid
// signature: an unsigned or re-signed-by-attacker line fails. An empty key
// is refused outright (fail closed: no key, no signed verification).
func VerifyChainSigned(chain []string, key []byte) error {
	if len(key) == 0 {
		return fmt.Errorf("receipt: signed verification needs a non-empty key")
	}
	prev := "GENESIS"
	for i, line := range chain {
		var l chainLine
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			return fmt.Errorf("receipt: line %d not valid JSON: %w", i, err)
		}
		if l.Prev != prev {
			return fmt.Errorf("receipt: line %d broken link: want prev %q, got %q", i, prev, l.Prev)
		}
		if l.Sig == "" || !hmac.Equal([]byte(l.Sig), []byte(Sign(l.Prev, l.Payload, key))) {
			return fmt.Errorf("receipt: line %d bad signature", i)
		}
		prev = hash(line)
	}
	return nil
}
