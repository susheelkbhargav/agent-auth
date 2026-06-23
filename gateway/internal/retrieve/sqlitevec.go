package retrieve

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/agent-auth/gateway/internal/labelvocab"
)

const embeddingDim = 768

// SQLiteRetriever implements Retriever with sqlite-vec vec0 and normalized chunk_labels.
type SQLiteRetriever struct {
	db *sql.DB
}

// NewSQLiteRetriever returns a retriever backed by an migrated app.db handle.
func NewSQLiteRetriever(db *sql.DB) *SQLiteRetriever {
	return &SQLiteRetriever{db: db}
}

// PrefilterTopK returns cosine top-k over chunks whose required labels ⊆ eff.
// Label filter runs in SQL before vec0 MATCH scoring (engine-level pre-filter).
func (r *SQLiteRetriever) PrefilterTopK(ctx context.Context, q []float32, eff labelvocab.LabelSet, k int) ([]Chunk, error) {
	if err := validateQuery(q); err != nil {
		return nil, err
	}
	if k <= 0 || len(eff) == 0 {
		return nil, nil
	}

	qBlob, err := sqlite_vec.SerializeFloat32(q)
	if err != nil {
		return nil, fmt.Errorf("serialize query: %w", err)
	}

	labelSQL, labelArgs := inClause(eff.Strings())
	query := fmt.Sprintf(`
SELECT c.id, c.text, c.parent_doc_id, c.token_count, v.distance
FROM chunk_vec v
JOIN chunks c ON c.id = v.chunk_id
WHERE v.embedding MATCH ?
  AND k = ?
  AND NOT EXISTS (
    SELECT 1 FROM chunk_labels req
    WHERE req.chunk_id = c.id AND req.label NOT IN (%s)
  )
ORDER BY v.distance`, labelSQL)

	args := append([]any{qBlob, k}, labelArgs...)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("prefilter top-k: %w", err)
	}
	defer rows.Close()

	var chunks []Chunk
	var ids []string
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.ID, &c.Text, &c.ParentDocID, &c.TokenCount, &c.Score); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		chunks = append(chunks, c)
		ids = append(ids, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	labelsByID, err := r.loadLabels(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range chunks {
		chunks[i].RequiredLabels = labelsByID[chunks[i].ID]
	}
	return chunks, nil
}

// ShadowTopK is the B1 naive baseline: unfiltered top-k, metadata only (no text).
func (r *SQLiteRetriever) ShadowTopK(ctx context.Context, q []float32, k int) ([]ChunkMeta, error) {
	if err := validateQuery(q); err != nil {
		return nil, err
	}
	if k <= 0 {
		return nil, nil
	}

	qBlob, err := sqlite_vec.SerializeFloat32(q)
	if err != nil {
		return nil, fmt.Errorf("serialize query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT c.id, c.token_count
FROM chunk_vec v
JOIN chunks c ON c.id = v.chunk_id
WHERE v.embedding MATCH ?
  AND k = ?
ORDER BY v.distance`, qBlob, k)
	if err != nil {
		return nil, fmt.Errorf("shadow top-k: %w", err)
	}
	defer rows.Close()

	var metas []ChunkMeta
	var ids []string
	for rows.Next() {
		var m ChunkMeta
		if err := rows.Scan(&m.ID, &m.TokenCount); err != nil {
			return nil, fmt.Errorf("scan meta: %w", err)
		}
		metas = append(metas, m)
		ids = append(ids, m.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	labelsByID, err := r.loadLabels(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range metas {
		metas[i].RequiredLabels = labelsByID[metas[i].ID]
	}
	return metas, nil
}

func (r *SQLiteRetriever) loadLabels(ctx context.Context, ids []string) (map[string]labelvocab.LabelSet, error) {
	if len(ids) == 0 {
		return map[string]labelvocab.LabelSet{}, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf(`SELECT chunk_id, label FROM chunk_labels WHERE chunk_id IN (%s)`, placeholders)

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load labels: %w", err)
	}
	defer rows.Close()

	out := make(map[string]labelvocab.LabelSet, len(ids))
	for rows.Next() {
		var id, label string
		if err := rows.Scan(&id, &label); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = labelvocab.New()
		}
		out[id][labelvocab.Label(label)] = struct{}{}
	}
	return out, rows.Err()
}

func validateQuery(q []float32) error {
	if len(q) != embeddingDim {
		return fmt.Errorf("query embedding dim %d, want %d", len(q), embeddingDim)
	}
	return nil
}

func inClause(values []string) (string, []any) {
	if len(values) == 0 {
		return "NULL", nil
	}
	parts := make([]string, len(values))
	args := make([]any, len(values))
	for i, v := range values {
		parts[i] = "?"
		args[i] = v
	}
	return strings.Join(parts, ","), args
}
