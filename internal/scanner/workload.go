package scanner

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
)

// workDoc is the subset shared by Deployment/StatefulSet/Job/Pod specs.
type workDoc struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Replicas *int32 `json:"replicas,omitempty"`
		Strategy struct {
			MaxUnavailable any `json:"maxUnavailable,omitempty"`
		} `json:"strategy,omitempty"`
		ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty"`
		UpdateStrategy          struct {
			Partition *int32 `json:"partition,omitempty"`
		} `json:"updateStrategy,omitempty"`
		Selector *struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector,omitempty"`
		Template struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Containers                    []wContainer     `json:"containers"`
				Volumes                       []map[string]any `json:"volumes,omitempty"`
				Affinity                      any              `json:"affinity,omitempty"`
				TopologySpreadConstraints     []any            `json:"topologySpreadConstraints,omitempty"`
				TerminationGracePeriodSeconds *int64           `json:"terminationGracePeriodSeconds,omitempty"`
				AutomountServiceAccountToken  *bool            `json:"automountServiceAccountToken,omitempty"`
				ServiceAccountName            string           `json:"serviceAccountName,omitempty"`
			} `json:"spec"`
		} `json:"template"`
		// StatefulSet headless service / Job knobs at spec level.
		ServiceName             string `json:"serviceName"`
		BackoffLimit            *int32 `json:"backoffLimit,omitempty"`
		ActiveDeadlineSeconds   *int64 `json:"activeDeadlineSeconds,omitempty"`
		TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
		Completions             *int32 `json:"completions,omitempty"`
		Parallelism             *int32 `json:"parallelism,omitempty"`
	} `json:"spec"`
}

type wContainer struct {
	Name            string         `json:"name"`
	Image           string         `json:"image,omitempty"`
	SecurityContext map[string]any `json:"securityContext,omitempty"`
	Resources       struct {
		Limits   map[string]string `json:"limits,omitempty"`
		Requests map[string]string `json:"requests,omitempty"`
	} `json:"resources,omitempty"`
	LivenessProbe  any              `json:"livenessProbe,omitempty"`
	ReadinessProbe any              `json:"readinessProbe,omitempty"`
	StartupProbe   any              `json:"startupProbe,omitempty"`
	Ports          []map[string]any `json:"ports,omitempty"`
}

func decodeWork(docs []byte) []workDoc {
	var out []workDoc
	for _, doc := range bytes.Split(docs, []byte("\n---")) {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		js, err := yaml.ToJSON(doc)
		if err != nil {
			continue
		}
		var w workDoc
		if err := json.Unmarshal(js, &w); err != nil {
			continue
		}
		switch w.Kind {
		case "Deployment", "StatefulSet", "Job", "ReplicaSet":
			out = append(out, w)
		}
	}
	return out
}

func workFinding(w workDoc, cat v1alpha1.FindingCategory, sev v1alpha1.FindingSeverity) v1alpha1.Finding {
	return v1alpha1.Finding{Spec: v1alpha1.FindingSpec{
		Severity: sev, Category: cat,
		TargetRef: v1alpha1.TargetRef{Kind: w.Kind, Namespace: w.Metadata.Namespace, Name: w.Metadata.Name},
		Detector:  DetectorName, State: v1alpha1.FindingStateDetected,
	}}
}

func containersOfW(w workDoc) []wContainer { return w.Spec.Template.Spec.Containers }

// DetectIdenticalProbes: liveness == readiness probes the same endpoint,
// so a wedged app never gets restarted (deadly + common).
func DetectIdenticalProbes(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, w := range decodeWork(docs) {
		hit := false
		for _, c := range containersOfW(w) {
			if c.LivenessProbe == nil || c.ReadinessProbe == nil {
				continue
			}
			if reflect.DeepEqual(normalizeProbe(c.LivenessProbe), normalizeProbe(c.ReadinessProbe)) {
				hit = true
			}
		}
		if hit {
			out = append(out, workFinding(w, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityMedium))
		}
	}
	return out
}

func normalizeProbe(p any) any {
	raw, _ := json.Marshal(p)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	delete(m, "initialDelaySeconds")
	delete(m, "periodSeconds")
	delete(m, "timeoutSeconds")
	delete(m, "successThreshold")
	delete(m, "failureThreshold")
	return m
}

// DetectSingleReplica: one replica is a single point of failure.
func DetectSingleReplica(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, w := range decodeWork(docs) {
		if w.Kind != "Deployment" && w.Kind != "StatefulSet" {
			continue
		}
		if w.Spec.Replicas != nil && *w.Spec.Replicas <= 1 {
			out = append(out, workFinding(w, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityMedium))
		}
	}
	return out
}

