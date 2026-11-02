package main

import "testing"

func TestCheckScriptRel(t *testing.T) {
	for _, ok := range []string{
		"scripts/verify/f07_crashloop.sh",
		"scripts/injectors/f01_no_limits.sh",
		"scripts/traps/t4_log_injection.sh",
	} {
		if err := checkScriptRel(ok); err != nil {
			t.Fatalf("legit script rejected: %s: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"",
		"bench/cases.yaml",
		"/etc/passwd.sh",
		"scripts/../../evil.sh",
		"../evil.sh",
		`scripts\verify\f07_crashloop.sh`,
		"scripts/verify/../../../etc/x.sh",
	} {
		if err := checkScriptRel(bad); err == nil {
			t.Fatalf("script accepted: %q", bad)
		}
	}
}
