package store

import (
	"context"
	"sync"
	"time"
)

// DefaultNonceTTL matches DESIGN/DECISION clock-skew window (±30s) with margin.
const DefaultNonceTTL = 30 * time.Second

// MemNonceStore tracks seen jti/nonce values in-process with TTL expiry.
// Swap to Redis/Valkey later by implementing the same NonceStore interface.
type MemNonceStore struct {
	mu   sync.Mutex
	ttl  time.Duration
	seen map[string]time.Time // nonce → expiry
}

// NewMemNonceStore returns an in-memory nonce store. ttl<=0 uses DefaultNonceTTL.
func NewMemNonceStore(ttl time.Duration) *MemNonceStore {
	if ttl <= 0 {
		ttl = DefaultNonceTTL
	}
	return &MemNonceStore{
		ttl:  ttl,
		seen: make(map[string]time.Time),
	}
}

// SeenBefore records nonce and reports whether it was already seen (replay).
func (s *MemNonceStore) SeenBefore(_ context.Context, nonce string) (bool, error) {
	if nonce == "" {
		return true, nil // fail-closed: empty nonce treated as replay
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.purgeLocked(now)

	if exp, ok := s.seen[nonce]; ok && exp.After(now) {
		return true, nil
	}
	s.seen[nonce] = now.Add(s.ttl)
	return false, nil
}

func (s *MemNonceStore) purgeLocked(now time.Time) {
	for n, exp := range s.seen {
		if !exp.After(now) {
			delete(s.seen, n)
		}
	}
}
