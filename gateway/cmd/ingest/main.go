// Command ingest seeds demo corpus + ACL and embeds chunks via Ollama into app.db.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/agent-auth/gateway/internal/acl"
	"github.com/agent-auth/gateway/internal/embed"
	"github.com/agent-auth/gateway/internal/store"
)

type seedChunk struct {
	ID      string
	Text    string
	Labels  []string
	Tokens  int
	Corpus  string
}

func main() {
	dbPath := flag.String("app-db", "./app.db", "sqlite database path")
	ollama := flag.String("ollama", "http://127.0.0.1:11434", "ollama base url")
	keysDir := flag.String("keys", "./demo/keys", "demo agent pub keys for scope seeding")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := seedACL(db, *keysDir); err != nil {
		log.Fatal(err)
	}
	chunks := demoChunks()
	emb := embed.NewOllamaEmbedder(*ollama, "nomic-embed-text")
	ctx := context.Background()

	for _, c := range chunks {
		if err := insertChunk(db, ctx, emb, c); err != nil {
			log.Fatalf("chunk %s: %v", c.ID, err)
		}
	}
	log.Printf("ingested %d chunks into %s", len(chunks), *dbPath)
}

func seedACL(db *store.DB, keysDir string) error {
	st := acl.NewSQLiteStore(db.SQL)
	roles := map[string][]string{
		"provider": {"phi", "prescription", "lab", "note:provider"},
		"billing":  {"billing", "scheduling"},
	}
	for role, labels := range roles {
		for _, l := range labels {
			if err := st.GrantRole(role, l); err != nil {
				return err
			}
		}
	}
	scopes := map[string][]string{
		"doctor":  {"phi", "prescription", "lab", "note:provider", "scheduling"},
		"billing": {"billing", "scheduling"},
		"patient": {"phi:patient:bob", "scheduling", "billing:patient:bob"},
	}
	for agent, labels := range scopes {
		pubPath := filepath.Join(keysDir, agent+"_pub.raw")
		pub, err := os.ReadFile(pubPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", pubPath, err)
		}
		kid := kidFromPub(pub)
		if err := st.RegisterAgent(kid, labels); err != nil {
			return err
		}
	}
	return nil
}

func insertChunk(db *store.DB, ctx context.Context, emb *embed.OllamaEmbedder, c seedChunk) error {
	vec, err := emb.Embed(ctx, c.Text)
	if err != nil {
		return err
	}
	blob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return err
	}
	tx, err := db.SQL.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR REPLACE INTO chunks (id, text, parent_doc_id, token_count, corpus) VALUES (?, ?, '', ?, ?)`,
		c.ID, c.Text, c.Tokens, c.Corpus); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chunk_labels WHERE chunk_id = ?`, c.ID); err != nil {
		return err
	}
	for _, l := range c.Labels {
		if _, err := tx.Exec(`INSERT INTO chunk_labels (chunk_id, label) VALUES (?, ?)`, c.ID, l); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM chunk_vec WHERE chunk_id = ?`, c.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO chunk_vec (chunk_id, embedding) VALUES (?, ?)`, c.ID, blob); err != nil {
		return err
	}
	return tx.Commit()
}

func demoChunks() []seedChunk {
	return []seedChunk{
		{ID: "lab-a1", Text: "Patient lab results show normal glucose and stable kidney function.", Labels: []string{"lab"}, Tokens: 40, Corpus: "wikidoc"},
		{ID: "lab-a2", Text: "Complete blood count indicates hemoglobin within reference range.", Labels: []string{"lab"}, Tokens: 35, Corpus: "wikidoc"},
		{ID: "phi-dx1", Text: "Diagnosis: Type 2 diabetes with recommended metformin therapy.", Labels: []string{"phi"}, Tokens: 45, Corpus: "synthea"},
		{ID: "phi-dx2", Text: "Assessment notes chronic hypertension requiring medication adjustment.", Labels: []string{"phi"}, Tokens: 42, Corpus: "synthea"},
		{ID: "note-p1", Text: "Provider private note: patient expressed anxiety about treatment plan.", Labels: []string{"note:provider"}, Tokens: 38, Corpus: "synthea"},
		{ID: "bill-b1", Text: "Billing statement for outpatient visit copay and insurance claim status.", Labels: []string{"billing"}, Tokens: 36, Corpus: "wikidoc"},
		{ID: "phi-bob1", Text: "Bob patient chart: follow-up labs scheduled next week.", Labels: []string{"phi:patient:bob"}, Tokens: 30, Corpus: "synthea"},
	}
}

func kidFromPub(pub []byte) string {
	if len(pub) != ed25519.PublicKeySize {
		return hex.EncodeToString(sha256Sum(pub))
	}
	return hex.EncodeToString(sha256Sum(pub))
}

func sha256Sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}
