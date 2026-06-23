// Package store holds SQLite (app.db: ACL, audit, chunks, vectors) and in-memory
// nonce replay protection. Parameterized queries only. NonceStore is swappable
// (Redis/Valkey later for multi-node). See ../../IMPLEMENTATION.md and ../../DECISION.md.
package store

import "context"

// NonceStore provides single-use nonce/jti tracking for replay protection.
type NonceStore interface {
	// SeenBefore atomically records the nonce and reports whether it was already seen.
	SeenBefore(ctx context.Context, nonce string) (bool, error)
}
