package stats

import (
	"database/sql"

	"github.com/agent-auth/gateway/internal/meter"
)

// Snapshot is the cumulative dashboard KPI.
type Snapshot struct {
	LeaksBlocked   int     `json:"leaks_blocked"`
	TokensSavedPct float64 `json:"tokens_saved_pct"`
	DollarsSaved   float64 `json:"dollars_saved"`
}

// Add accumulates one request measurement into stats_counters.
func Add(db *sql.DB, m meter.Result) error {
	_, err := db.Exec(`
UPDATE stats_counters SET
  leaks_blocked = leaks_blocked + ?,
  would_be_tokens = would_be_tokens + ?,
  auth_tokens = auth_tokens + ?,
  dollars_saved = dollars_saved + ?
WHERE id = 1`,
		m.LeaksBlocked, m.WouldBeTokens, m.AuthTokens, m.DollarsSaved,
	)
	return err
}

// Read returns cumulative stats for GET /v1/stats.
func Read(db *sql.DB) (Snapshot, error) {
	var s Snapshot
	var wouldBe, auth int
	var dollars float64
	err := db.QueryRow(`SELECT leaks_blocked, would_be_tokens, auth_tokens, dollars_saved FROM stats_counters WHERE id = 1`).
		Scan(&s.LeaksBlocked, &wouldBe, &auth, &dollars)
	if err != nil {
		return s, err
	}
	if dollars < 0 {
		s.DollarsSaved = 0
	} else {
		s.DollarsSaved = dollars
	}
	if wouldBe > 0 {
		s.TokensSavedPct = float64(wouldBe-auth) / float64(wouldBe) * 100
		if s.TokensSavedPct < 0 {
			s.TokensSavedPct = 0
		}
	}
	return s, nil
}
