package retrieve

import (
	"context"
	"fmt"

	"github.com/agent-auth/gateway/internal/labelvocab"
)

// ExpandParentDocs adds sibling chunks that share parent_doc_id with hits, re-gating each
// candidate with required ⊆ eff. Initial ANN hits are preserved; authorized siblings not
// already present are appended. Siblings that fail the label gate are dropped (no aggregation leak).
func (r *SQLiteRetriever) ExpandParentDocs(ctx context.Context, hits []Chunk, eff labelvocab.LabelSet) ([]Chunk, error) {
	if len(hits) == 0 || len(eff) == 0 {
		return hits, nil
	}

	seen := make(map[string]struct{}, len(hits))
	parents := make(map[string]struct{})
	for _, c := range hits {
		seen[c.ID] = struct{}{}
		if c.ParentDocID != "" {
			parents[c.ParentDocID] = struct{}{}
		}
	}
	if len(parents) == 0 {
		return hits, nil
	}

	labelSQL, labelArgs := inClause(eff.Strings())
	out := append([]Chunk(nil), hits...)

	for parentID := range parents {
		siblings, err := r.loadAuthorizedSiblings(ctx, parentID, labelSQL, labelArgs, seen)
		if err != nil {
			return nil, err
		}
		for _, s := range siblings {
			if !eff.Dominates(s.RequiredLabels) {
				continue
			}
			seen[s.ID] = struct{}{}
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *SQLiteRetriever) loadAuthorizedSiblings(
	ctx context.Context,
	parentID, labelSQL string,
	labelArgs []any,
	exclude map[string]struct{},
) ([]Chunk, error) {
	excludeIDs := make([]string, 0, len(exclude))
	for id := range exclude {
		excludeIDs = append(excludeIDs, id)
	}

	query := fmt.Sprintf(`
SELECT c.id, c.text, c.parent_doc_id, c.token_count
FROM chunks c
WHERE c.parent_doc_id = ?
  AND NOT EXISTS (
    SELECT 1 FROM chunk_labels req
    WHERE req.chunk_id = c.id AND req.label NOT IN (%s)
  )`, labelSQL)

	args := append([]any{parentID}, labelArgs...)

	if len(excludeIDs) > 0 {
		exSQL, exArgs := inClause(excludeIDs)
		query += fmt.Sprintf(` AND c.id NOT IN (%s)`, exSQL)
		args = append(args, exArgs...)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load siblings for %q: %w", parentID, err)
	}
	defer rows.Close()

	var chunks []Chunk
	var ids []string
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.ID, &c.Text, &c.ParentDocID, &c.TokenCount); err != nil {
			return nil, err
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

// ExpandParentDocs is the Retriever seam hook used by httpapi after PrefilterTopK.
func ExpandParentDocs(r Retriever, ctx context.Context, hits []Chunk, eff labelvocab.LabelSet) ([]Chunk, error) {
	type expander interface {
		ExpandParentDocs(context.Context, []Chunk, labelvocab.LabelSet) ([]Chunk, error)
	}
	if e, ok := r.(expander); ok {
		return e.ExpandParentDocs(ctx, hits, eff)
	}
	return hits, nil
}
