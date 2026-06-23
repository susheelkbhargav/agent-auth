package gen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agent-auth/gateway/internal/retrieve"
)

// OllamaGenerator calls Ollama /api/generate.
type OllamaGenerator struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOllamaGenerator(baseURL, model string) *OllamaGenerator {
	return &OllamaGenerator{
		BaseURL: baseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 180 * time.Second},
	}
}

func (g *OllamaGenerator) Generate(ctx context.Context, query string, chunks []retrieve.Chunk) (string, error) {
	var b strings.Builder
	for _, c := range chunks {
		fmt.Fprintf(&b, "[%s] %s\n", c.ID, c.Text)
	}
	prompt := fmt.Sprintf("Answer the question using only the context below.\n\nContext:\n%s\n\nQuestion: %s\n", b.String(), query)

	body, _ := json.Marshal(map[string]any{
		"model":  g.Model,
		"prompt": prompt,
		"stream": false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama generate: %s", resp.Status)
	}
	var out struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Response), nil
}

// Router picks local vs frontier generator by tier name.
type Router struct {
	Local    Generator
	Frontier Generator
}

func (r *Router) ForTier(local bool) Generator {
	if local {
		return r.Local
	}
	return r.Frontier
}
