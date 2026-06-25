-- app.db schema: ACL + audit + chunks + normalized labels + sqlite-vec (768-dim).
-- Migrations run from Go at gateway boot; ingestlib writes the same schema offline.
-- Vector extension: github.com/asg017/sqlite-vec (vec0 virtual table).

PRAGMA foreign_keys = ON;

-- ── ACL (seeded by ingest from Synthea principals) ──────────────────────────

CREATE TABLE IF NOT EXISTS role_grants (
  role  TEXT NOT NULL,
  label TEXT NOT NULL,
  PRIMARY KEY (role, label)
);

CREATE TABLE IF NOT EXISTS agent_scope (
  kid   TEXT NOT NULL,
  label TEXT NOT NULL,
  PRIMARY KEY (kid, label)
);

CREATE TABLE IF NOT EXISTS revoked_agents (
  kid TEXT PRIMARY KEY
);

-- ── Chunks + labels (required_labels frozen at ingest) ──────────────────────

CREATE TABLE IF NOT EXISTS chunks (
  id            TEXT PRIMARY KEY,
  text          TEXT NOT NULL,
  parent_doc_id TEXT NOT NULL DEFAULT '',
  token_count   INTEGER NOT NULL,
  corpus        TEXT NOT NULL DEFAULT '' -- wikidoc | synthea
);

-- Normalized labels enable required ⊆ eff BEFORE KNN (engine-level pre-filter).
CREATE TABLE IF NOT EXISTS chunk_labels (
  chunk_id TEXT NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
  label    TEXT NOT NULL,
  PRIMARY KEY (chunk_id, label)
);

CREATE INDEX IF NOT EXISTS idx_chunk_labels_label ON chunk_labels(label);

-- ── Embeddings (768 float32 = nomic-embed-text) ───────────────────────────────

CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vec USING vec0(
  chunk_id  TEXT PRIMARY KEY,
  embedding FLOAT[768]
);

-- ── Audit hash chain ────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS audit_log (
  seq       INTEGER PRIMARY KEY AUTOINCREMENT,
  prev_hash BLOB NOT NULL,
  row_hash  BLOB NOT NULL,
  payload   BLOB NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ── Thesis dashboard (cumulative; updated from meter.Result per request) ─────

CREATE TABLE IF NOT EXISTS stats_counters (
  id                INTEGER PRIMARY KEY CHECK (id = 1),
  leaks_blocked     INTEGER NOT NULL DEFAULT 0,
  would_be_tokens   INTEGER NOT NULL DEFAULT 0,
  auth_tokens       INTEGER NOT NULL DEFAULT 0,
  dollars_saved     REAL    NOT NULL DEFAULT 0.0,
  total_requests    INTEGER NOT NULL DEFAULT 0,  -- denominator for empty_set_rate
  empty_set_count   INTEGER NOT NULL DEFAULT 0,  -- authorized set empty → 0 LLM tokens
  tier_downgrades   INTEGER NOT NULL DEFAULT 0,  -- naive frontier → actual local/refuse
  downgrade_dollars REAL    NOT NULL DEFAULT 0.0  -- $ saved by tier downgrade alone
);

INSERT OR IGNORE INTO stats_counters (id) VALUES (1);

-- Idempotent upgrades for app.db files created before the KPI split. SQLite has no
-- ADD COLUMN IF NOT EXISTS; Migrate tolerates the "duplicate column name" error on re-run.
ALTER TABLE stats_counters ADD COLUMN total_requests    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE stats_counters ADD COLUMN empty_set_count   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE stats_counters ADD COLUMN tier_downgrades   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE stats_counters ADD COLUMN downgrade_dollars REAL    NOT NULL DEFAULT 0.0;

-- Per-request savings_pct samples — enables p50/p90/p99 (token-weighted mean hides the tail).
CREATE TABLE IF NOT EXISTS request_savings (
  seq         INTEGER PRIMARY KEY AUTOINCREMENT,
  savings_pct REAL NOT NULL
);

-- ── Prefilter query pattern (PrefilterTopK) ─────────────────────────────────
-- eff_labels = temp table or bound IN-list of effective label strings.
--
--   SELECT c.id, c.text, c.parent_doc_id, c.token_count
--   FROM chunks c
--   JOIN chunk_vec v ON v.chunk_id = c.id
--   WHERE NOT EXISTS (
--     SELECT 1 FROM chunk_labels req
--     WHERE req.chunk_id = c.id
--       AND req.label NOT IN (/* eff labels */)
--   )
--   WHERE v.embedding MATCH ?
--     AND k = ?
--     AND NOT EXISTS (
--     SELECT 1 FROM chunk_labels req
--     WHERE req.chunk_id = c.id
--       AND req.label NOT IN (/* eff labels */)
--   )
--   ORDER BY v.distance;
--
-- ShadowTopK (meter B1 baseline): same ORDER BY, no label WHERE, SELECT metadata only.
