package receipt

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/attribution"
)

func seedFinding() v1alpha1.Finding {
	return v1alpha1.Finding{
		ObjectMeta: metav1.ObjectMeta{Name: "f01"},
		Spec: v1alpha1.FindingSpec{
			Severity:         v1alpha1.FindingSeverityHigh,
			Category:         v1alpha1.FindingCategoryResources,
			TargetRef:        v1alpha1.TargetRef{Kind: "Deployment", Namespace: "demo", Name: "web"},
			Detector:         "scanner",
			Diagnosis:        "no resources set",
			ProposedPatch:    "add limits",
			PrePatchManifest: "original yaml",
			State:            v1alpha1.FindingStateFixProposed,
		},
	}
}

func TestBuildDeterministic(t *testing.T) {
	f := seedFinding()
	ladder := []StepResult{{Step: "sandbox", Pass: true, Detail: "green"}, {Step: "rollback", Pass: false, Detail: "boom"}}
	attr := []attribution.FieldAttribution{{Field: "spec.replicas", Manager: "kubectl", UpdatedAt: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)}}
	a, b := Build(f, ladder, attr, "https://example.com/pr/1"), Build(f, ladder, attr, "https://example.com/pr/1")
	if a.JSON != b.JSON || a.SHA256 != b.SHA256 {
		t.Fatal("same input gave different receipts")
	}
	sum := sha256.Sum256([]byte(a.JSON))
	if a.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("SHA256 %q does not match JSON body", a.SHA256)
	}
	for _, want := range []string{
		`"finding":"f01"`, `"rootCause":"no resources set"`, `"diff":"add limits"`,
		`"revertPlan":"original yaml"`, `"step":"sandbox"`, `"pass":true`,
		`"field":"spec.replicas"`, `"manager":"kubectl"`,
		`"prURL":"https://example.com/pr/1"`, `Deployment/demo/web`,
	} {
		if !strings.Contains(a.JSON, want) {
			t.Fatalf("receipt missing %s:\n%s", want, a.JSON)
		}
	}
}

func TestBuildEmpty(t *testing.T) {
	r := Build(seedFinding(), nil, nil, "")
	if !strings.Contains(r.JSON, `"ladder":[]`) || !strings.Contains(r.JSON, `"attribution":[]`) {
		t.Fatalf("empty inputs not rendered as []:\n%s", r.JSON)
	}
	if strings.Contains(r.JSON, "prURL") {
		t.Fatalf("empty prURL should be omitted:\n%s", r.JSON)
	}
}

func TestChainDetectsTamper(t *testing.T) {
	var chain []string
	for _, p := range []string{"first", "second", "third"} {
		chain = AppendRegistro(chain, p)
	}
	if err := VerifyChain(chain); err != nil {
		t.Fatalf("intact chain rejected: %v", err)
	}
	if err := VerifyChain(nil); err != nil {
		t.Fatalf("empty chain rejected: %v", err)
	}
	bad := append([]string{}, chain...)
	bad[1] = strings.Replace(bad[1], "second", "SECOND", 1)
	if err := VerifyChain(bad); err == nil {
		t.Fatal("tampered chain accepted")
	} else if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error should name line 2, got: %v", err)
	}
	truncated := chain[:2]
	if err := VerifyChain(truncated); err != nil {
		t.Fatalf("prefix chain rejected: %v", err)
	}
}
