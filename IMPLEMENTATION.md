# Agent-Auth Gateway — Implementation Design

Real wired path ("2b"). Derived from `DECISION.md` + the existing interface stubs in
`gateway/internal/*`. This document is the design of record; the phased implementation plan
is built from it.

> Trust thesis (DECISION.md): **security and token-reduction are the same operation.** One
> pre-retrieval authorization filter minimizes both attack surface and context cost. This
> design does not merely assert that claim — it builds the trusted Go core, the offline ingest,
> and the **measurement harness** needed to **test** the thesis under sparse/dense/empty regimes
> with honest baselines (B1/B2) and dashboard KPIs.

## Thesis experiment (what we are proving)

The architecture is production-grade on its own; the **experiment** is whether the duality holds
when authz stays **label-decidable pre-retrieval** (metadata only, no chunk text on the hot path).

**Regimes** (name them or a RAG-literate reviewer breaks the pitch):

| Regime | Condition | What happens | Thesis |
|---|---|---|---|
| **Empty** | `eff == ∅` or `n_authorized == 0` | Unified refusal, **0 tokens**, no embed/gen | Security and cost both zero — side-channel closed |
| **Sparse** | `|authorized| < k` | Pre-filter returns fewer than `k` chunks; naive top-k would stuff forbidden slots | `tokens_saved_context ≈ leaks_blocked_in_topk × avg_chunk_tokens` — **same quantity**, duality exact |
| **Dense** | `|authorized| ≥ k` | Both secure pre-filter and naive top-k return `k` chunks → context tokens ≈ equal vs B1 | Context savings vs B1 ≈ 0; **leaks still blocked** |

State honestly: duality is exact *where it matters* — under-privileged principals against a broad
corpus are almost always **sparse**.

**Two baselines** (not one):

| Baseline | Mechanism | What it proves |
|---|---|---|
| **B1: insecure top-k** | `retrieve.ShadowTopK` — no label filter, metadata only | `leaks_blocked` + sparse-regime token savings |
| **B2: realistic secure RAG** | over-fetch `k+buffer` → ANN → post-filter → (often) rerank | Pre-filter also kills buffer tokens and rerank cost; no forbidden text transits the model — the **fair fight** |

Nobody secure ships B1; they ship **B2**. Meter and the P6 harness report savings against both.

**Dashboard KPI** (`GET /v1/stats`, backed by `stats_counters`): cumulative
`leaks_blocked`, rolling `tokens_saved_pct`, `dollars_saved` — one number, both KPIs (demo arc step 5).

**P6 thesis harness**: offline query set over seeded principals; for each query run authorized
path vs B1 shadow vs (simulated) B2 buffer+post-filter; emit savings%, leaks table, regime tags
(sparse/dense/empty). This is the experiment that **tests** the theory; MVP `afr_mvp.py` remains
the mechanism reference only (**do not modify**).

