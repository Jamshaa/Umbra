// Package agent is the tool-calling loop (spec T2).
//
// The model only ever returns data: function calls the loop validates
// and executes against a read-only cluster view (Kube), plus one
// write-shaped tool, propose_patch, that never applies anything and just
// hands the validated proposal to the harness. Unknown tools get an
// error result; invalid arguments get an error result; anything the
// model cannot address (bad call_id) aborts the run. Fail closed,
// always: doubt means "proposed", never "applied".
//
// Tool outputs are sanitized before they reach the model (see
// internal/evidence): logs, events and annotations are untrusted data.
package agent