// DetectNoAntiAffinity: replicas on one node die together; neither
// affinity nor topologySpreadConstraints present.
func DetectNoAntiAffinity(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, w := range decodeWork(docs) {
		if w.Kind != "Deployment" && w.Kind != "StatefulSet" {
			continue
		}
		if w.Spec.Replicas != nil && *w.Spec.Replicas > 1 &&
			w.Spec.Template.Spec.Affinity == nil && len(w.Spec.Template.Spec.TopologySpreadConstraints) == 0 {
			out = append(out, workFinding(w, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityLow))
		}
	}
	return out
}

// DetectRiskyMaxUnavailable: maxUnavailable that takes all pods down at once.
func DetectRiskyMaxUnavailable(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, w := range decodeWork(docs) {
		if w.Kind != "Deployment" {
			continue
		}
		replicas := int32(3)
		if w.Spec.Replicas != nil {
			replicas = *w.Spec.Replicas
		}
		switch v := w.Spec.Strategy.MaxUnavailable.(type) {
		case string:
			if strings.TrimSpace(v) == "100%" {
				out = append(out, workFinding(w, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityHigh))
			}
		case float64:
			if int32(v) >= replicas && replicas > 0 {
				out = append(out, workFinding(w, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityHigh))
			}
		}
	}
	return out
}

// DetectInsecureContainer: root, privileged, privilege escalation,
// dangerous capabilities, latest tag, automounted default token.
func DetectInsecureContainer(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, w := range decodeWork(docs) {
		bad := false
		for _, c := range containersOfW(w) {
			img := c.Image
			if tag := imageTagOf(img); tag == "latest" || tag == "" {
				bad = true
			}
			sc := c.SecurityContext
			if sc != nil {
				if v, _ := sc["privileged"].(bool); v {
					bad = true
				}
				if v, _ := sc["allowPrivilegeEscalation"].(bool); v {
					bad = true
				}
				if v, _ := sc["runAsNonRoot"].(bool); !v {
					if n, ok := sc["runAsUser"].(float64); !ok || int(n) == 0 {
						// runAsUser 0 (or absent with runAsNonRoot unset)
						// means root; but absent context entirely is
						// inconclusive, so only flag explicit root.
						if ok {
							bad = true
						}
					}
				}
			}
			if caps, _ := sc["capabilities"].(map[string]any); caps != nil {
				for _, k := range []string{"add"} {
					if l, _ := caps[k].([]any); len(l) > 0 {
						for _, e := range l {
							if s, _ := e.(string); s == "SYS_ADMIN" || s == "NET_ADMIN" {
								bad = true
							}
						}
					}
				}
			}
		}
		if w.Spec.Template.Spec.AutomountServiceAccountToken != nil &&
			*w.Spec.Template.Spec.AutomountServiceAccountToken {
			bad = true
		}
		if bad {
			out = append(out, workFinding(w, v1alpha1.FindingCategorySecurity, v1alpha1.FindingSeverityHigh))
		}
	}
	return out
}

func imageTagOf(image string) string {
	if image == "" {
		return ""
	}
	if i := strings.Index(image, "@"); i >= 0 {
		return image[i+1:]
	}
	rest := image
	if i := strings.LastIndex(rest, "/"); i >= 0 {
		rest = rest[i+1:]
	}
	if i := strings.Index(rest, ":"); i >= 0 {
		return rest[i+1:]
	}
	return ""
}

// DetectStatefulSetPartition: a forgotten partition freezes rollouts.
func DetectStatefulSetPartition(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, w := range decodeWork(docs) {
		if w.Kind == "StatefulSet" && w.Spec.UpdateStrategy.Partition != nil && *w.Spec.UpdateStrategy.Partition > 0 {
			out = append(out, workFinding(w, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityMedium))
		}
	}
	return out
}

// DetectJobNoBounds: Job without backoffLimit/activeDeadline/ttl.
func DetectJobNoBounds(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, w := range decodeWork(docs) {
		if w.Kind != "Job" {
			continue
		}
		if w.Spec.BackoffLimit == nil || w.Spec.ActiveDeadlineSeconds == nil || w.Spec.TTLSecondsAfterFinished == nil {
			out = append(out, workFinding(w, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityMedium))
		}
		if w.Spec.Parallelism != nil && w.Spec.Completions != nil &&
			*w.Spec.Parallelism > *w.Spec.Completions {
			out = append(out, workFinding(w, v1alpha1.FindingCategoryConfig, v1alpha1.FindingSeverityMedium))
		}
	}
	return out
}
