package template

import (
	"k8s.io/apimachinery/pkg/api/resource"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
)

// Violation is one concrete deviation from a ComplianceTemplate.
type Violation struct {
	Kind      string
	Namespace string
	Name      string
	Field     string
	Expected  string
	Actual    string
	Message   string
}

// resourceFor pulls one resource value out of a container's resources block.
func resourceFor(c map[string]any, bucket, name string) (string, bool) {
	if c == nil {
		return "", false
	}
	b, ok := c[bucket].(map[string]any)
	if !ok {
		return "", false
	}
	v, ok := b[name].(string)
	return v, ok
}

// hasProbe reports whether the named probe exists on a container.
func hasProbe(c map[string]any, name string) bool {
	_, ok := c[name].(map[string]any)
	return ok
}

// imageTag returns the tag of an image reference, or "" when there is none.
func imageTag(image string) string {
	if image == "" {
		return ""
	}
	// digest wins over tag
	if i := len(image) - 1; i >= 0 && image[i] == ')' {
		// no-op guard
	}
	if idx := indexOf(image, "@"); idx >= 0 {
		return image[idx+1:]
	}
	if idx := lastSlash(image); idx >= 0 {
		image = image[idx+1:]
	}
	if idx := indexOf(image, ":"); idx >= 0 {
		return image[idx+1:]
	}
	return ""
}

func indexOf(s string, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func lastSlash(s string) int {
	idx := -1
	for i := range s {
		if s[i] == '/' {
			idx = i
		}
	}
	return idx
}

// Evaluate compares every object against the template and reports violations.
// Non-workload kinds are ignored silently.
func Evaluate(tpl v1alpha1.ComplianceTemplate, objs []map[string]any) []Violation {
	var out []Violation
	for _, obj := range objs {
		kind, _ := obj["kind"].(string)
		if !applies(tpl, obj, kind) {
			continue
		}
		meta, _ := obj["metadata"].(map[string]any)
		name, _ := meta["name"].(string)
		ns, _ := meta["namespace"].(string)
		spec, _ := obj["spec"].(map[string]any)
		template, _ := spec["template"].(map[string]any)
		podSpec, _ := template["spec"].(map[string]any)
		containers, _ := podSpec["containers"].([]any)
		if len(containers) == 0 {
			// Pod objects (no template wrapper).
			if cs, ok := spec["containers"].([]any); ok {
				containers = cs
			}
		}
		for _, raw := range containers {
			c, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, checkContainer(tpl, kind, ns, name, c)...)
		}
		// CronJob concurrencyPolicy is a pod-template-independent field.
		if kind == "CronJob" && tpl.Spec.Required.ConcurrencyPolicy != "" {
			if cp, _ := spec["concurrencyPolicy"].(string); cp != tpl.Spec.Required.ConcurrencyPolicy {
				out = append(out, Violation{
					Kind: kind, Namespace: ns, Name: name,
					Field:    "spec.concurrencyPolicy",
					Expected: tpl.Spec.Required.ConcurrencyPolicy,
					Actual:   cp,
					Message:  "concurrencyPolicy required by template",
				})
			}
		}
	}
	return out
}

