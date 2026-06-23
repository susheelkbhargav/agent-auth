// Package resolve is the security heart: it computes the effective principal as a
// PURE function of cryptographically-verified inputs. The LLM never participates.
// See ../../DECISION.md (Authorization core).
package resolve

import "github.com/agent-auth/gateway/internal/labelvocab"

// Effective computes effective = grants ⊓ agentScope ⊓ (reqTask ⊓ oboTask).
//
// Every operand is bounded above by a verified credential, and meet is monotone, so no
// adversary input can grow the result — only shrink it or no-op. Worst case is the empty
// set (0 candidates → 0 tokens). The capping of reqTask by oboTask is the load-bearing step.
//
// grants should already be expanded via labelvocab.ClearanceFrom so chunk dominance checks
// (required ⊆ effective) respect the confidentiality chain.
func Effective(grants, agentScope, reqTask, oboTask labelvocab.LabelSet) labelvocab.LabelSet {
	taskCapped := reqTask.Meet(oboTask)
	return grants.Meet(agentScope).Meet(taskCapped)
}
