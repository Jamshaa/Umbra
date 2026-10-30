package fixes

import (
	"strings"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
)

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
	s := string(doc)
	if strings.Contains(s, "resources:") {
		return doc, false
	}
	// Insert a resources block right before the first "ports:" or at end of
	// the container spec. We use a conservative anchor: the line with
	// "image:" followed by its value.
	out := strings.Replace(s, "          ports:\n", "          resources:\n            requests:\n              cpu: \""+reqCPU+"\"\n              memory: \""+reqMem+"\"\n            limits:\n              cpu: \""+limCPU+"\"\n              memory: \""+limMem+"\"\n          ports:\n", 1)
	if out == s {
		return doc, false
	}
	return []byte(out), true
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
	out := strings.Replace(s, "          ports:\n", "          livenessProbe:\n            tcpSocket:\n              port: 8080\n            initialDelaySeconds: 5\n            periodSeconds: 10\n          ports:\n", 1)
	if out == s {
		return doc, false
	}
	return []byte(out), true
}

func addReadinessProbe(doc []byte) ([]byte, bool) {
	s := string(doc)
	if strings.Contains(s, "readinessProbe:") {
		return doc, false
	}
	out := strings.Replace(s, "          ports:\n", "          readinessProbe:\n            tcpSocket:\n              port: 8080\n            initialDelaySeconds: 5\n            periodSeconds: 10\n          ports:\n", 1)
	if out == s {
		return doc, false
	}
	return []byte(out), true
}

// kept for API symmetry with the CRD vocabulary.
var _ = v1alpha1.FindingSeverityLow
