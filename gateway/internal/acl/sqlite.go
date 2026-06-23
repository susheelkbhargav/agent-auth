package acl

import (
	"database/sql"

	"github.com/agent-auth/gateway/internal/labelvocab"
)

// SQLiteStore reads ACL rows from app.db.
type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Grants(role string) labelvocab.LabelSet {
	rows, err := s.db.Query(`SELECT label FROM role_grants WHERE role = ?`, role)
	if err != nil {
		return labelvocab.New()
	}
	defer rows.Close()
	return scanLabels(rows)
}

func (s *SQLiteStore) AgentScope(kid string) labelvocab.LabelSet {
	rows, err := s.db.Query(`SELECT label FROM agent_scope WHERE kid = ?`, kid)
	if err != nil {
		return labelvocab.New()
	}
	defer rows.Close()
	return scanLabels(rows)
}

func (s *SQLiteStore) IsRevoked(kid string) bool {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM revoked_agents WHERE kid = ?`, kid).Scan(&n)
	return err == nil && n > 0
}

// RegisterAgent inserts scope labels for kid (admin).
func (s *SQLiteStore) RegisterAgent(kid string, labels []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM agent_scope WHERE kid = ?`, kid); err != nil {
		return err
	}
	for _, l := range labels {
		if _, err := tx.Exec(`INSERT INTO agent_scope (kid, label) VALUES (?, ?)`, kid, l); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GrantRole adds a label to a role (ingest/admin).
func (s *SQLiteStore) GrantRole(role, label string) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO role_grants (role, label) VALUES (?, ?)`, role, label)
	return err
}

func scanLabels(rows *sql.Rows) labelvocab.LabelSet {
	set := labelvocab.New()
	for rows.Next() {
		var l string
		if rows.Scan(&l) == nil {
			set[labelvocab.Label(l)] = struct{}{}
		}
	}
	return set
}

// GrantsUnion merges ClearanceFrom(role grants) across roles.
func GrantsUnion(st Store, roles []string) labelvocab.LabelSet {
	out := labelvocab.New()
	for _, role := range roles {
		out = out.Union(labelvocab.ClearanceFrom(st.Grants(role)))
	}
	return out
}

func labelsFromStrings(ss []string) labelvocab.LabelSet {
	set := labelvocab.New()
	for _, s := range ss {
		set[labelvocab.Label(s)] = struct{}{}
	}
	return set
}

// LabelsFromStrings exports label set construction for httpapi.
func LabelsFromStrings(ss []string) labelvocab.LabelSet {
	return labelsFromStrings(ss)
}
