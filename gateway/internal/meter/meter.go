// Package meter measures the thesis: would-be (insecure top-k) vs authorized tokens, plus
// leaks_blocked and dollars saved. Two baselines (B1 insecure, B2 secure-RAG) and the
// sparse/dense regime distinction live here. See ../../DECISION.md (Thesis math).
package meter

// Result is the per-request measurement. leaks_blocked counts forbidden chunks the insecure
// top-k WOULD have surfaced for this exact query.
type Result struct {
	WouldBeTokens int
	AuthTokens    int
	LeaksBlocked  int
	SavingsPct    float64
	DollarsSaved  float64
}
