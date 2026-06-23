// Package acl reads the external ACL store (source of truth for labels, keyed by role/agent).
// The corpus has no native ACL, so labels are an external join. See ../../DECISION.md
// (Ingest & ACL store).
package acl

import "github.com/agent-auth/gateway/internal/labelvocab"

// Store resolves grants and agent scopes from trusted server state.
type Store interface {
	// Grants returns the labels granted to a role (expand via labelvocab.ClearanceFrom).
	Grants(role string) labelvocab.LabelSet
	// AgentScope returns the labels an agent may act within, keyed by its kid.
	AgentScope(kid string) labelvocab.LabelSet
	// IsRevoked reports whether an agent kid has been revoked.
	IsRevoked(kid string) bool
}
