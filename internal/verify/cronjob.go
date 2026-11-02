package verify

import (
	"bytes"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"
)

type cronManifest struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
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
			} `json:"spec"`
		} `json:"jobTemplate"`
	} `json:"spec"`
	Status struct {
		LastScheduleTime   string `json:"lastScheduleTime"`
		LastSuccessfulTime string `json:"lastSuccessfulTime"`
		Active             []any  `json:"active"`
	} `json:"status"`
}

func decodeCron(doc []byte) (cronManifest, bool) {
	js, err := yaml.ToJSON(bytes.TrimSpace(doc))
	if err != nil {
		return cronManifest{}, false
	}
	var m cronManifest
	if err := json.Unmarshal(js, &m); err != nil || m.Kind != "CronJob" {
		return cronManifest{}, false
	}
	return m, true
}

// VerifyCronDeadline gates the 100-missed fix: a deadline is set and sane.
func VerifyCronDeadline(doc []byte, _ PodStatus) Result {
	m, ok := decodeCron(doc)
	if !ok {
		return fail("CronDeadline: manifest does not decode as CronJob")
	}
	if m.Spec.StartingDeadlineSeconds == nil {
		return fail("CronDeadline: startingDeadlineSeconds still absent (100 missed starts stop the CronJob)")
	}
	if *m.Spec.StartingDeadlineSeconds < 10 {
		return fail("CronDeadline: deadline %ds below the ~10s controller check", *m.Spec.StartingDeadlineSeconds)
	}
	if m.Spec.Suspend != nil && *m.Spec.Suspend {
		return fail("CronDeadline: CronJob still suspended")
	}
	return Result{Pass: true, Reason: "CronDeadline: deadline set, not suspended"}
}

// VerifyCronSuspended gates resume: suspend must be false/absent and a
// deadline present (otherwise resume fires a burst).
func VerifyCronSuspended(doc []byte, _ PodStatus) Result {
	m, ok := decodeCron(doc)
	if !ok {
		return fail("CronSuspended: manifest does not decode as CronJob")
	}
	if m.Spec.Suspend != nil && *m.Spec.Suspend {
		return fail("CronSuspended: still suspended")
	}
	if m.Spec.StartingDeadlineSeconds == nil {
		return fail("CronSuspended: resumed without startingDeadlineSeconds (burst risk)")
	}
	return Result{Pass: true, Reason: "CronSuspended: active with deadline guard"}
}

// VerifyCronOverlap gates the FanOut fix: Forbid or Replace, never Allow.
func VerifyCronOverlap(doc []byte, _ PodStatus) Result {
	m, ok := decodeCron(doc)
	if !ok {
		return fail("CronOverlap: manifest does not decode as CronJob")
	}
	if m.Spec.ConcurrencyPolicy == "" || strings.EqualFold(m.Spec.ConcurrencyPolicy, "Allow") {
		return fail("CronOverlap: concurrencyPolicy still allows overlap")
	}
	return Result{Pass: true, Reason: "CronOverlap: concurrencyPolicy forbids overlap"}
}

// VerifyCronBounds gates backoff/activeDeadline/ttl/history hygiene.
func VerifyCronBounds(doc []byte, _ PodStatus) Result {
	m, ok := decodeCron(doc)
	if !ok {
		return fail("CronBounds: manifest does not decode as CronJob")
	}
	js := m.Spec.JobTemplate.Spec
	if js.BackoffLimit == nil {
		return fail("CronBounds: backoffLimit still absent (Job can hang)")
	}
	if js.ActiveDeadlineSeconds == nil {
		return fail("CronBounds: activeDeadlineSeconds still absent")
	}
	if js.TTLSecondsAfterFinished == nil {
		return fail("CronBounds: ttlSecondsAfterFinished still absent (objects accumulate)")
	}
	if m.Spec.SuccessfulJobsHistoryLimit != nil && *m.Spec.SuccessfulJobsHistoryLimit > 5 {
		return fail("CronBounds: successfulJobsHistoryLimit %d keeps too much history", *m.Spec.SuccessfulJobsHistoryLimit)
	}
	return Result{Pass: true, Reason: "CronBounds: backoff, deadline, ttl and history sane"}
}

// VerifyCronTimezone gates an explicit timeZone (DST-safe schedule).
func VerifyCronTimezone(doc []byte, _ PodStatus) Result {
	m, ok := decodeCron(doc)
	if !ok {
		return fail("CronTimezone: manifest does not decode as CronJob")
	}
	if strings.TrimSpace(m.Spec.TimeZone) == "" {
		return fail("CronTimezone: timeZone still absent (controller zone decides DST)")
	}
	return Result{Pass: true, Reason: "CronTimezone: explicit timeZone"}
}

// VerifyCronHistory gates capped history limits: unbounded Job history
// accumulates objects in etcd. Mirrors the detector threshold.
func VerifyCronHistory(doc []byte, _ PodStatus) Result {
	m, ok := decodeCron(doc)
	if !ok {
		return fail("CronHistory: manifest does not decode as CronJob")
	}
	if m.Spec.SuccessfulJobsHistoryLimit != nil && *m.Spec.SuccessfulJobsHistoryLimit > 5 {
		return fail("CronHistory: successfulJobsHistoryLimit %d keeps too much history", *m.Spec.SuccessfulJobsHistoryLimit)
	}
	if m.Spec.FailedJobsHistoryLimit != nil && *m.Spec.FailedJobsHistoryLimit > 5 {
		return fail("CronHistory: failedJobsHistoryLimit %d keeps too much history", *m.Spec.FailedJobsHistoryLimit)
	}
	return Result{Pass: true, Reason: "CronHistory: history limits capped"}
}

// VerifyCronSilentFail needs runtime: lastSchedule recent but
// lastSuccessfulTime older than two intervals means "runs but never
// succeeds". Without status times it fails closed with the reason.
func VerifyCronSilentFail(doc []byte, _ PodStatus) Result {
	m, ok := decodeCron(doc)
	if !ok {
		return fail("CronSilentFail: manifest does not decode as CronJob")
	}
	if m.Status.LastScheduleTime == "" || m.Status.LastSuccessfulTime == "" {
		return fail("CronSilentFail: no lastScheduleTime/lastSuccessfulTime status yet")
	}
	if m.Status.LastSuccessfulTime < m.Status.LastScheduleTime && len(m.Status.Active) == 0 {
		return fail("CronSilentFail: last success %s older than last schedule %s, nothing active",
			m.Status.LastSuccessfulTime, m.Status.LastScheduleTime)
	}
	return Result{Pass: true, Reason: "CronSilentFail: recent success covers last schedule"}
}
