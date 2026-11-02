// Package receipt builds the evidence packet of every change.
//
// Build binds the root cause, the diff, each ladder step result
// (invariants, dry-run, sandbox, rollback), the field attribution
// (who changed what) and the PR URL into one JSON document with its
// SHA256. The append-only chain (AppendRegistro/VerifyChain) links
// registry lines by previous hash so partial edits, reorders and drops
// are detected. The signed variant (AppendRegistroSigned/VerifyChainSigned,
// keyed by UMBRA_RECEIPT_HMAC_KEY) additionally rejects wholesale rewrites
// made without the key: hash links alone cannot do that, since an attacker
// able to rewrite the file could recompute every link.
//
// Timeline audit-readiness is covered upstream: every TimelineEntry in
// Finding status carries tool, model version and the SHA256 of the
// sanitized prompt+result (see internal/controller and internal/evidence).
package receipt
