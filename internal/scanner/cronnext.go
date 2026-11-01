package scanner

import (
	"strconv"
	"strings"
	"time"
)

// nextRuns returns up to n Unix times strictly after fromUnix matching a
// 5-field cron schedule (minute hour dom month dow). Only "*", "*/k" and
// single numbers are expanded for minute/hour and "*" for the day fields;
// anything else yields nil (unknown, never healthy). Minute-step scan is
// capped at ~366 days so a degenerate schedule cannot hang Scan.
func nextRuns(schedule string, fromUnix int64, n int) []int64 {
	if n < 1 {
		n = 8
	}
	if n > 32 {
		n = 32
	}
	f := strings.Fields(schedule)
	if len(f) != 5 {
		return nil
	}
	mins, ok1 := expandStep(f[0], 0, 59)
	hours, ok2 := expandStep(f[1], 0, 23)
	if !ok1 || !ok2 || !simpleField(f[2]) || !simpleField(f[3]) || !simpleField(f[4]) {
		return nil
	}
	minSet := map[int]bool{}
	for _, m := range mins {
		minSet[m] = true
	}
	hourSet := map[int]bool{}
	for _, h := range hours {
		hourSet[h] = true
	}
	t := time.Unix(fromUnix, 0).UTC().Truncate(time.Minute).Add(time.Minute)
	var out []int64
	for step := 0; step < 366*24*60 && len(out) < n; step++ {
		if minSet[t.Minute()] && hourSet[t.Hour()] {
			out = append(out, t.Unix())
		}
		t = t.Add(time.Minute)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// NextCronRuns is the exported calendar helper for the UI timeline.
func NextCronRuns(schedule, _ string, fromUnix int64, n int) []int64 {
	return nextRuns(schedule, fromUnix, n)
}

func expandStep(field string, lo, hi int) ([]int, bool) {
	if field == "*" {
		out := make([]int, 0, hi-lo+1)
		for i := lo; i <= hi; i++ {
			out = append(out, i)
		}
		return out, true
	}
	if strings.HasPrefix(field, "*/") {
		k, err := strconv.Atoi(strings.TrimPrefix(field, "*/"))
		if err != nil || k < 1 {
			return nil, false
		}
		var out []int
		for i := lo; i <= hi; i += k {
			out = append(out, i)
		}
		return out, true
	}
	if v, err := strconv.Atoi(field); err == nil && v >= lo && v <= hi {
		return []int{v}, true
	}
	return nil, false
}

func simpleField(f string) bool { return f == "*" }
