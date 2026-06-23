# gateway — Production Identity Gateway (scaffold)

Go implementation of the production design in [`../DECISION.md`](../DECISION.md). This is the
trusted core: it proves identity cryptographically, computes authorization deterministically,
pre-filters retrieval at the engine layer, meters tokens, and audits — all before any LLM runs.

**Status:** directory scaffold + per-package responsibility docs. No Go code yet — see
`DECISION.md` and run `writing-plans` to produce the implementation plan.

## Module

`module github.com/agent-auth/gateway` (Go 1.22). Build once code lands: `go build ./...`.

## Component map (→ DECISION.md)

| Package | Responsibility | DECISION.md section |
|---|---|---|
| `cmd/gateway` | wire-up, listen, run migrations, `audit.Verify(n)` at boot | Component layout |
| `cmd/ingest` | OFFLINE: chunk → label → embed → load Chroma + ACL store | Ingest & ACL store |
| `internal/httpapi` | routes; error → `403 {}` or unified refusal; no counts to agent | HTTP contracts |
| `internal/verify` | `VerifyOBO`, `VerifyPoP` (DPoP: `htm/htu/ath/jti` + body hash), nonce+clock (Redis) | Identity boundary |
| `internal/resolve` | `Effective(...)` — PURE, table-tested lattice **meet**; `required ⊑ effective` | Authorization core |
| `internal/acl` | `grants(role)`, `agentScope(kid)`, revocation; reads ACL store | Authorization core |
| `internal/embed` | `Embedder` iface — embeds the **query at request time** | Retrieval (NEED-NOW) |
| `internal/retrieve` | `Retriever` iface + Chroma impl: `PrefilterTopK`, `ShadowTopK` | Retrieval & meter |
| `internal/meter` | would-be vs auth tokens, savings%, `leaks_blocked`, `$` (B1/B2 baselines) | Thesis math / meter |
| `internal/route` | sensitivity-gated egress: empty→refuse · phi→local · else→frontier | Routing (egress) |
| `internal/audit` | `Append` (hash-chain), `Verify(n)` | Audit & fail-closed |
| `internal/store` | SQLite (ACL + audit), Redis/Valkey (nonce/session/ratelimit) | Stack |
| `internal/labelvocab` | FHIR label constants + `LabelSet` set-ops (meet, dominance) | Label model |
| `pylib/agent_auth` | ~50-line PyNaCl client: signs requests, calls the gateway | Component layout |
| `ingestlib` | OFFLINE python: Synthea loader + Presidio labeler (not on request path) | Ingest & ACL store |

## Invariants (do not violate)

- Authorization (`resolve` + `required ⊑ effective`) is **pure, deterministic, trusted code**.
  The LLM never participates in its own access decision.
- `Retriever` receives **only** the effective label set — never identity, intent, or model output.
- Pre-filter is **engine-level** (before ANN), never post-filter.
- **Fail-closed** everywhere; audit-append failure → deny the request.
