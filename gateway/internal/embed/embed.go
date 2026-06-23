// Package embed defines the Embedder port. The query is embedded at request time (the only
// embedding on the hot path); ingest embedding is offline. Hide the model choice behind this
// interface so it is swappable (Ollama nomic-embed-text / MiniLM). See ../../DECISION.md.
package embed

import "context"

// Embedder turns text into a vector. The query-time embedder MUST use the same model the
// corpus was embedded with.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
