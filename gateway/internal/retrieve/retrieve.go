// Package retrieve defines the Retriever port. Implementations pre-filter by the effective
// label set BEFORE ANN scoring (engine-level, never post-filter). The Retriever receives ONLY
// the effective labels — never identity, intent, or model output — so it cannot be injection
// steered. See ../../DECISION.md (Retrieval & token meter).
package retrieve

import (
	"context"

	"github.com/agent-auth/gateway/internal/labelvocab"
)

// Chunk is an authorized retrieved chunk returned to the caller.
type Chunk struct {
	ID             string
	Text           string
	ParentDocID    string
	TokenCount     int // precomputed at ingest with the target tokenizer
	RequiredLabels labelvocab.LabelSet
	Score          float64
}

// ChunkMeta is metadata-only (no text) used to compute the would-be baseline + leaks_blocked.
type ChunkMeta struct {
	ID             string
	TokenCount     int
	RequiredLabels labelvocab.LabelSet
}

// Retriever is the vector-store seam. Chroma is one impl; swap to Qdrant/pgvector later.
type Retriever interface {
	// PrefilterTopK filters required ⊆ eff BEFORE ANN, then returns cosine top-k survivors.
	PrefilterTopK(ctx context.Context, q []float32, eff labelvocab.LabelSet, k int) ([]Chunk, error)

	// ShadowTopK is meter-only: unfiltered top-k, returns metadata (never text).
	ShadowTopK(ctx context.Context, q []float32, k int) ([]ChunkMeta, error)
}