func applies(tpl v1alpha1.ComplianceTemplate, obj map[string]any, kind string) bool {
	for _, t := range tpl.Spec.AppliesTo {
		if t.Kind != kind {
			continue
		}
		if len(t.MatchLabels) == 0 {
			return true
		}
		meta, _ := obj["metadata"].(map[string]any)
		labels, _ := meta["labels"].(map[string]any)
		ok := true
		for k, v := range t.MatchLabels {
			got, _ := labels[k].(string)
			if got != v {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func checkContainer(tpl v1alpha1.ComplianceTemplate, kind, ns, name string, c map[string]any) []Violation {
	var out []Violation
	res := tpl.Spec.Required.Resources
	if res != nil {
		for bucket, rules := range map[string]map[string]v1alpha1.ResourceRule{"limits": res.Limits, "requests": res.Requests} {
			for rname, rule := range rules {
				actual, present := resourceFor(c, "resources", bucket+"."+rname)
				// resources block shape: c["resources"].(map)["limits"].(map)[name]
				if !present {
					// try the real nested shape
					if r, ok := c["resources"].(map[string]any); ok {
						if b, ok := r[bucket].(map[string]any); ok {
							if v, ok := b[rname].(string); ok {
								actual = v
								present = true
							}
						}
					}
				}
				if rule.Required && !present {
					out = append(out, Violation{Kind: kind, Namespace: ns, Name: name, Field: "resources." + bucket + "." + rname, Expected: "present", Actual: "(absent)", Message: "resource required by template"})
					continue
				}
				if rule.Min != "" && present {
					minQ, err := resource.ParseQuantity(rule.Min)
					if err == nil {
						actQ, err := resource.ParseQuantity(actual)
						if err == nil && actQ.Cmp(minQ) < 0 {
							out = append(out, Violation{Kind: kind, Namespace: ns, Name: name, Field: "resources." + bucket + "." + rname, Expected: ">=" + rule.Min, Actual: actual, Message: "below template minimum"})
						}
					}
				}
				// Extended resource: requests must equal limits (nvidia.com/gpu).
				if rule.Required && rname == "nvidia.com/gpu" {
					lim, _ := resourceFor(c, "resources", "limits."+rname)
					req, _ := resourceFor(c, "resources", "requests."+rname)
					// Fallback to the nested shape.
					if lim == "" {
						if r, ok := c["resources"].(map[string]any); ok {
							if b, ok := r["limits"].(map[string]any); ok {
								if v, ok := b[rname].(string); ok {
									lim = v
								}
							}
						}
					}
					if req == "" {
						if r, ok := c["resources"].(map[string]any); ok {
							if b, ok := r["requests"].(map[string]any); ok {
								if v, ok := b[rname].(string); ok {
									req = v
								}
							}
						}
					}
					if lim != "" && req != "" && lim != req {
						out = append(out, Violation{Kind: kind, Namespace: ns, Name: name, Field: "resources.requests." + rname, Expected: lim, Actual: req, Message: "extended resource requests must equal limits"})
					}
				}
			}
		}
	}
	if p := tpl.Spec.Required.Probes; p != nil {
		if p.Liveness == v1alpha1.ProbePolicyRequired && !hasProbe(c, "livenessProbe") {
			out = append(out, Violation{Kind: kind, Namespace: ns, Name: name, Field: "livenessProbe", Expected: "present", Actual: "(absent)", Message: "liveness probe required by template"})
		}
		if p.Readiness == v1alpha1.ProbePolicyRequired && !hasProbe(c, "readinessProbe") {
			out = append(out, Violation{Kind: kind, Namespace: ns, Name: name, Field: "readinessProbe", Expected: "present", Actual: "(absent)", Message: "readiness probe required by template"})
		}
	}
	if img := tpl.Spec.Required.Image; img != nil && img.TagPolicy == v1alpha1.ImageTagPolicyExplicit {
		image, _ := c["image"].(string)
		// container spec may nest image under nested key
		if image == "" {
			if v, ok := c["image"].(string); ok {
				image = v
			}
		}
		tag := imageTag(image)
		if tag == "" {
			out = append(out, Violation{Kind: kind, Namespace: ns, Name: name, Field: "image", Expected: "explicit tag", Actual: "(no tag)", Message: "image must carry an explicit tag (digest accepted)"})
		} else if tag == "latest" {
			out = append(out, Violation{Kind: kind, Namespace: ns, Name: name, Field: "image", Expected: "explicit tag", Actual: "latest", Message: "image pinned to :latest is not allowed"})
		}
	}
	return out
}

// resourceFor is also used for the nested shape; keep a helper for the
// flat "resources.limits.cpu" string used by some manifests.
func init() {}
