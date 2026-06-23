# gateway — Production Identity Gateway

Go trusted core per [`../IMPLEMENTATION.md`](../IMPLEMENTATION.md) (design of record) and
[`../DECISION.md`](../DECISION.md) (thesis rationale).

**Status:** P0–P6 landed — sqlite-vec prefilter, verify, ACL, ingest seed, Ollama embed/gen,
HTTP routes, audit hash-chain, stats, and demo arc. See [`../context.md`](../context.md) for
session handoff and bootstrap.

## Module

`module github.com/agent-auth/gateway` (Go 1.25+). Requires **CGO** for sqlite-vec:

```bash
cd gateway && CGO_ENABLED=1 go test ./... && CGO_ENABLED=1 go build ./...
```

## Component map

| Package | Responsibility | Status |
|---|---|---|
| `cmd/gateway` | Open `APP_DB`, migrate, listen | **Done** |
| `cmd/ingest` | OFFLINE demo corpus + ACL seed | **Done** |
| `cmd/keygen` | Demo Ed25519 key material | **Done** |
| `internal/httpapi` | 4 routes, unified refusal | **Done** |
| `internal/verify` | DPoP + OBO + nonce | **Done** |
| `internal/resolve` | `Effective(...)` meet | **Done** |
| `internal/acl` | grants / agentScope / revocation | **Done** |
| `internal/embed` | Query-time Ollama embed | **Done** |
| `internal/gen` | Local + frontier Ollama gen | **Done** |
| `internal/retrieve` | `SQLiteRetriever` prefilter + shadow | **Done** |
| `internal/meter` | `Compute`, `TierForShadow` | **Done** |
| `internal/route` | `IsRestricted` egress tier | **Done** |
| `internal/audit` | Hash-chain append/verify | **Done** |
| `internal/stats` | Cumulative KPI counters | **Done** |
| `internal/store` | SQLite schema + **`MemNonceStore`** | **Done** |
| `internal/labelvocab` | Labels, dominance, egress family | **Done** |
| `ingestlib` | Python wrapper → `cmd/ingest` | **Done** |
| `pylib/agent_auth` | PyNaCl signer client | **Done** |

## Stack notes (IMPLEMENTATION overrides DECISION)

- **Vector store:** sqlite-vec in `app.db` — not Chroma.
- **Nonce replay:** in-memory `MemNonceStore` — **not Redis**.
- **Rate-limit / session:** not implemented (single-node demo).

## Invariants

- Authorization is pure trusted code; LLM never participates in access decisions.
- `Retriever` receives **only** effective labels.
- Pre-filter is **engine-level** (before ANN), never post-filter.
- Audit append failure → deny request.
