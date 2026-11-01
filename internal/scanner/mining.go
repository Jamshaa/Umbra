package scanner

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/attribution"
)

// TemplateStats is the mined de-facto standard of one workload group.
type TemplateStats struct {
	Kind           string
	Count          int
	FieldPresence  map[string]float64 // field -> fraction present
	MemoryMinP10   string             // p10 of memory limits, "" when unknown
	CPURequestsP10 string
}

// MineTemplate analyses all Deployment-like docs and proposes the standard:
// fields present in >=80% become required, memory floor is the p10.
func MineTemplate(docs []byte) (TemplateStats, v1alpha1.ComplianceTemplate) {
	docs2 := decodeAll(docs)
	stats := TemplateStats{Kind: "Deployment", FieldPresence: map[string]float64{}}
	var mems []resource.Quantity
	present := map[string]int{}
	total := 0
	for _, g := range docs2 {
		if g.Kind != "Deployment" && g.Kind != "StatefulSet" {
			continue
		}
		total++
		var m manifest
		raw, _ := json.Marshal(map[string]any{"kind": g.Kind, "metadata": g.Metadata, "spec": g.Spec})
		if err := json.Unmarshal(raw, &m); err != nil {
			total--
			continue
		}
		for _, c := range containersOf(m) {
			if len(c.Resources.Limits) > 0 || len(c.Resources.Requests) > 0 {
				present["resources"]++
			}
			if c.LivenessProbe != nil {
				present["livenessProbe"]++
			}
			if c.ReadinessProbe != nil {
				present["readinessProbe"]++
			}
			if q, ok := c.Resources.Limits["memory"]; ok {
				if v, err := resource.ParseQuantity(q); err == nil {
					mems = append(mems, v)
				}
			}
		}
	}
	stats.Count = total
	for f, n := range present {
		if total > 0 {
			stats.FieldPresence[f] = float64(n) / float64(total)
		}
	}
	sort.Slice(mems, func(i, j int) bool { return mems[i].Cmp(mems[j]) < 0 })
	if len(mems) > 0 {
		stats.MemoryMinP10 = mems[len(mems)/10].String()
	}
	tpl := v1alpha1.ComplianceTemplate{}
	tpl.Spec.AppliesTo = []v1alpha1.TargetKind{{Kind: "Deployment"}}
	if stats.FieldPresence["resources"] >= 0.8 {
		if tpl.Spec.Required.Resources == nil {
			tpl.Spec.Required.Resources = &v1alpha1.ResourceRequirements{
				Limits: map[string]v1alpha1.ResourceRule{}, Requests: map[string]v1alpha1.ResourceRule{},
			}
		}
		min := stats.MemoryMinP10
		if min == "" {
			min = "64Mi"
		}
		tpl.Spec.Required.Resources.Limits["memory"] = v1alpha1.ResourceRule{Required: true, Min: min}
		tpl.Spec.Required.Resources.Requests["memory"] = v1alpha1.ResourceRule{Required: true}
	}
	if stats.FieldPresence["livenessProbe"] >= 0.8 || stats.FieldPresence["readinessProbe"] >= 0.8 {
		tpl.Spec.Required.Probes = &v1alpha1.ProbeRequirements{}
		if stats.FieldPresence["livenessProbe"] >= 0.8 {
			tpl.Spec.Required.Probes.Liveness = v1alpha1.ProbePolicyRequired
		}
		if stats.FieldPresence["readinessProbe"] >= 0.8 {
			tpl.Spec.Required.Probes.Readiness = v1alpha1.ProbePolicyRequired
		}
	}
	return stats, tpl
}

// GroupByCause clusters findings by image/template label: one bad base
// image or template explains many findings; fix once.
func GroupByCause(findings []v1alpha1.Finding, labels func(v1alpha1.Finding) string) map[string][]v1alpha1.Finding {
	out := map[string][]v1alpha1.Finding{}
	for _, f := range findings {
		k := "unknown"
		if labels != nil {
			if l := labels(f); l != "" {
				k = l
			}
		} else {
			k = string(f.Spec.Category) + "/" + f.Spec.TargetRef.Kind
		}
		out[k] = append(out[k], f)
	}
	return out
}

// ImpactScore grades a finding 0-100 by dependency neighbourhood size:
// Services selecting it, volumes, bindings. Deterministic, no AI.
func ImpactScore(services, configMaps, secrets, pvcs, bindings int, kind string) int {
	s := 10 + 8*services + 4*configMaps + 6*secrets + 8*pvcs + 5*bindings
	switch kind {
	case "CronJob":
		s += 10 // time bombs affect future runs
	case "Namespace", "ClusterRole", "ClusterRoleBinding":
		s += 30 // blast radius is the whole cluster
	}
	if s > 100 {
		s = 100
	}
	return s
}

// CorrelateChanges answers "what changed just before it broke": managed
// field attributions + event lines inside the 2h window before failTime,
// rendered newest-last for the timeline. Pure: no cluster calls.
func CorrelateChanges(fields []attribution.FieldAttribution, events []string, failTime time.Time) []string {
	var out []string
	cutoff := failTime.Add(-2 * time.Hour)
	for _, f := range fields {
		if f.UpdatedAt.IsZero() || f.UpdatedAt.Before(cutoff) || f.UpdatedAt.After(failTime) {
			continue
		}
		out = append(out, fmt.Sprintf("%s changed by %s", f.Field, f.Manager))
	}
	sort.Strings(out)
	// Events carry their own timestamps as text prefix; keep those that
	// mention the window is impossible to parse reliably, so cap output.
	for _, e := range events {
		if strings.TrimSpace(e) == "" {
			continue
		}
		out = append(out, "event: "+e)
		if len(out) >= 20 {
			break
		}
	}
	return out
}
