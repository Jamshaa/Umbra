package receipt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/attribution"
)

// StepResult is the outcome of one verification ladder step
// (for example invariants, dry-run, sandbox or rollback).
type StepResult struct {
	Step   string
	Pass   bool
	Detail string
}

// Receipt is the evidence packet: the canonical JSON document plus
// its SHA256. The same inputs always yield the same bytes.
type Receipt struct {
	JSON   string
	SHA256 string
}

type attributionOut struct {
	Field     string `json:"field"`
	Manager   string `json:"manager"`
	UpdatedAt string `json:"updatedAt"`
}

type stepOut struct {
	Step   string `json:"step"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

type receiptBody struct {
	Finding     string           `json:"finding"`
	Severity    string           `json:"severity"`
	Category    string           `json:"category"`
	Target      string           `json:"target"`
	RootCause   string           `json:"rootCause"`
	Diff        string           `json:"diff"`
	RevertPlan  string           `json:"revertPlan"`
	Ladder      []stepOut        `json:"ladder"`
	Attribution []attributionOut `json:"attribution"`
	PRURL       string           `json:"prURL,omitempty"`
}

// Build binds a Finding, its ladder results, its attribution and the
// PR URL (if any) into one hashed evidence packet. Root cause, diff
// and revert plan come from the Finding spec (Diagnosis, ProposedPatch
// and PrePatchManifest).
func Build(f v1alpha1.Finding, ladder []StepResult, attr []attribution.FieldAttribution, prURL string) Receipt {
	body := receiptBody{
		Finding:    f.Name,
		Severity:   string(f.Spec.Severity),
		Category:   string(f.Spec.Category),
		Target:     f.Spec.TargetRef.Kind + "/" + f.Spec.TargetRef.Namespace + "/" + f.Spec.TargetRef.Name,
		RootCause:  f.Spec.Diagnosis,
		Diff:       f.Spec.ProposedPatch,
		RevertPlan: f.Spec.PrePatchManifest,
		PRURL:      prURL,
	}
	for _, s := range ladder {
		body.Ladder = append(body.Ladder, stepOut{Step: s.Step, Pass: s.Pass, Detail: s.Detail})
	}
	if body.Ladder == nil {
		body.Ladder = []stepOut{}
	}
	for _, a := range attr {
		// shortcut: UpdatedAt drops sub-second precision; upgrade to RFC3339Nano if ordering within a second matters.
		body.Attribution = append(body.Attribution, attributionOut{
			Field: a.Field, Manager: a.Manager,
			UpdatedAt: a.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	if body.Attribution == nil {
		body.Attribution = []attributionOut{}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		// Structs with string/bool fields never fail to marshal.
		panic("receipt: marshal: " + err.Error())
	}
	sum := sha256.Sum256(raw)
	return Receipt{JSON: string(raw), SHA256: hex.EncodeToString(sum[:])}
}

// chainLine is one stored registry line: payload plus previous hash.
type chainLine struct {
	Payload string `json:"payload"`
	Prev    string `json:"prev"`
}

// AppendRegistro appends payload to the registry chain, linking it to
// the hash of the previous line (or GENESIS for the first line).
// Lines are append-only: never edit or reorder them.
func AppendRegistro(chain []string, payload string) []string {
	prev := "GENESIS"
	if len(chain) > 0 {
		sum := sha256.Sum256([]byte(chain[len(chain)-1]))
		prev = hex.EncodeToString(sum[:])
	}
	raw, err := json.Marshal(chainLine{Payload: payload, Prev: prev})
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
		sum := sha256.Sum256([]byte(line))
		prev = hex.EncodeToString(sum[:])
	}
	return nil
}
