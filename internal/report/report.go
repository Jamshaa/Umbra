package report

import (
	"fmt"
	"strings"
)

// Outcome is the per-rule verdict. Unknown values are listed verbatim
// but never counted in the summary.
type Outcome string

const (
	Pass  Outcome = "pass"
	Fail  Outcome = "fail"
	Warn  Outcome = "warn"
	Error Outcome = "error"
	Skip  Outcome = "skip"
)

// Result is one rule evaluation against one resource.
type Result struct {
	Policy   string
	Rule     string
	Outcome  Outcome
	Message  string
	Resource string
}

// Build renders scope+template+results as a PolicyReport YAML document.
// The same input always yields the same bytes.
func Build(scope, template string, results []Result) []byte {
	var b strings.Builder
	b.WriteString("apiVersion: wgpolicyk8s.io/v1alpha2\nkind: PolicyReport\nmetadata:\n")
	fmt.Fprintf(&b, "  name: %s\n", reportName(scope, template))
	b.WriteString("  labels:\n    umbra.io/managed-by: report\n")
	fmt.Fprintf(&b, "scope:\n  namespace: %s\n", yamlQuote(scope))
	b.WriteString("results:\n")
	counts := map[Outcome]int{}
	for _, r := range results {
		fmt.Fprintf(&b, "- policy: %s\n  rule: %s\n  result: %s\n  message: %s\n  resource: %s\n",
			yamlQuote(r.Policy), yamlQuote(r.Rule), yamlQuote(string(r.Outcome)),
			yamlQuote(r.Message), yamlQuote(r.Resource))
		switch r.Outcome {
		case Pass, Fail, Warn, Error, Skip:
			counts[r.Outcome]++
		}
	}
	if len(results) == 0 {
		b.WriteString("  []\n")
	}
	b.WriteString("summary:\n")
	for _, o := range []Outcome{Pass, Fail, Warn, Error, Skip} {
		fmt.Fprintf(&b, "  %s: %d\n", o, counts[o])
	}
	return []byte(b.String())
}

// reportName derives a DNS-1123-safe deterministic name from scope and
// template. Same inputs always give the same name.
func reportName(scope, template string) string {
	raw := strings.ToLower("umbra-" + scope + "-" + template)
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if len(name) > 253 {
		name = name[:253]
	}
	if name == "" {
		name = "umbra-report"
	}
	return name
}

// yamlQuote emits s plain when it is plain-safe, else double-quoted
// with escapes. Enough for names, messages and resource refs.
func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	plain := true
	for i, r := range s {
		if r > 0x7e || r < 0x20 {
			plain = false
			break
		}
		switch r {
		case '"', '\'', '#', ',', '[', ']', '{', '}', '&', '*', '!', '|', '>', '@', '`', '%', '\\', ':':
			plain = false
		case '-', '?', '.':
			if i == 0 {
				plain = false
			}
		case ' ':
			if i == 0 {
				plain = false
			}
		}
		if !plain {
			break
		}
	}
	if plain {
		return s
	}
	var b strings.Builder
	b.WriteRune('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteRune('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r > 0x7e {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteRune('"')
	return b.String()
}