## Locked decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| 1 | JWT library | `github.com/golang-jwt/jwt/v5`, pinned `WithValidMethods(["EdDSA"])` | Pure-Go, no transitive deps ("small footprint" intact); pinning EdDSA eliminates the alg-confusion attack class (`alg:none`, RS/HS confusion); free `exp`/`nbf`/`aud` validation. Crypto stays `crypto/ed25519` stdlib, so DECISION's "no hand-rolled crypto" pitch holds. |
| 2 | Agent pubkey transport | **Method A** — `jwk` embedded in DPoP header; gate `sha256(jwk) == OBO.act` | RFC 9449 standard shape; self-contained (no registry round-trip on hot path); the `act` binding makes the embedded key trustworthy (attacker can't swap a key without breaking the OBO signature). Keeps `verify` stateless / dependency-free. Honors DECISION:153 "ACL store never in request path". |
| 3 | Scope | **2b** — full real wired path (real Chroma-replacement + Ollama + ingest), no fakes | Demoable for real. |
| 4 | Generation | **Gateway generates** the answer (new `internal/gen`); `route.Tier` selects the model | Only way `route` egress control has teeth under the assume-breach threat model: the untrusted agent never sees raw restricted chunks; gateway feeds them to the local model and returns only the answer. Matches HTTP contract `200 {result, chunks[]}`. |
| 5 | Vector store | **sqlite-vec** | One embedded `app.db` already holds ACL + audit (DECISION:95); vectors join it. Zero extra services, lowest RAM on M1 8GB, label `WHERE` pre-filter + KNN in one SQL query. Swap to Qdrant/pgvector later behind the `Retriever` interface. |
| 6 | Embedding model | **`nomic-embed-text`** via Ollama, 768-dim, **same model ingest + query** | Single Ollama runtime for embed + gen (DECISION:223); retires MVP's MiniLM. Sets sqlite-vec vector dimension = 768. |
| 7 | Audit | Real SQLite hash-chain **now** | `append-fail → deny` (DECISION:200) is on the `/v1/retrieve` hot path; the endpoint cannot function without it. Not deferrable. |
| 8 | Revocation | `acl.IsRevoked(kid)` runs in the **httpapi wiring layer** (after Verify, before resolve) | Keeps `verify` pure crypto; matches Method A's stateless-verify boundary; keeps the ACL store off the crypto core. |
| 9 | Corpus | **Both** — WikiDoc (HuggingFace `medical_meadow_wikidoc`) topic labels + Synthea synthetic notes Presidio-labeled `phi:patient:<id>` | WikiDoc = public knowledge corpus, no native ACL (proves external-join). Synthea = synthetic principals + PHI notes that power the row-level isolation demo (arc steps 2–3). Presidio fires on Synthea, no-ops on WikiDoc. |
| 10 | Nonce / replay store | **In-memory** (`map` + mutex + TTL sweep) | Zero infra, fits M1 budget. Single-node demo; nonces expire within the clock window. Swap to Redis/Valkey later behind `store.NonceStore`. |
| 11 | Unit tests | **None this round** (every phase) | Explicit user decision; tests are a later addition. |
| 12 | Frontier generation model | **Ollama `llama3.2`** (HTTP, same runtime as embed + local gen) | Frontier tier = de-identified/public chunks only; still one Ollama pool on M1. Local tier stays **`phi4-mini`** (BAA boundary). |
| 13 | Authz vs egress label rules | **Authz:** exact `eff.Dominates(required)` on frozen chunk labels (after `ClearanceFrom`). **Egress:** `labelvocab.IsRestricted(l)` family (phi/restricted/sensitivity categories) in `route.Decide` | Same label vocabulary, two predicates: subset check gates retrieval; broader sensitivity family gates which model may process authorized data. Fail-closed on both. |

## Actors & trust boundary

```
[User] --delegates--> [Agent]            HTTP /v1/retrieve         [GATEWAY]
                       untrusted     ------------------------>     trusted Go core
                       holds enclave key                          (this repo)
                       signs OBO + DPoP                                 |
                       may be prompt-injected                           v
                                                          app.db (sqlite-vec)
                                                          acl + audit + chunks + chunk_labels + vectors
                                                                 |
                                                          [Ollama] nomic-embed-text + phi4-mini + llama3.2

[ingestlib]  OFFLINE Python  --writes-->  app.db (same schema as gateway/internal/store/schema.sql)
  HuggingFace WikiDoc loader + Synthea notes + Presidio labeler + Ollama embed
```

- **Agent** is untrusted: runs LLM-driven logic, can be injected, holds the Ed25519 enclave key.
  Its model output never participates in the authz decision (DECISION:130, CaMeL).
- **Gateway** is the trusted deterministic boundary. Threat model (DECISION:244):
  *assume-breach — injection succeeds and still gets nothing.*
- **Python** runs offline ingest only. **Go** runs the online request path only. Clean seam.

## Identity verification — `internal/verify`

`StandardVerifier` implements `Verifier.Verify(ctx, *Request) (*Principal, error)` against the
existing `verify.go` types (do not change `Request`/`Principal`/`ErrDenied`).

Fields in play: `Request{Body, OBO, Sig, Nonce, Timestamp, Method, URI}` →
`Principal{UserID, UserRoles, AgentKID, TaskScope}`.

**Order (fail-closed, cheapest-decisive first — DECISION:104):**

1. **Parse DPoP `Sig`** (golang-jwt, pinned EdDSA): extract `jwk` from header = agent pubkey;
   verify the DPoP signature with it. *(PoP first; self-contained, no key lookup.)*
2. **DPoP claims:**
   - `htm == Request.Method`
   - `htu == Request.URI`
   - `iat` within clock-skew window (use `Request.Timestamp`)
   - `body_hash == base64url(sha256(Request.Body))`
3. **Nonce / jti:** require `jti == Request.Nonce`; `NonceStore.SeenBefore(ctx, jti)` → reject replay.
4. **OBO** (golang-jwt, pinned EdDSA, key = trusted `IssuerPubKey`): verify signature; check
   `exp` (reject expired) and `aud`; extract `user_roles`, `task_scope`, `act`.
5. **Bindings:**
   - `sha256(jwk) == act` (proof-of-possession binding)
   - `ath == base64url(sha256(OBO))`
6. **Build Principal:** `UserID`, `UserRoles`, `AgentKID = sha256(jwk)`, `TaskScope` (from OBO).

> The `ath`/`act` cross-bindings necessarily run after the OBO is parsed; PoP signature + all
> OBO-independent DPoP claims are checked before any OBO work, satisfying "PoP → nonce → OBO".

**Single opaque error:** every failure returns `ErrDenied` only — never a descriptive or wrapped
error. Callers must not be able to branch on the reason (existence side-channel closed).
`StandardVerifier` holds `IssuerPubKey ed25519.PublicKey` + a `store.NonceStore`.

## Authorization core

- **`internal/resolve`** — **already implemented** (`Effective = grants ⊓ agentScope ⊓ (reqTask ⊓ oboTask)`).
  Pure, table-tested shape; untouched.
- **`internal/acl`** — SQLite-backed `acl.Store` (`Grants`, `AgentScope`, `IsRevoked`), seeded by ingest.
  **Grants union (multi-role):** for each `role ∈ principal.UserRoles`, load `acl.Grants(role)`, expand
  with `labelvocab.ClearanceFrom`, then **`grantsUnion = ⋃ roleGrants`** via `LabelSet.Union`. Single-role
  MVP keyword match is retired.
- **Fail-closed defaults:** empty union, empty agent scope, or meet that collapses to `∅` → unified refusal
  (0 tokens); never widen effective labels from agent/model input.
- **`internal/route`** — **already implemented** egress gate: `Decide(authorizedChunkLabels) → Refuse | Local | Frontier`.
  Uses **`labelvocab.IsRestricted(l)`** per label (PHI/sensitivity **family**), not the same predicate as authz.

**Label rules — authz vs egress**

| Concern | Predicate | Where | Fail mode |
|---|---|---|---|
| **Retrieval (authz)** | `eff.Dominates(required)` — exact subset on frozen chunk labels (conf chain via `ClearanceFrom`) | `retrieve.PrefilterTopK` SQL + dominance check | Chunk excluded before ANN; never post-filter on hot path |
| **Model egress** | `labelvocab.IsRestricted(l)` — `phi`, `phi:*`, `restricted`, `note:provider`, `note:psych`, `genetic`, `hiv`, `sud:*`, … | `route.Decide` | Any hit → **Local** (`phi4-mini`); all clear → **Frontier** (`llama3.2`); empty auth → **Refuse** |

## Request path — `POST /v1/retrieve`

```
verify.Verify(req) ............................ fail → 403 {}
acl.IsRevoked(principal.AgentKID) ............. revoked → unified refusal (200)
grantsUnion = ⋃ ClearanceFrom(acl.Grants(role)) for role ∈ UserRoles
eff = resolve.Effective(grantsUnion, acl.AgentScope(kid), reqTask, oboTask)
eff == ∅ ...................................... unified refusal, 0 tokens (no model called)
qVec   = embed.Embed(query)                     (Ollama nomic-embed-text)
k      = min(defaultK, n_authorized)            (defaultK = 5)
auth   = retrieve.PrefilterTopK(qVec, eff, k)   (sqlite-vec: required ⊆ eff BEFORE ANN)
shadow = retrieve.ShadowTopK(qVec, k)           (B1 meter — metadata, no text)
tier   = route.Decide(labels(auth))
tierNaive = meter.TierForShadow(shadow)         (egress tier B1 would have forced)
answer = gen[tier].Generate(query, auth)        (Local phi4-mini | Frontier llama3.2)
m      = meter.Compute(shadow, auth, eff, tier, tierNaive)
stats_counters += m                               (cumulative KPI for /v1/stats)
audit.Append(ids + labels + counts) ........... append-fail → deny
return 200 {result: answer, chunks: auth}       (counts NEVER here)
```

**Unified refusal** (DECISION:201): "no access" and "no data" return identical
`{result:"not permitted or no data", chunks:[]}` with **HTTP 200** and 0 tokens. Side-channel closed.
(Crypto/identity failures remain **403 {}**.)

## HTTP contracts — `internal/httpapi`

```
POST /v1/retrieve      Authorization: Bearer <OBO>; X-Sig (DPoP); X-Nonce; X-Timestamp
                       body {query, task_scope[]} → 200 {result, chunks[]} | unified refusal | 403 {}
GET  /v1/stats         {leaks_blocked, tokens_saved_pct, dollars_saved}   (separate scope)
GET  /v1/audit         chain read + Verify                                (admin scope)
POST /v1/admin/agents  register agent pubkey → {kid, scope}               (admin scope)
```

Agent response is `chunks[]` only — `leaks_blocked`/`denied_count` go to `/v1/stats` + audit, never to the agent.

## Storage — `internal/store`, one `app.db` (sqlite-vec)

**Schema of record:** `gateway/internal/store/schema.sql` — gateway migrations and **ingestlib** both
apply the same DDL (ACL, audit, chunks, **`chunk_labels`**, `chunk_vec` vec0 768-dim).

- SQLite open + migrations at boot. Parameterized queries only.
- **`chunks`**: id, text, parent_doc_id, token_count, corpus — **no** embedded label JSON.
- **`chunk_labels`**: normalized `(chunk_id, label)` rows; ingest writes one row per required label.
  Enables engine-level pre-filter: every required label must appear in `eff` before KNN.
- **`chunk_vec`**: sqlite-vec virtual table, `FLOAT[768]` for `nomic-embed-text`.
- **Prefilter SQL** (see comments in `schema.sql`): `NOT EXISTS` subquery on `chunk_labels` with
  `req.label NOT IN (eff…)` **before** `ORDER BY vec_distance_cosine … LIMIT k`.
- **In-memory `NonceStore`**: `SeenBefore` atomically records jti and reports prior use; TTL sweep
  drops entries past the clock window.
- **`stats_counters`**: single-row (or keyed) aggregate updated atomically from `meter.Result` after
  each successful retrieve — feeds `/v1/stats` without exposing counts to the agent.

## Configuration (bootstrap)

| Variable / flag | Default | Purpose |
|---|---|---|
| `AGENT_AUTH_DB` | `./app.db` | SQLite path (ACL + audit + chunks + vec) |
| `GATEWAY_ADDR` | `:8080` | HTTP listen |
| `ISSUER_PUBKEY` | *(required)* | Ed25519 public key (hex or file path) for OBO verify |
| `OLLAMA_URL` | `http://127.0.0.1:11434` | Embed + generation HTTP |
| `OLLAMA_EMBED_MODEL` | `nomic-embed-text` | Query + ingest embedding (768-d) |
| `OLLAMA_LOCAL_MODEL` | `phi4-mini` | `route.Local` generation |
| `OLLAMA_FRONTIER_MODEL` | `llama3.2` | `route.Frontier` generation |
| `DEFAULT_K` | `5` | Top-k cap; runtime uses `min(DEFAULT_K, n_authorized)` |
| `CLOCK_SKEW_SEC` | `60` | DPoP `iat` window |
| `AUDIT_VERIFY_ROWS` | `0` (= all) | Chain verify at boot; mismatch → refuse start |

## Audit — `internal/audit` (real now)

- `row_hash = sha256(prev_hash || canonical_json(payload))`, append-only SQLite.
- `Append` = one INSERT chaining to `prev_hash`; failure → request denied (no unrecorded access).
- `Verify(n)` walks the chain at boot; mismatch → gateway refuses to start.
- Logs IDs + labels + counts only — **never chunk text** (audit must not become a PHI store).

## Generation — `internal/gen` (NEW)

- `Generator` interface: `Generate(ctx, query string, chunks []retrieve.Chunk) (string, error)`.
- Two impls: **Local** (Ollama `phi4-mini`, BAA-safe) and **Frontier** (Ollama `llama3.2`).
- `route.Tier` selects the impl: `Local` when any authorized chunk carries an `IsRestricted` label,
  `Frontier` when all labels are de-identified/public, `Refuse` short-circuits (no model call). Egress
  is enforced by the gateway, not advised to the agent.

## Retrieval & meter

- **`internal/embed`** — Ollama `nomic-embed-text` HTTP client (query-time embedding only on hot path).
- **`internal/retrieve`** — sqlite-vec `Retriever`: `PrefilterTopK` (label SQL **before** KNN, never
  post-filter) and `ShadowTopK` (unfiltered top-k, **metadata only**, no chunk text).
- **`internal/meter`** — per-request `Result` and cumulative **`stats_counters`**:

  - `WouldBeTokens = Σ TokenCount(shadow)` (B1)
  - `AuthTokens = Σ TokenCount(auth)`
  - `LeaksBlocked = |{ c ∈ shadow : ¬ eff.Dominates(c.RequiredLabels) }|`
  - `SavingsPct = (WouldBeTokens − AuthTokens) / WouldBeTokens × 100` (0 if `WouldBeTokens == 0`)
  - `DollarsSaved = pricePer1K(tierNaive) × WouldBeTokens/1000 − pricePer1K(tier) × AuthTokens/1000`
    where `tierNaive = meter.TierForShadow(shadow)` and tier is the egress actually used.

  Token counts are precomputed at ingest. Dashboard reads rolling aggregates from `stats_counters`.

## Offline ingest — `ingestlib` (Python)

- Load HuggingFace `medical_meadow_wikidoc`; load Synthea synthetic FHIR (Practitioner/Patient/
  CareTeam) → seed users/roles/grants into `role_grants` / `agent_scope`.
- Chunk: recursive split + split at every sensitivity boundary; record `parent_doc_id`.
- Label (tiered, fail-closed = uncertain → more restrictive): FHIR inherit ~70% / schema-rule
  ~20% / **Presidio** PHI-classifier ~10%. WikiDoc → topic labels (`lab`/`billing`/`note:provider`);
  Synthea notes → row-level `phi:patient:<id>`.
- Embed each chunk via Ollama `nomic-embed-text`; precompute `token_count`.
- Write **`app.db` using the same schema as `schema.sql`**: `chunks` + **`chunk_labels`** (normalized,
  one row per label) + `chunk_vec` embeddings + seeded ACL. Labels frozen offline; request-time
  enforcement reads them deterministically.

## Agent signer — `pylib/agent_auth` (Python)

~50-line PyNaCl client that mints a valid OBO (signed by the issuer key) + per-request DPoP
(signed by the agent enclave key, `jwk` in header, `htm/htu/ath/jti/iat` + `body_hash`). Used to
drive the demo and any manual verification.

## Build phases

| Phase | Deliverable |
|---|---|
| **P0 Foundation** | `go.mod` deps (golang-jwt/v5, sqlite + sqlite-vec driver); `store` SQLite open + migrations from `schema.sql` |
| **P1 Identity** | `verify` `StandardVerifier` (golang-jwt, Method A); in-memory `NonceStore`; `pylib/agent_auth` signer |
| **P2 Authz** | `acl` SQLite impl; grants union wiring; revocation in httpapi |
| **P3 Ingest** (Python) | `ingestlib`: WikiDoc + Synthea + Presidio + Ollama embed → `app.db` + `chunk_labels`; seed ACL |
| **P4 Retrieval + gen** | `embed`, `retrieve` (sqlite-vec prefilter SQL), `gen` (phi4-mini + llama3.2), `meter` + `stats_counters` |
| **P5 HTTP + audit + boot** | `audit` hash-chain; `httpapi` (4 routes); `cmd/gateway` wire + migrate + `audit.Verify(n)` + config bootstrap |
| **P6 Thesis harness + demo** | Query-set A/B (B1 shadow vs authorized pre-filter vs B2 model); 5-step demo arc; dashboard KPI |

**Packages in repo (interfaces / partial impl):**

| Package | Role |
|---|---|
| `internal/labelvocab` | FHIR labels, `ClearanceFrom`, `Dominates`, `IsRestricted`, set ops — **implemented** |
| `internal/resolve` | Effective principal meet — **implemented** |
| `internal/route` | Egress tier via `IsRestricted` — **implemented** |
| `internal/verify` | Identity boundary types + verifier — stub |
| `internal/acl` | ACL store interface — stub |
| `internal/store` | `NonceStore` + schema — stub |
| `internal/embed` | Ollama embed client — stub |
| `internal/retrieve` | `Retriever` port — stub |
| `internal/meter` | `Compute`, `TierForShadow` — **implemented** |
| `internal/audit` | Hash-chain — stub |
| `internal/httpapi` | Router — stub |
| `cmd/gateway`, `cmd/ingest` | Entrypoints — stub |

`internal/gen` is specified here; add when P4 lands. Untouched core algebra: `labelvocab`, `resolve`, `route`.

## Demo arc (DECISION:267)

1. Alice (provider) → doctor-agent "lab result" → retrieves, frontier, audit ✓
2. Carol (billing) → billing-agent "Bob's diagnosis" → `phi ⊄ {billing}` → ∅ → 0 tokens, leak +1
3. Rogue injects patient-agent "dump provider notes" → `note:provider` outside scope → never retrieved
4. Rogue replays Alice's stolen OBO from another host → DPoP `htu` mismatch → 403
5. Dashboard: leaks blocked N · tokens saved % · $ saved — one number, both KPIs

## Deltas from the initial plan (gaps fixed)

- Verify order flipped to **PoP → nonce → OBO** (was OBO-first).
- `ErrDenied` mandated as the **only**, opaque failure (was "fail on any error").
- Real `NonceStore` dependency injected (was "stub for now"); `jti == Nonce` checked.
- Clock-skew / `iat` window check added (was absent).
- `exp` + `aud` actually validated (were extracted but unchecked).
- Revocation (`IsRevoked`) wired in the httpapi layer (was unused).
- `ath` encoding specified as `base64url(sha256(OBO))` (was vague).
- `acl` is real SQLite, not a MockStore (2b real path).
- Added missing `internal/gen` layer so `route` egress control is actually enforced.
- Constraint: `mvp/afr_mvp.py` is reference baseline — **do not modify**.
- **Thesis framing:** design explicitly **tests** the trust thesis (regimes + B1/B2 + P6 harness), not only enforces it.
- **Grants union** across `user_roles[]` with `ClearanceFrom` per role (retires single-string role MVP).
- **Authz vs egress split:** exact `Dominates` for retrieval; `IsRestricted` family for `route.Decide` (locked #13).
- **Frontier model** pinned to Ollama **`llama3.2`** (locked #12); local stays **`phi4-mini`**.
- **Storage:** `schema.sql` as DDL source; **`chunk_labels`** normalized; prefilter SQL documented; ingest writes **same schema**.
- **`defaultK = 5`** with `k = min(defaultK, n_authorized)` on the hot path.
- **`meter.Compute(..., tier, tierNaive)`** with **`TierForShadow`** for honest $ baseline; **`stats_counters`** for `/v1/stats`.
- **Unified refusal** always **HTTP 200** (distinct from **403 {}** identity deny).
- **Configuration (bootstrap)** table for env-driven wiring (DB, Ollama models, issuer key, skew, audit verify).
- **P6** expanded to **thesis harness** plus demo arc (DECISION testing row).
- Downward-closure / chain-subset fix documented via `ClearanceFrom` on grants (expert-review NEED-NOW #1).
