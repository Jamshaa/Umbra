// Package attribution attributes object fields to the manager that last
// wrote them, using metadata.managedFields only (no cluster access).
//
// The input is the already-decoded object (e.g. an unstructured content map
// unmarshalled from YAML/JSON); only Update/Apply entries are considered and
// the status subresource is ignored.
package attribution
