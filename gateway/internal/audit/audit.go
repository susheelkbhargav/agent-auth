// Package audit is an append-only, hash-chained, tamper-evident log.
// row_hash = sha256(prev_hash || canonical_json(payload)). Verify(n) runs at boot and the
// gateway refuses to start on a mismatch. Logs IDs+labels+counts, never chunk text.
// See ../../DECISION.md (Audit & fail-closed).
package audit

// Appender appends one record per request and can verify the chain.
type Appender interface {
	// Append writes one chained row and returns its row hash. A failure here must cause the
	// request to be denied — no unrecorded access.
	Append(payload []byte) (rowHash []byte, err error)
	// Verify walks the last n rows (or all if n<=0) and returns an error on any break.
	Verify(n int) error
}
