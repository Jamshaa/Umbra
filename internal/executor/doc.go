// Package executor opens the GitOps exit: a PR with the patched files,
// the receipt and the verification results. Without GH_TOKEN it returns
// the plan (branch + diff) without any network access.
package executor
