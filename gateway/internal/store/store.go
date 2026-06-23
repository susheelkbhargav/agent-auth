// Package store holds the storage backends: SQLite (ACL + audit) and Redis/Valkey
// (nonce/session/ratelimit). Parameterized queries only. Distributed-ready, single-node local.
// See ../../DECISION.md (Stack).
package store

import "context"

// NonceStore provides single-use nonce/jti tracking for replay protection.
type NonceStore interface {
	// SeenBefore atomically records the nonce and reports whether it was already seen.
	SeenBefore(ctx context.Context, nonce string) (bool, error)
}
