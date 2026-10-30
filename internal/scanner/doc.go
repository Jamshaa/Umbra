// Package scanner implements Umbra's read-only detector.
//
// It inspects workload manifests (YAML) and reports the faults it knows as
// api/v1alpha1 Finding objects with Detector "scanner". It never touches a
// cluster: the same checks run offline in unit tests and, later, against
// live manifests fetched by the inventory step. Severity and category follow
// docs/PLAN.md; anything the scanner cannot decide statically is left out
// on purpose (no guessing, no AI in this package).
package scanner
