package store_test

import (
	"testing"

	"github.com/agent-auth/gateway/internal/store"
)

func TestOpen_Migrate(t *testing.T) {
	t.Parallel()

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var version string
	if err := db.SQL.QueryRow("SELECT vec_version()").Scan(&version); err != nil {
		t.Fatalf("vec_version: %v", err)
	}
	if version == "" {
		t.Fatal("empty vec_version")
	}

	var n int
	if err := db.SQL.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'chunk_vec'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("chunk_vec table missing, count=%d", n)
	}
}
