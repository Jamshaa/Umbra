// Package api serves the read-only Umbra UI backend.
//
// GET /api/findings lists Finding CRs mapped to the UI shape the
// App.jsx seam consumes. Every other method is rejected (405): the API
// never writes to the cluster. Static files come from the vite bundle
// directory (ui/dist); the manager binary stays the only writer.
package api
