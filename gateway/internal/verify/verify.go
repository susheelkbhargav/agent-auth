// Package verify performs the cryptographic identity checks, fail-closed and ordered:
// DPoP proof (htm/htu/ath/jti + body hash) → nonce/clock → OBO. Any failure returns ErrDenied
// (a single error type, so the caller cannot branch on the reason). See ../../DECISION.md
// (Identity boundary).
package verify

import (
	"context"
	"errors"
)

// ErrDenied is the single, reason-opaque verification failure.
var ErrDenied = errors.New("verification failed")

// Request carries everything needed to verify one call.
type Request struct {
	Body      []byte
	OBO       string // signed JWT, act = sha256(agent pubkey)
	Sig       string // DPoP-aligned proof over htm|htu|ath|jti|iat + sha256(body)
	Nonce     string
	Timestamp string
	Method    string // htm
	URI       string // htu
}

// Principal is the verified result. All authorization inputs originate here, never from
// agent JSON or model output.
type Principal struct {
	UserID    string
	UserRoles []string
	AgentKID  string   // SPIFFE-style id = sha256(agent pubkey)
	TaskScope []string // already capped by the OBO
}

// Verifier verifies a request or returns ErrDenied.
type Verifier interface {
	Verify(ctx context.Context, r *Request) (*Principal, error)
}
