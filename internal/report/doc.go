// Package report emits PolicyReport documents.
//
// Build renders results as wgpolicyk8s.io/v1alpha2 YAML with a
// deterministic metadata.name (scope+template) so writes are
// append/update idempotent. It only generates the document: no cluster
// client, no network.
package report
