// Package meter measures the thesis: would-be (insecure top-k) vs authorized tokens, plus
// leaks_blocked and dollars saved. Two baselines (B1 insecure, B2 secure-RAG) and the
// sparse/dense regime distinction live here. See ../../DECISION.md (Thesis math).
package meter

import (
	"github.com/agent-auth/gateway/internal/labelvocab"
	"github.com/agent-auth/gateway/internal/retrieve"
	"github.com/agent-auth/gateway/internal/route"
)

// Demo price sheet ($/1k tokens). Frontier tier uses the naive shadow baseline for $ comparison.
const (
	priceLocalPer1K    = 0.0  // Ollama phi4-mini — local, no API cost
	priceFrontierPer1K = 0.015
)

// Result is the per-request measurement. leaks_blocked counts forbidden chunks the insecure
// top-k WOULD have surfaced for this exact query.
type Result struct {
	WouldBeTokens int
	AuthTokens    int
	LeaksBlocked  int
	SavingsPct    float64
	DollarsSaved  float64
}

// Compute fills Result from shadow (B1 naive top-k metadata) vs authorized chunks.
// tier is the egress tier actually used; tierNaive is the tier shadow would have forced.
func Compute(shadow []retrieve.ChunkMeta, auth []retrieve.Chunk, eff labelvocab.LabelSet, tier, tierNaive route.Tier) Result {
	var wouldBe, authTok int
	for _, c := range shadow {
		wouldBe += c.TokenCount
	}
	for _, c := range auth {
		authTok += c.TokenCount
	}
	leaks := 0
	for _, c := range shadow {
		if !eff.Dominates(c.RequiredLabels) {
			leaks++
		}
	}
	var savings float64
	if wouldBe > 0 {
		savings = float64(wouldBe-authTok) / float64(wouldBe) * 100
	}
	dollars := pricePer1K(tierNaive)*float64(wouldBe)/1000 - pricePer1K(tier)*float64(authTok)/1000
	return Result{
		WouldBeTokens: wouldBe,
		AuthTokens:    authTok,
		LeaksBlocked:  leaks,
		SavingsPct:    savings,
		DollarsSaved:  dollars,
	}
}

// TierForShadow picks the egress tier the naive baseline would have used (meter only).
func TierForShadow(shadow []retrieve.ChunkMeta) route.Tier {
	if len(shadow) == 0 {
		return route.Refuse
	}
	labels := make([]labelvocab.LabelSet, len(shadow))
	for i, c := range shadow {
		labels[i] = c.RequiredLabels
	}
	return route.Decide(labels)
}

func pricePer1K(t route.Tier) float64 {
	switch t {
	case route.Frontier:
		return priceFrontierPer1K
	default:
		return priceLocalPer1K
	}
}
