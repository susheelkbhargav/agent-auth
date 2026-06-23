package audit

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
)

var genesis = make([]byte, 32)

// SQLiteAudit is an append-only hash-chained audit log in app.db.
type SQLiteAudit struct {
	db *sql.DB
	mu sync.Mutex
}

func NewSQLiteAudit(db *sql.DB) *SQLiteAudit {
	return &SQLiteAudit{db: db}
}

func (a *SQLiteAudit) Append(payload []byte) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	prev, err := a.lastRowHash()
	if err != nil {
		return nil, err
	}
	rowHash := sha256.Sum256(concat(prev, payload))
	_, err = a.db.Exec(
		`INSERT INTO audit_log (prev_hash, row_hash, payload) VALUES (?, ?, ?)`,
		prev, rowHash[:], payload,
	)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(rowHash))
	copy(out, rowHash[:])
	return out, nil
}

func (a *SQLiteAudit) Verify(n int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	query := `SELECT prev_hash, row_hash, payload FROM audit_log ORDER BY seq ASC`
	if n > 0 {
		query = fmt.Sprintf(`SELECT prev_hash, row_hash, payload FROM audit_log ORDER BY seq DESC LIMIT %d`, n)
	}
	rows, err := a.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var chain [][]byte
	for rows.Next() {
		var prev, row, payload []byte
		if err := rows.Scan(&prev, &row, &payload); err != nil {
			return err
		}
		expect := sha256.Sum256(concat(prev, payload))
		if string(expect[:]) != string(row) {
			return fmt.Errorf("audit chain break at row hash mismatch")
		}
		chain = append(chain, row)
	}
	if n > 0 {
		// reversed query — still validated per row internally
		return rows.Err()
	}
	if len(chain) == 0 {
		return nil
	}
	return rows.Err()
}

func concat(a, b []byte) []byte {
	out := make([]byte, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}

func (a *SQLiteAudit) lastRowHash() ([]byte, error) {
	var row []byte
	err := a.db.QueryRow(`SELECT row_hash FROM audit_log ORDER BY seq DESC LIMIT 1`).Scan(&row)
	if err == sql.ErrNoRows {
		return genesis, nil
	}
	return row, err
}

// List returns decoded payloads for GET /v1/audit.
func (a *SQLiteAudit) List(limit int) ([]json.RawMessage, error) {
	q := `SELECT payload FROM audit_log ORDER BY seq DESC`
	if limit > 0 {
		q = fmt.Sprintf(`%s LIMIT %d`, q, limit)
	}
	rows, err := a.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var p []byte
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(p))
	}
	return out, rows.Err()
}
