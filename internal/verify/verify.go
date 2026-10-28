package verify

import (
	"bytes"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Thresholds mirror internal/scanner's (see doc.go on why they are
// duplicated instead of shared).
var (
	minSaneMemory = resource.MustParse("16Mi")
	maxSaneCPU    = resource.MustParse("32")
	maxSaneMemory = resource.MustParse("512Gi")
)

// PodStatus is the observed runtime state the harness feeds in.
// Production code fills it from the API server; tests fill it by hand.
type PodStatus struct {
	Phase                 string // e.g. "Running", "Pending"
	Ready                 bool
	Restarts              int32
	LastTerminationReason string // e.g. "OOMKilled", "" when never terminated
}

// Result is the verdict of one gate check. Reason is always set, pass or
// fail: a verifier that cannot explain itself is not deterministic, it is
// just quiet.
type Result struct {
	Pass   bool
	Reason string
}

func fail(format string, args ...any) Result {
	return Result{Pass: false, Reason: fmt.Sprintf(format, args...)}
}

// container mirrors the subset of a container spec the gates read.
type container struct {
	Name           string    `json:"name"`
	Resources      resources `json:"resources,omitempty"`
	LivenessProbe  *struct{} `json:"livenessProbe,omitempty"`
	ReadinessProbe *struct{} `json:"readinessProbe,omitempty"`
}

type resources struct {
	Limits   map[string]string `json:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty"`
}

type manifest struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []container `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
		Containers []container `json:"containers"`
	} `json:"spec"`
}

func containersOf(m manifest) []container {
	if len(m.Spec.Template.Spec.Containers) > 0 {
		return m.Spec.Template.Spec.Containers
	}
	return m.Spec.Containers
}

func decodeOne(doc []byte) (manifest, bool) {
	js, err := yaml.ToJSON(bytes.TrimSpace(doc))
	if err != nil {
		return manifest{}, false
	}
	var m manifest
	if err := json.Unmarshal(js, &m); err != nil || m.Kind == "" {
		return manifest{}, false
	}
	return m, true
}

func runningAndReady(pod PodStatus, what string) *Result {
	if pod.Phase != "Running" {
		r := fail("%s: pod phase is %q, want Running", what, pod.Phase)
		return &r
	}
	if !pod.Ready {
		r := fail("%s: pod Running but not Ready", what)
		return &r
	}
	return nil
}

func qty(m map[string]string, key string) (resource.Quantity, bool) {
	raw, ok := m[key]
	if !ok {
		return resource.Quantity{}, false
	}
	q, err := resource.ParseQuantity(raw)
	if err != nil {
		return resource.Quantity{}, false
	}
	return q, true
}

// VerifyF01 gates the F01 fix: limits present and sane on every container,
// and the pod Running and Ready.
func VerifyF01(doc []byte, pod PodStatus) Result {
	m, ok := decodeOne(doc)
	if !ok {
		return fail("F01: manifest does not decode")
	}
	for _, c := range containersOf(m) {
		lim, ok := qty(c.Resources.Limits, "memory")
		if !ok {
			return fail("F01: container %q has no memory limit", c.Name)
		}
		if lim.Sign() <= 0 {
			return fail("F01: container %q has non-positive memory limit", c.Name)
		}
		if req, ok := qty(c.Resources.Requests, "memory"); ok && lim.Cmp(req) < 0 {
			return fail("F01: container %q limit below request", c.Name)
		}
	}
	if r := runningAndReady(pod, "F01"); r != nil {
		return *r
	}
	return Result{Pass: true, Reason: "F01: limits present and sane, pod Running and Ready"}
}

// VerifyF02 gates the F02 fix: memory limit sane and no recent OOMKill.
func VerifyF02(doc []byte, pod PodStatus) Result {
	m, ok := decodeOne(doc)
	if !ok {
		return fail("F02: manifest does not decode")
	}
	for _, c := range containersOf(m) {
		lim, ok := qty(c.Resources.Limits, "memory")
		if !ok {
			return fail("F02: container %q has no memory limit", c.Name)
		}
		if lim.Cmp(minSaneMemory) < 0 {
			return fail("F02: container %q limit %s below sane minimum %s", c.Name, lim.String(), minSaneMemory.String())
		}
	}
	if pod.LastTerminationReason == "OOMKilled" {
		return fail("F02: last termination was OOMKilled")
	}
	if r := runningAndReady(pod, "F02"); r != nil {
		return *r
	}
	return Result{Pass: true, Reason: "F02: sane memory limit, no OOMKill, pod Running and Ready"}
}

// VerifyF03 gates the F03 fix: requests within schedulable bounds.
func VerifyF03(doc []byte, pod PodStatus) Result {
	m, ok := decodeOne(doc)
	if !ok {
		return fail("F03: manifest does not decode")
	}
	for _, c := range containersOf(m) {
		if q, ok := qty(c.Resources.Requests, "cpu"); ok && q.Cmp(maxSaneCPU) > 0 {
			return fail("F03: container %q requests cpu %s above schedulable %s", c.Name, q.String(), maxSaneCPU.String())
		}
		if q, ok := qty(c.Resources.Requests, "memory"); ok && q.Cmp(maxSaneMemory) > 0 {
			return fail("F03: container %q requests memory %s above schedulable %s", c.Name, q.String(), maxSaneMemory.String())
		}
	}
	if r := runningAndReady(pod, "F03"); r != nil {
		return *r
	}
	return Result{Pass: true, Reason: "F03: requests schedulable, pod Running and Ready"}
}

// VerifyF04 gates the F04 fix: limits present (bursty load stays bounded)
// and no restart escalation.
func VerifyF04(doc []byte, pod PodStatus) Result {
	m, ok := decodeOne(doc)
	if !ok {
		return fail("F04: manifest does not decode")
	}
	for _, c := range containersOf(m) {
		if _, ok := qty(c.Resources.Limits, "memory"); !ok {
			return fail("F04: container %q has no memory limit", c.Name)
		}
	}
	if pod.Restarts > 1 {
		return fail("F04: %d restarts show escalation under load", pod.Restarts)
	}
	if r := runningAndReady(pod, "F04"); r != nil {
		return *r
	}
	return Result{Pass: true, Reason: "F04: limits present, restarts stable, pod Running and Ready"}
}

// VerifyF05 gates the F05 fix: liveness probe defined and pod Running.
func VerifyF05(doc []byte, pod PodStatus) Result {
	m, ok := decodeOne(doc)
	if !ok {
		return fail("F05: manifest does not decode")
	}
	for _, c := range containersOf(m) {
		if c.LivenessProbe == nil {
			return fail("F05: container %q has no liveness probe", c.Name)
		}
	}
	if pod.Phase != "Running" {
		return fail("F05: pod phase is %q, want Running", pod.Phase)
	}
	return Result{Pass: true, Reason: "F05: liveness probe defined and pod Running"}
}

// VerifyF06 gates the F06 fix: readiness probe defined and pod Ready.
func VerifyF06(doc []byte, pod PodStatus) Result {
	m, ok := decodeOne(doc)
	if !ok {
		return fail("F06: manifest does not decode")
	}
	for _, c := range containersOf(m) {
		if c.ReadinessProbe == nil {
			return fail("F06: container %q has no readiness probe", c.Name)
		}
	}
	if r := runningAndReady(pod, "F06"); r != nil {
		return *r
	}
	return Result{Pass: true, Reason: "F06: readiness probe defined, pod Running and Ready"}
}
