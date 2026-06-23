package retrieve_test

import (
	"context"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/agent-auth/gateway/internal/labelvocab"
	"github.com/agent-auth/gateway/internal/retrieve"
	"github.com/agent-auth/gateway/internal/store"
)

func TestSQLiteRetriever_PrefilterTopK(t *testing.T) {
	t.Parallel()

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	seedChunk(t, db, "lab-a", unitVec(0, 1.0), 100, "lab")
	seedChunk(t, db, "phi-b", unitVec(1, 1.0), 200, "phi")
	seedChunk(t, db, "lab-c", unitVec(0, 0.9), 150, "lab")

	ret := retrieve.NewSQLiteRetriever(db.SQL)
	ctx := context.Background()
	query := unitVec(0, 1.0)

	got, err := ret.PrefilterTopK(ctx, query, labelvocab.New("lab"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d chunks, want 2 authorized lab chunks", len(got))
	}
	if got[0].ID != "lab-a" {
		t.Fatalf("first chunk %q, want lab-a (closest to query)", got[0].ID)
	}
	for _, c := range got {
		if !labelvocab.New("lab").Dominates(c.RequiredLabels) {
			t.Fatalf("chunk %q labels %v not dominated by lab", c.ID, c.RequiredLabels)
		}
	}

	empty, err := ret.PrefilterTopK(ctx, query, labelvocab.New("billing"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("billing eff got %d chunks, want 0", len(empty))
	}

	none, err := ret.PrefilterTopK(ctx, query, labelvocab.LabelSet{}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if none != nil {
		t.Fatalf("empty eff got %#v, want nil", none)
	}
}

func TestSQLiteRetriever_ExpandParentDocs_ReGate(t *testing.T) {
	t.Parallel()

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	seedChunkWithParent(t, db, "lab-a", "doc-1", unitVec(0, 1.0), 100, "lab")
	seedChunkWithParent(t, db, "lab-b", "doc-1", unitVec(0, 0.8), 90, "lab")
	seedChunkWithParent(t, db, "phi-x", "doc-1", unitVec(1, 1.0), 200, "phi")

	ret := retrieve.NewSQLiteRetriever(db.SQL)
	ctx := context.Background()
	hits := []retrieve.Chunk{
		{ID: "lab-a", ParentDocID: "doc-1", TokenCount: 100, RequiredLabels: labelvocab.New("lab")},
	}

	expanded, err := ret.ExpandParentDocs(ctx, hits, labelvocab.New("lab"))
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 2 {
		t.Fatalf("lab eff got %d chunks, want 2 (lab-a + lab-b)", len(expanded))
	}

	expandedAll, err := ret.ExpandParentDocs(ctx, hits, labelvocab.New("lab", "phi"))
	if err != nil {
		t.Fatal(err)
	}
	if len(expandedAll) != 3 {
		t.Fatalf("lab+phi eff got %d chunks, want 3 siblings", len(expandedAll))
	}
}

func TestSQLiteRetriever_ShadowTopK(t *testing.T) {
	t.Parallel()

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	seedChunk(t, db, "lab-a", unitVec(0, 1.0), 100, "lab")
	seedChunk(t, db, "phi-b", unitVec(1, 1.0), 200, "phi")

	ret := retrieve.NewSQLiteRetriever(db.SQL)
	metas, err := ret.ShadowTopK(context.Background(), unitVec(0, 1.0), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("got %d metas, want 2", len(metas))
	}
	if metas[0].ID != "lab-a" {
		t.Fatalf("first meta %q, want lab-a", metas[0].ID)
	}
}

func seedChunk(t *testing.T, db *store.DB, id string, emb []float32, tokens int, labels ...string) {
	seedChunkWithParent(t, db, id, "", emb, tokens, labels...)
}

func seedChunkWithParent(t *testing.T, db *store.DB, id, parent string, emb []float32, tokens int, labels ...string) {
	t.Helper()
	blob, err := sqlite_vec.SerializeFloat32(emb)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.Exec(
		`INSERT INTO chunks (id, text, parent_doc_id, token_count, corpus) VALUES (?, ?, ?, ?, 'test')`,
		id, "text-"+id, parent, tokens,
	); err != nil {
		t.Fatal(err)
	}
	for _, l := range labels {
		if _, err := db.SQL.Exec(`INSERT INTO chunk_labels (chunk_id, label) VALUES (?, ?)`, id, l); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.SQL.Exec(`INSERT INTO chunk_vec (chunk_id, embedding) VALUES (?, ?)`, id, blob); err != nil {
		t.Fatal(err)
	}
}

func unitVec(dim int, mag float32) []float32 {
	v := make([]float32, 768)
	if dim < 0 || dim >= len(v) {
		panic("unitVec dim out of range")
	}
	v[dim] = mag
	return v
}
