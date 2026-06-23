// Command vecwrite bulk-inserts chunk embeddings into chunk_vec (sqlite-vec).
// Python ingestlib writes chunks + labels; this helper owns vec0 INSERTs.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"os"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/agent-auth/gateway/internal/store"
)

type vecRecord struct {
	ChunkID   string    `json:"chunk_id"`
	Embedding []float32 `json:"embedding"`
}

func main() {
	dbPath := flag.String("app-db", "./app.db", "sqlite database path")
	inPath := flag.String("in", "", "NDJSON file: {chunk_id, embedding:[768 floats]}")
	clear := flag.Bool("clear", true, "delete existing chunk_vec rows before insert")
	flag.Parse()
	if *inPath == "" {
		log.Fatal("-in required")
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if *clear {
		if _, err := db.SQL.Exec(`DELETE FROM chunk_vec`); err != nil {
			log.Fatalf("clear chunk_vec: %v", err)
		}
	}

	f, err := os.Open(*inPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	tx, err := db.SQL.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var rec vecRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			log.Fatalf("line %d: %v", n+1, err)
		}
		if rec.ChunkID == "" || len(rec.Embedding) == 0 {
			log.Fatalf("line %d: missing chunk_id or embedding", n+1)
		}
		blob, err := sqlite_vec.SerializeFloat32(rec.Embedding)
		if err != nil {
			log.Fatalf("serialize %s: %v", rec.ChunkID, err)
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO chunk_vec (chunk_id, embedding) VALUES (?, ?)`,
			rec.ChunkID, blob); err != nil {
			log.Fatalf("insert %s: %v", rec.ChunkID, err)
		}
		n++
	}
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d vectors to %s", n, *dbPath)
}
