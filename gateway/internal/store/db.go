package store

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

//go:embed schema.sql
var schemaFS embed.FS

// DB wraps the shared app.db used by gateway and ingest.
type DB struct {
	SQL *sql.DB
}

// Open opens path (or ":memory:"), enables foreign keys, and runs migrations.
func Open(path string) (*DB, error) {
	sqlite_vec.Auto()
	dsn := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		dsn = "file:" + path
	}
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Single connection: sqlite-vec + temp tables for label prefilter.
	sqlDB.SetMaxOpenConns(1)

	db := &DB{SQL: sqlDB}
	if err := db.Migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Migrate applies embedded schema.sql statements in order.
func (db *DB) Migrate() error {
	if _, err := db.SQL.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("foreign_keys: %w", err)
	}
	raw, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	for i, stmt := range splitStatements(string(raw)) {
		if _, err := db.SQL.Exec(stmt); err != nil {
			// ADD COLUMN migrations are not idempotent in SQLite; on an already-upgraded DB
			// they fail with "duplicate column name". Treat that as a no-op.
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("migrate stmt %d: %w", i+1, err)
		}
	}
	return nil
}

// Close closes the underlying database.
func (db *DB) Close() error {
	return db.SQL.Close()
}

func splitStatements(sqlText string) []string {
	var lines []string
	for _, line := range strings.Split(sqlText, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "--") {
			continue
		}
		lines = append(lines, line)
	}
	body := strings.Join(lines, "\n")
	parts := strings.Split(body, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
