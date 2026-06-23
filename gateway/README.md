# gateway — Production Identity Gateway

Go trusted core per [`../IMPLEMENTATION.md`](../IMPLEMENTATION.md) (design of record) and
[`../DECISION.md`](../DECISION.md) (thesis rationale).

**Status:** P0 foundation landed — sqlite-vec migrations, `PrefilterTopK` / `ShadowTopK`, meter
algebra, in-memory nonce store. HTTP pipeline, verify, ingest, audit, and gen still TODO.
See [`../context.md`](../context.md) for session handoff.

## Module

`module github.com/agent-auth/gateway` (Go 1.25+). Requires **CGO** for sqlite-vec:

```bash
cd gateway && CGO_ENABLED=1 go test ./... && CGO_ENABLED=1 go build ./...
```

## Component map

| Package | Responsibility | Status |
|---|---|---|
| `cmd/gateway` | Open `APP_DB`, migrate, listen | DB boot only |
| `cmd/ingest` | OFFLINE Go stub | TODO — use Python `ingestlib` |
| `internal/httpapi` | 4 routes, unified refusal | Stub router |
| `internal/verify` | DPoP + OBO + nonce | Types only |
| `internal/resolve` | `Effective(...)` meet | **Done** |
| `internal/acl` | grants / agentScope / revocation | Interface only |
| `internal/embed` | Query-time Ollama embed | Interface only |
| `internal/retrieve` | `SQLiteRetriever` prefilter + shadow | **Done** |
| `internal/meter` | `Compute`, `TierForShadow` | **Done** |
| `internal/route` | `IsRestricted` egress tier | **Done** |
| `internal/audit` | Hash-chain append/verify | Interface only |
| `internal/store` | SQLite schema + **`MemNonceStore`** | **Done** |
| `internal/labelvocab` | Labels, dominance, egress family | **Done** |
| `ingestlib` | Python offline ingest | Not started |
| `pylib/agent_auth` | PyNaCl signer | Not started |

## Stack notes (IMPLEMENTATION overrides DECISION)

- **Vector store:** sqlite-vec in `app.db` — not Chroma.
- **Nonce replay:** in-memory `MemNonceStore` — **not Redis**.
- **Rate-limit / session:** not implemented (single-node demo).

## Invariants

- Authorization is pure trusted code; LLM never participates in access decisions.
- `Retriever` receives **only** effective labels.
- Pre-filter is **engine-level** (before ANN), never post-filter.
- Audit append failure → deny request.
