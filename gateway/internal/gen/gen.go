package gen

import (
	"context"

	"github.com/agent-auth/gateway/internal/retrieve"
)

// Generator produces an answer from authorized chunks only.
type Generator interface {
	Generate(ctx context.Context, query string, chunks []retrieve.Chunk) (string, error)
}
