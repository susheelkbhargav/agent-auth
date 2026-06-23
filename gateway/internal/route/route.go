// Package route is a sensitivity-gated data-egress control (NOT load balancing): the same
// label that authorizes also decides which model may legally process the data.
// See ../../DECISION.md (Routing).
package route

import "github.com/agent-auth/gateway/internal/labelvocab"

// Tier is the egress decision for an authorized chunk set.
type Tier int

const (
	Refuse   Tier = iota // empty authorized set → 0 tokens
	Local                // any phi/restricted label → local model only (BAA boundary)
	Frontier             // all de-identified/public → frontier permitted
)

// Restricted labels force local-only processing. Extend as the vocabulary grows.
var restricted = labelvocab.New("phi", "restricted")

// Decide returns the egress tier for the authorized chunk labels. Fail-closed: unknown
// sensitivity is treated as restricted by the caller before reaching here.
func Decide(authorized []labelvocab.LabelSet) Tier {
	if len(authorized) == 0 {
		return Refuse
	}
	for _, chunk := range authorized {
		for l := range chunk {
			if restricted.Contains(l) {
				return Local
			}
		}
	}
	return Frontier
}
