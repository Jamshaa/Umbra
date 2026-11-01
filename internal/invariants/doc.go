// Package invariants is the validation pipeline (spec T3): one fixed
// order of gates every patch must pass, deterministic fixes and model
// proposals alike.
//
//  1. allowlist: the patch may only touch the fields its rule authorizes.
//  2. invariants: same object (kind/namespace/name, selector), size cap,
//     no secret values, no RBAC widening, no resources below observed use.
//  3. dry-run: the API server admits the result (injected, faked in tests).
//  4. sandbox: apply in umbra-sandbox only, get a snapshot id back.
//  5. verifier: the rule's deterministic check, with a timeout.
//
// Any failure stops the pipeline and leaves the finding "proposed" with
// the reason. There is no partial apply and no auto-apply: the pipeline
// only ever returns a verdict.
package invariants
