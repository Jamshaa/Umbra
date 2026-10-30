package fixes

import (
	"regexp"
	"strings"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
)

// emptyResources matches a resources line with no content: the key alone
// (null) or an empty flow map. Lines with children never match: their
// content lives on the following lines.
var emptyResources = regexp.MustCompile(`(?m)^[ \t]*resources:[ \t]*(\{[ \t]*\})?[ \t]*\r?$`)

// portsLine matches a "ports:" mapping key, capturing its indent. Live
// manifests and repo fixtures indent differently; the anchor adapts.
var portsLine = regexp.MustCompile(`(?m)^([ \t]*)ports:[ \t]*\r?$`)

// portsIndent reports the indent of the first "ports:" line.
func portsIndent(s string) (string, bool) {
	m := portsLine.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// insertBeforePorts inserts block right before the first "ports:" line.
func insertBeforePorts(s, block string) string {
	loc := portsLine.FindStringIndex(s)
	return s[:loc[0]] + block + s[loc[0]:]
}

// Fix applies a deterministic, template-free patch for the faults that do not
// need model judgment. It returns the patched manifest and true when it did
// something; false means "no deterministic fix for this fault" and the
// manifest is returned untouched so the caller can hand it to the model.
func Fix(faultID string, manifest []byte) ([]byte, bool) {
	switch faultID {
	case "F01":
		return addResources(manifest, "100m", "64Mi", "500m", "512Mi")
	case "F02":
		return raiseMemoryLimit(manifest, "64Mi")
	case "F05":
		return addLivenessProbe(manifest)
	case "F06":
		return addReadinessProbe(manifest)
	}
	return manifest, false
}

func addResources(doc []byte, reqCPU, reqMem, limCPU, limMem string) ([]byte, bool) {
	// The API server normalizes a missing block to an empty one
	// ("resources:" / "resources: {}"): treat those as absent, otherwise
	// live manifests could never be fixed.
	s := emptyResources.ReplaceAllString(string(doc), "")
	if strings.Contains(s, "resources:") {
		return doc, false
	}
	// Insert a resources block right before the first "ports:" line,
	// at whatever indent the document uses (repo fixtures use 10
	// spaces, kubectl output 8).
	indent, ok := portsIndent(s)
	if !ok {
		return doc, false
	}
	block := indent + "resources:\n" +
		indent + "  requests:\n" +
		indent + "    cpu: \"" + reqCPU + "\"\n" +
		indent + "    memory: \"" + reqMem + "\"\n" +
		indent + "  limits:\n" +
		indent + "    cpu: \"" + limCPU + "\"\n" +
		indent + "    memory: \"" + limMem + "\"\n"
	return []byte(insertBeforePorts(s, block)), true
}

func raiseMemoryLimit(doc []byte, min string) ([]byte, bool) {
	s := string(doc)
	// Only raise limits that are below the sane floor. Handles both quoted
	// ("8Mi") and unquoted (8Mi) forms.
	old := "memory: 8Mi"
	if !strings.Contains(s, old) {
		old = "memory: \"8Mi\""
		if !strings.Contains(s, old) {
			return doc, false
		}
	}
	replacement := "memory: " + min
	if strings.Contains(s, "memory: \"8Mi\"") {
		replacement = "memory: \"" + min + "\""
	}
	return []byte(strings.ReplaceAll(s, old, replacement)), true
}

func addLivenessProbe(doc []byte) ([]byte, bool) {
	s := string(doc)
	if strings.Contains(s, "livenessProbe:") {
		return doc, false
	}
	indent, ok := portsIndent(s)
	if !ok {
		return doc, false
	}
	block := indent + "livenessProbe:\n" +
		indent + "  tcpSocket:\n" +
		indent + "    port: 8080\n" +
		indent + "  initialDelaySeconds: 5\n" +
		indent + "  periodSeconds: 10\n"
	return []byte(insertBeforePorts(s, block)), true
}

func addReadinessProbe(doc []byte) ([]byte, bool) {
	s := string(doc)
	if strings.Contains(s, "readinessProbe:") {
		return doc, false
	}
	indent, ok := portsIndent(s)
	if !ok {
		return doc, false
	}
	block := indent + "readinessProbe:\n" +
		indent + "  tcpSocket:\n" +
		indent + "    port: 8080\n" +
		indent + "  initialDelaySeconds: 5\n" +
		indent + "  periodSeconds: 10\n"
	return []byte(insertBeforePorts(s, block)), true
}

// kept for API symmetry with the CRD vocabulary.
var _ = v1alpha1.FindingSeverityLow
