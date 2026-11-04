package scanner

import (
	"bytes"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
)

// cronSpec is the subset of a CronJob the detectors read. Pointers
// distinguish "absent" from zero: absent deadline is the 100-missed trap.
type cronSpec struct {
	Schedule                   string `json:"schedule"`
	Suspend                    *bool  `json:"suspend,omitempty"`
	ConcurrencyPolicy          string `json:"concurrencyPolicy"`
	StartingDeadlineSeconds    *int64 `json:"startingDeadlineSeconds,omitempty"`
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`
	FailedJobsHistoryLimit     *int32 `json:"failedJobsHistoryLimit,omitempty"`
	TimeZone                   string `json:"timeZone"`
	JobTemplate                struct {
		Spec struct {
			TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
			BackoffLimit            *int32 `json:"backoffLimit,omitempty"`
			ActiveDeadlineSeconds   *int64 `json:"activeDeadlineSeconds,omitempty"`
			Template                struct {
				Spec struct {
					Containers   []container       `json:"containers"`
					NodeSelector map[string]string `json:"nodeSelector,omitempty"`
					Tolerations  []map[string]any  `json:"tolerations,omitempty"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	} `json:"jobTemplate"`
}

type cronDoc struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec cronSpec `json:"spec"`
}

func decodeCron(docs []byte) []cronDoc {
	var out []cronDoc
	for _, doc := range bytes.Split(docs, []byte("\n---")) {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		js, err := yaml.ToJSON(doc)
		if err != nil {
			continue
		}
		var c cronDoc
		if err := json.Unmarshal(js, &c); err != nil || c.Kind != "CronJob" {
			continue
		}
		out = append(out, c)
	}
	return out
}

func cronFinding(c cronDoc, cat v1alpha1.FindingCategory, sev v1alpha1.FindingSeverity) v1alpha1.Finding {
	return v1alpha1.Finding{Spec: v1alpha1.FindingSpec{
		Severity: sev, Category: cat,
		TargetRef: v1alpha1.TargetRef{Kind: "CronJob", Namespace: c.Metadata.Namespace, Name: c.Metadata.Name},
		Detector:  DetectorName, State: v1alpha1.FindingStateDetected,
	}}
}

// DetectCronMissingDeadline: no startingDeadlineSeconds means 100 missed
// starts silently stop the CronJob (controller counts them, then gives up).
func DetectCronMissingDeadline(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, c := range decodeCron(docs) {
		if c.Spec.StartingDeadlineSeconds == nil && (c.Spec.Suspend == nil || !*c.Spec.Suspend) {
			out = append(out, cronFinding(c, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityHigh))
		}
	}
	return out
}

// DetectCronShortDeadline: the controller checks ~every 10s; a shorter
// deadline may never be met.
func DetectCronShortDeadline(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, c := range decodeCron(docs) {
		if c.Spec.StartingDeadlineSeconds != nil && *c.Spec.StartingDeadlineSeconds < 10 {
			out = append(out, cronFinding(c, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityHigh))
		}
	}
	return out
}

// DetectCronSuspended: suspend:true stops all starts; misses accumulate
// and resume without a deadline fires a burst.
func DetectCronSuspended(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, c := range decodeCron(docs) {
		if c.Spec.Suspend != nil && *c.Spec.Suspend {
			out = append(out, cronFinding(c, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityMedium))
		}
	}
	return out
}

// DetectCronOverlap: Allow (or absent, which defaults to Allow) lets a new
// Job start while the previous still runs.
func DetectCronOverlap(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, c := range decodeCron(docs) {
		if c.Spec.ConcurrencyPolicy == "" || strings.EqualFold(c.Spec.ConcurrencyPolicy, "Allow") {
			out = append(out, cronFinding(c, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityMedium))
		}
	}
	return out
}

// DetectCronMissingTimezone: without timeZone the controller zone (UTC)
// decides DST behavior.
func DetectCronMissingTimezone(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, c := range decodeCron(docs) {
		if strings.TrimSpace(c.Spec.TimeZone) == "" {
			out = append(out, cronFinding(c, v1alpha1.FindingCategoryConfig, v1alpha1.FindingSeverityLow))
		}
	}
	return out
}

// DetectCronMissingJobBounds: no backoffLimit/activeDeadlineSeconds means a
// Job can hang forever; no ttlSecondsAfterFinished accumulates objects.
func DetectCronMissingJobBounds(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, c := range decodeCron(docs) {
		js := c.Spec.JobTemplate.Spec
		if js.BackoffLimit == nil || js.ActiveDeadlineSeconds == nil || js.TTLSecondsAfterFinished == nil {
			out = append(out, cronFinding(c, v1alpha1.FindingCategoryReliability, v1alpha1.FindingSeverityMedium))
		}
	}
	return out
}

// DetectCronHistoryBloat: high history limits with no TTL accumulate Jobs.
func DetectCronHistoryBloat(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, c := range decodeCron(docs) {
		if c.Spec.SuccessfulJobsHistoryLimit != nil && *c.Spec.SuccessfulJobsHistoryLimit > 5 {
			out = append(out, cronFinding(c, v1alpha1.FindingCategoryResources, v1alpha1.FindingSeverityLow))
			continue
		}
		if c.Spec.FailedJobsHistoryLimit != nil && *c.Spec.FailedJobsHistoryLimit > 5 {
			out = append(out, cronFinding(c, v1alpha1.FindingCategoryResources, v1alpha1.FindingSeverityLow))
		}
	}
	return out
}

// DetectCronGPUWithoutPlacement: a GPU request without nodeSelector or
// tolerations can stay Pending forever (scheduler signal, verify by event).
func DetectCronGPUWithoutPlacement(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, c := range decodeCron(docs) {
		ps := c.Spec.JobTemplate.Spec.Template.Spec
		found := false
		for _, cont := range ps.Containers {
			for _, m := range []map[string]string{cont.Resources.Limits, cont.Resources.Requests} {
				for k := range m {
					if strings.Contains(k, "nvidia.com/gpu") || strings.Contains(k, "amd.com/gpu") {
						if len(ps.NodeSelector) == 0 && len(ps.Tolerations) == 0 {
							found = true
						}
					}
				}
			}
		}
		if found {
			out = append(out, cronFinding(c, v1alpha1.FindingCategoryResources, v1alpha1.FindingSeverityMedium))
		}
	}
	return out
}
