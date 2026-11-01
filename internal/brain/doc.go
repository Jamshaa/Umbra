// Package brain is the model client (spec T1).
//
// Transport-only HTTP against Meta's Responses API, which keeps reasoning
// across tool calls. The client sends read-only context plus tool
// definitions, parses function_call turns, retries transient failures
// with backoff, and records per-call usage (tokens, latency, response
// id) for the receipt. It never writes to the cluster, never executes
// anything, and never logs credentials: the key travels only as an
// Authorization header and never appears in errors.
//
// Budgets (max steps, max tokens) and the finding-hash cache live here
// as knobs; the tool loop in T2 enforces them.
package brain
