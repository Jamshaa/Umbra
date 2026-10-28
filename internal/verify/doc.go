// Package verify implements Umbra's deterministic fix verification.
//
// Each VerifyFxx function encodes one row of the PLAN.md scoreboard gate
// ("Correct fix"): given the (possibly patched) manifest plus an observed
// pod status, it answers pass/fail with a reason. No cluster client, no AI,
// no guessing — the same code runs in unit tests and, later, in the harness
// fed with live data.
//
// Self-contained decoding: the manifest subset decoded here mirrors
// internal/scanner's subset on purpose, so this package has no dependency
// on sibling branches. Once merged, both packages may share one decoder;
// until then, duplication beats coupling.
package verify
