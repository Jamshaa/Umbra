// Package receipt builds the evidence packet of every change.
//
// Build binds the root cause, the diff, each ladder step result
// (invariants, dry-run, sandbox, rollback), the field attribution
// (who changed what) and the PR URL into one JSON document with its
// SHA256. The append-only chain (AppendRegistro/VerifyChain) links
// registry lines by previous hash so any tampering is detected.
//
// Timeline audit-readiness is covered upstream: every TimelineEntry in
// Finding status carries tool, model version and the SHA256 of the
// sanitized prompt+result (see internal/controller and internal/evidence).
package receipt
