# Agent-Auth — Session Context

Handoff file for new sessions. **Design of record:** [`IMPLEMENTATION.md`](IMPLEMENTATION.md) (supersedes stack rows in [`DECISION.md`](DECISION.md) where they differ).

## Thesis under test

**Principle of Least Context:** one pre-retrieval authz filter minimizes attack surface *and* context tokens. Same filter blocks leaks and prunes prompt size. Prove via meter: `leaks_blocked`, `tokens_saved_pct`, `dollars_saved` (B1 `ShadowTopK` vs authorized `PrefilterTopK`).

**Target machine:** local Mac M1, 8 GB RAM — one Ollama runtime, no Redis, no Chroma, sqlite-vec in-process.

## Workflow convention

- Work on **`main`**, commit, **push** to `origin/main` when a chunk is done.
- Do **not** modify [`mvp/afr_mvp.py`](mvp/afr_mvp.py) (frozen proof-of-mechanism).

## Stack (locked — IMPLEMENTATION.md)

| Concern | Choice |
|---------|--------|
| Vector store | **sqlite-vec** in `app.db` (CGO: `mattn/go-sqlite3` + `sqlite-vec-go-bindings/cgo`) |
| Nonce / replay | **In-memory** `store.MemNonceStore` — **not Redis** |
| ACL + audit + chunks | Single SQLite `app.db`, schema in `gateway/internal/store/schema.sql` |
| Embed + gen | Ollama: `nomic-embed-text` (768-dim), `phi4-mini` (local), `llama3.2` (frontier demo) |
| Offline ingest | Python **`ingestlib`** → same `app.db` (WikiDoc + Synthea + Presidio) |
| Online path | Go gateway only |

Build/tests: `cd gateway && CGO_ENABLED=1 go test ./... && CGO_ENABLED=1 go build ./...`

## Done

### P0 — Foundation (mostly complete)

- [x] `schema.sql` — ACL tables, `chunks`, **`chunk_labels`**, `chunk_vec` (vec0 768-dim), `audit_log`, `stats_counters`
- [x] `store.Open` / `Migrate` — embeds schema, runs at boot
- [x] `retrieve.SQLiteRetriever` — **`PrefilterTopK`** (label SQL filter **before** vec0 `MATCH` + `k=`) and **`ShadowTopK`** (B1 baseline)
- [x] Tests: `store/db_test.go`, `retrieve/sqlitevec_test.go`, `store/nonce_mem_test.go`
- [x] `cmd/gateway` opens `APP_DB` (default `./app.db`), runs migrations

### Core algebra (complete)

- [x] `labelvocab` — `Meet`, `Union`, `ClearanceFrom`, `Dominates`, `IsRestricted`, `Strings`
- [x] `resolve.Effective` — grants ⊓ agentScope ⊓ (reqTask ⊓ oboTask)
- [x] `route.Decide` — PHI family → Local via `IsRestricted`
- [x] `meter.Compute`, `meter.TierForShadow`

### Design doc

- [x] `IMPLEMENTATION.md` — thesis experiment, bootstrap config table, authz vs egress split, demo arc

## Not done (build order for M1)

### P1 — Identity (next after nonce store)

- [ ] `verify.StandardVerifier` (golang-jwt, EdDSA, PoP → nonce → OBO)
- [ ] Wire `store.NewMemNonceStore` into verifier
- [ ] `pylib/agent_auth` PyNaCl signer (~50 lines)
- [ ] Issuer key bootstrap (`ISSUER_PUBKEY_PATH`, `OBO_AUD`, `CLOCK_SKEW`)

### P2 — Authz

- [ ] SQLite `acl.Store` over `role_grants` / `agent_scope` / `revoked_agents`
- [ ] Grants union + `ClearanceFrom` in httpapi wiring
- [ ] `IsRevoked` after verify

### P3 — Ingest (**blocks meaningful demo**)

- [ ] `ingestlib` (Python): WikiDoc + Synthea + Presidio + Ollama embed → `app.db`
- [ ] Seed ACL from Synthea principals
- [ ] Small corpus slice (~50 WikiDoc rows) for 8 GB RAM

### P4 — Retrieval hot path

- [ ] `embed` Ollama client (`nomic-embed-text`)
- [ ] **`internal/gen`** (new package): Local + Frontier Ollama clients
- [ ] Update `stats_counters` from `meter.Result` per request
- [ ] `k = min(defaultK, n_authorized)` in handler

### P5 — HTTP + audit

- [ ] `POST /v1/retrieve` full pipeline
- [ ] `GET /v1/stats`, `GET /v1/audit`, `POST /v1/admin/agents`
- [ ] Hash-chain `audit.Append` / `Verify(n)` at boot; append-fail → deny
- [ ] Unified refusal HTTP 200 vs verify `403 {}`

### P6 — Thesis harness + demo

- [ ] Scripted demo arc (Alice / Carol / rogue / replay)
- [ ] Query-set A/B table (B1 shadow vs authorized vs optional B2)
- [ ] Dashboard or terminal KPI rollup

## Key files

| Path | Role |
|------|------|
| `IMPLEMENTATION.md` | Design of record, phases, config env vars |
| `gateway/internal/store/schema.sql` | DDL + prefilter SQL comment |
| `gateway/internal/store/db.go` | Open + migrate |
| `gateway/internal/store/nonce_mem.go` | In-memory nonce store |
| `gateway/internal/retrieve/sqlitevec.go` | Thesis mechanism (pre-filter before KNN) |
| `gateway/internal/meter/meter.go` | Per-request metrics |
| `gateway/cmd/gateway/main.go` | Boot (DB only so far) |

## M1 8 GB runtime recipe

1. **Ingest offline** (Python): build `app.db`, then exit.
2. **Demo:** `ollama serve` + gateway only.
3. **Models:** pull `nomic-embed-text`, `phi4-mini`; add `llama3.2` for frontier step — avoid loading two large gen models at once.
4. **No** Redis, Chroma, sentence-transformers, or parallel embed stacks.

## Demo arc (must all pass)

1. Alice → doctor-agent → lab → Frontier, audit ✓
2. Carol → billing-agent → diagnosis/`phi` → ∅ → 0 tokens, leak +1 in meter
3. Rogue patient-agent injection → `note:provider` never retrieved
4. Stolen OBO replay → DPoP `htu` mismatch → 403
5. Stats: leaks blocked · tokens saved % · $ saved

## Last session stop point

**Completed:** Redis references retired in gateway docs; **`MemNonceStore`** implemented and tested; **`context.md`** added (this file).

**Stopped before:** P1 `StandardVerifier` + golang-jwt wiring.

**Suggested next commit scope:** P1 verify + issuer key env + optional minimal `pylib/agent_auth` signer.
