package fixes

import (
	"regexp"
)

// CronJob deterministic patches: small scalar fields with safe defaults.
// Anything needing cluster truth (selectors, RBAC, probes) stays with the
// model; Fix returns false there.

var (
	cronDeadlineRe = regexp.MustCompile(`(?m)^([ \t]*)startingDeadlineSeconds:[ \t]*\d+[ \t]*\r?$`)
	cronSuspendRe  = regexp.MustCompile(`(?m)^([ \t]*)suspend:[ \t]*true[ \t]*\r?$`)
	cronPolicyRe   = regexp.MustCompile(`(?m)^([ \t]*)concurrencyPolicy:[ \t]*"?\w+"?[ \t]*\r?$`)
	cronTZRe       = regexp.MustCompile(`(?m)^([ \t]*)timeZone:[ \t]*"?[^"\n]*"?[ \t]*\r?$`)
	cronBackoffRe  = regexp.MustCompile(`(?m)^([ \t]*)backoffLimit:[ \t]*\d+[ \t]*\r?$`)
	cronActiveRe   = regexp.MustCompile(`(?m)^([ \t]*)activeDeadlineSeconds:[ \t]*\d+[ \t]*\r?$`)
	cronTTLRe      = regexp.MustCompile(`(?m)^([ \t]*)ttlSecondsAfterFinished:[ \t]*\d+[ \t]*\r?$`)
	cronSchedRe    = regexp.MustCompile(`(?m)^([ \t]*)schedule:[ \t]*.*\r?$`)
)

// indentOf returns the indent of the first match of re.
func indentOf(s string, re *regexp.Regexp) (string, bool) {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// setOrAddField replaces the line matched by re, or inserts
// "indent+line" right after the schedule line when absent.
func setOrAddField(s, line string, re *regexp.Regexp) (string, bool) {
	if re.MatchString(s) {
		ind, _ := indentOf(s, re)
		return re.ReplaceAllString(s, ind+line), true
	}
	loc := cronSchedRe.FindStringIndex(s)
	if loc == nil {
		return s, false
	}
	// schedule indent == spec indent; fields live at the same level.
	m := cronSchedRe.FindStringSubmatch(s)
	ind := ""
	if m != nil {
		ind = m[1]
	}
	return s[:loc[1]] + "\n" + ind + line + s[loc[1]:], true
}

// fixCronDeadline sets startingDeadlineSeconds: 60 (above the ~10s check).
func fixCronDeadline(doc []byte) ([]byte, bool) {
	out, ok := setOrAddField(string(doc), "startingDeadlineSeconds: 60", cronDeadlineRe)
	if !ok {
		return doc, false
	}
	return []byte(out), true
}

// fixCronSuspended flips suspend:true to false (deadline enforced by gate).
func fixCronSuspended(doc []byte) ([]byte, bool) {
	s := string(doc)
	if !cronSuspendRe.MatchString(s) {
		return doc, false
	}
	ind, _ := indentOf(s, cronSuspendRe)
	return []byte(cronSuspendRe.ReplaceAllString(s, ind+"suspend: false")), true
}

// fixCronOverlap sets concurrencyPolicy: Forbid.
func fixCronOverlap(doc []byte) ([]byte, bool) {
	out, ok := setOrAddField(string(doc), "concurrencyPolicy: Forbid", cronPolicyRe)
	if !ok {
		return doc, false
	}
	return []byte(out), true
}

// fixCronTimezone sets timeZone: UTC (explicit beats controller default).
func fixCronTimezone(doc []byte) ([]byte, bool) {
	out, ok := setOrAddField(string(doc), "timeZone: UTC", cronTZRe)
	if !ok {
		return doc, false
	}
	return []byte(out), true
}

var cronJobSpecRe = regexp.MustCompile(`(?m)^([ \t]*)jobTemplate:[ \t]*\r?\n(?:.*\n)?([ \t]*)spec:[ \t]*\r?$`)

// setJobField replaces the line matched by re, or inserts "line" under the
// jobTemplate.spec block (one level deeper than its "spec:").
func setJobField(s, line string, re *regexp.Regexp) (string, bool) {
	if re.MatchString(s) {
		ind, _ := indentOf(s, re)
		return re.ReplaceAllString(s, ind+line), true
	}
	m := cronJobSpecRe.FindStringSubmatch(s)
	if m == nil {
		return s, false
	}
	ind := m[2] + "  "
	loc := cronJobSpecRe.FindStringIndex(s)
	return s[:loc[1]] + "\n" + ind + line + s[loc[1]:], true
}

// fixCronBounds sets backoffLimit/activeDeadline/ttl when absent or insane.
func fixCronBounds(doc []byte) ([]byte, bool) {
	s := string(doc)
	changed := false
	if !cronBackoffRe.MatchString(s) {
		out, ok := setJobField(s, "backoffLimit: 6", cronBackoffRe)
		if !ok {
			return doc, false
		}
		s, changed = out, true
	}
	if !cronActiveRe.MatchString(s) {
		out, ok := setJobField(s, "activeDeadlineSeconds: 600", cronActiveRe)
		if !ok {
			return doc, false
		}
		s, changed = out, true
	}
	if !cronTTLRe.MatchString(s) {
		out, ok := setJobField(s, "ttlSecondsAfterFinished: 100", cronTTLRe)
		if !ok {
			return doc, false
		}
		s, changed = out, true
	}
	if !changed {
		return doc, false
	}
	return []byte(s), true
}

// fixCronHistory caps successfulJobsHistoryLimit at 3.
func fixCronHistory(doc []byte) ([]byte, bool) {
	re := regexp.MustCompile(`(?m)^([ \t]*)successfulJobsHistoryLimit:[ \t]*\d+[ \t]*\r?$`)
	s := string(doc)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return doc, false
	}
	return []byte(re.ReplaceAllString(s, m[1]+"successfulJobsHistoryLimit: 3")), true
}
