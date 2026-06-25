package stats

import (
	"database/sql"
	"sort"

	"github.com/agent-auth/gateway/internal/meter"
)

// Snapshot is the cumulative dashboard KPI. The single token-weighted TokensSavedPct hides the
// distribution, so the real thesis signals are split out: empty_set_rate (the dominant
// real-world win — queries that hit data the principal can't see, answered at 0 LLM tokens),
// the tier-downgrade saving, and per-request savings percentiles (the mean is misleading when
// one large authorized retrieval dominates the token volume).
type Snapshot struct {
	LeaksBlocked   int     `json:"leaks_blocked"`
	TokensSavedPct float64 `json:"tokens_saved_pct"`
	DollarsSaved   float64 `json:"dollars_saved"`

	TotalRequests        int     `json:"total_requests"`
	EmptySetCount        int     `json:"empty_set_count"`
	EmptySetRate         float64 `json:"empty_set_rate"`
	TierDowngrades       int     `json:"tier_downgrades"`
	TierDowngradeSavings float64 `json:"tier_downgrade_savings"`

	SavingsP50 float64 `json:"savings_p50"`
	SavingsP90 float64 `json:"savings_p90"`
	SavingsP99 float64 `json:"savings_p99"`
}

// Add accumulates one request measurement into stats_counters and records the per-request
// savings sample used for percentiles.
func Add(db *sql.DB, m meter.Result) error {
	empty, downgrade := 0, 0
	if m.EmptySet {
		empty = 1
	}
	if m.DowngradeDollars > 0 {
		downgrade = 1
	}
	if _, err := db.Exec(`
UPDATE stats_counters SET
  leaks_blocked     = leaks_blocked + ?,
  would_be_tokens   = would_be_tokens + ?,
  auth_tokens       = auth_tokens + ?,
  dollars_saved     = dollars_saved + ?,
  total_requests    = total_requests + 1,
  empty_set_count   = empty_set_count + ?,
  tier_downgrades   = tier_downgrades + ?,
  downgrade_dollars = downgrade_dollars + ?
WHERE id = 1`,
		m.LeaksBlocked, m.WouldBeTokens, m.AuthTokens, m.DollarsSaved,
		empty, downgrade, m.DowngradeDollars,
	); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT INTO request_savings (savings_pct) VALUES (?)`, m.SavingsPct)
	return err
}

// Reset zeroes the cumulative counters and clears per-request samples. Called at the start of a
// demo run so each run reports clean, non-accumulated numbers.
func Reset(db *sql.DB) error {
	if _, err := db.Exec(`
UPDATE stats_counters SET
  leaks_blocked = 0, would_be_tokens = 0, auth_tokens = 0, dollars_saved = 0.0,
  total_requests = 0, empty_set_count = 0, tier_downgrades = 0, downgrade_dollars = 0.0
WHERE id = 1`); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM request_savings`)
	return err
}

// Read returns cumulative stats for GET /v1/stats.
func Read(db *sql.DB) (Snapshot, error) {
	var s Snapshot
	var wouldBe, auth int
	var dollars, downgrade float64
	err := db.QueryRow(`
SELECT leaks_blocked, would_be_tokens, auth_tokens, dollars_saved,
       total_requests, empty_set_count, tier_downgrades, downgrade_dollars
FROM stats_counters WHERE id = 1`).
		Scan(&s.LeaksBlocked, &wouldBe, &auth, &dollars,
			&s.TotalRequests, &s.EmptySetCount, &s.TierDowngrades, &downgrade)
	if err != nil {
		return s, err
	}

	if dollars < 0 {
		s.DollarsSaved = 0
	} else {
		s.DollarsSaved = dollars
	}
	if downgrade < 0 {
		s.TierDowngradeSavings = 0
	} else {
		s.TierDowngradeSavings = downgrade
	}
	if wouldBe > 0 {
		s.TokensSavedPct = float64(wouldBe-auth) / float64(wouldBe) * 100
		if s.TokensSavedPct < 0 {
			s.TokensSavedPct = 0
		}
	}
	if s.TotalRequests > 0 {
		s.EmptySetRate = float64(s.EmptySetCount) / float64(s.TotalRequests) * 100
	}

	pct, err := readSavings(db)
	if err != nil {
		return s, err
	}
	s.SavingsP50 = percentile(pct, 50)
	s.SavingsP90 = percentile(pct, 90)
	s.SavingsP99 = percentile(pct, 99)
	return s, nil
}

func readSavings(db *sql.DB) ([]float64, error) {
	rows, err := db.Query(`SELECT savings_pct FROM request_savings ORDER BY savings_pct`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// percentile returns the p-th percentile (0..100) of sorted-or-unsorted samples using
// nearest-rank. Returns 0 for an empty sample set.
func percentile(samples []float64, p float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	rank := int(p / 100 * float64(len(sorted)))
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}
