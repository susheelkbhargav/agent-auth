# Agent-Auth — Design Decisions

Production-grade design decisions, derived from `DESIGN.md` (hackathon thesis) + the
"Token Reduction and security are the same thing" theory + reality-check against
`afr_mvp/afr_mvp.py`. Each row = a locked decision + why.

## Expert-review triage (2026-06-23 second-agent pass)

Folded in below with inline `[NEED-NOW]` / `[GOOD-TO-HAVE]` tags. Index:

**NEED-NOW (correctness + makes it run/demo + free credibility):**
1. Chain-subset bug → downward-closure encoding (authz core).
2. DPoP naming + `htm/htu/ath/jti` binding — closes endpoint-portable replay (identity).
3. Parent-doc expansion must re-gate each chunk — siblings differ in label by design (ingest).
4. Query-time `Embedder` interface — was missing from the hot path (component layout).
5. Thesis regimes (sparse/dense) + **two baselines** B1 insecure / B2 secure-RAG (thesis math).
6. Lattice / Bell-LaPadula naming + the meet theorem; `task_scope` capping = the crux.
7. Decidability taxonomy (doc-only).
8. M1 budget: one Ollama for embed+gen; `k = min(k, n_authorized)`.

**GOOD-TO-HAVE (given M1/staging limits — defer/enterprise):** privilege/savings curve · RFC 8707
`aud` binding · SPIFFE/SPIRE SVIDs · per-tier Chroma collections · disable cloud cache for
`restricted` · Merkle audit (RFC 9162) · OAuth 2.1 resource-server / MCP-auth interop.

## Thesis under test

**Security and token-reduction are the same operation.** One pre-retrieval authorization
filter minimizes both the attack surface (lethal-trifecta data leg) and cost (context tokens).

**Verdict from the MVP check:** the theory is *sound*; the MVP validates the **mechanism**
(engine-level pre-filter prunes the candidate set before scoring; empty set → 0 tokens) but
only **asserts** the security half (`current_user_role` is an unauthenticated string, comment
falsely labels it "Verified") and does **not measure** the token half (only `char_count`, no
would-be baseline, no `leaks_blocked`). MVP also uses single-role exact-match assigned by
keyword substring — not the labels algebra.

**Sharp finding (a design constraint, not just a claim):** the duality holds *only if*
authorization is decidable from **metadata labels alone, pre-retrieval**. The moment authz must
inspect chunk *content* to decide access, you fetch-then-decide → pay tokens → "more secure =
cheaper" breaks. Production must keep authz label-decidable before scoring.

### Decidability taxonomy [NEED-NOW: doc only, free]

| Authz type | Cost at request time | Thesis |
|---|---|---|
| RBAC / ABAC / precomputed-ReBAC → **metadata labels** | free (pre-filter, no LLM) | **holds, exact** |
| ReBAC / Zanzibar relationship traversal | **latency** (graph query), not tokens | **holds** — cost is wall-clock, not context |
| **content-dependent** ("docs not mentioning X") | fetch-then-read → **tokens** | **breaks** |

Rule: keep all authz in the top two rows. Never let an access decision depend on chunk content.

### Thesis math — regimes & two baselines [NEED-NOW: thesis survives only with this]

Savings depend on a **regime** that must be named or a RAG-literate judge breaks the pitch:

- **Sparse** (`|authorized| < k`): pre-filter returns `< k`; naive stuffs `k` (some forbidden).
  Context tokens genuinely drop. `tokens_saved_context ≈ leaks_blocked_in_topk × avg_chunk_tokens`.
  Here `leaks_blocked` and `tokens_saved` are **the same quantity** — duality *exact*.
- **Dense** (`|authorized| ≥ k`): both return `k` chunks → context tokens ≈ equal vs an insecure
  baseline → context-stuffing savings ≈ **0** (leaks still blocked).

State it honestly: the duality is exact *precisely where it matters* — under-privileged
principals against a broad corpus are almost always sparse.

**Measure against TWO baselines, not one:**

| Baseline | What | Proves |
|---|---|---|
| **B1: insecure top-k** | no filter (= `ShadowTopK`) | `leaks_blocked` + sparse-regime savings |
| **B2: realistic *secure* RAG** | over-fetch `k+buffer` → retrieve → post-filter → (often) rerank | dense-regime savings too: pre-filter kills the buffer **and** the rerank call; no forbidden text transits the model |

Nobody secure builds B1 — they build **B2**. Against B2 pre-filter wins on tokens *always*. B2 is
the fair fight; add it to the thesis harness.

[GOOD-TO-HAVE] **Privilege/savings curve:** sweep principals by privilege-breadth, plot `savings%`
vs breadth → monotone decreasing. That curve *is* the thesis as an empirical result (frugality =
least-privilege, same axis). High value for the writeup; not required for the core demo.

## Goal of this design

Production blueprint across all axes (identity, authz, retrieval, measurement, ops, compliance).
The measurement layer continuously **tests the thesis**, but the architecture stands on its own
merits regardless of the savings number it produces.

## Stack — staging-ready, runs local, code supports scale (no paid clusters)

| Concern | Decision (staging, local) | Why / Enterprise path |
|---|---|---|
| Identity/authz core | **Go gateway**, deterministic, trusted code | the security heart; authz computed in trusted code, never by the LLM |
| Crypto | **Ed25519** (Go stdlib; PyNaCl on agent side) | wire-compatible; ~50-line agent sign lib |
| Retriever seam | **Go `Retriever` interface** — `PrefilterTopK(q, effLabels, k)` + `ShadowTopK(q, k)` | swap vector store = new impl, callers unchanged; receives ONLY effective labels → cannot be injection-steered |
| Vector store | **ChromaDB** behind `Retriever`, engine `where` pre-filter | proven in MVP; tiny footprint. Swap → Qdrant/pgvector at scale |
| ACL source of truth | **external ACL store** (SQLite), labels keyed by chunk id | corpus has no native ACL (see below). Enterprise → live FHIR security-label sync + TTL |
| State | **Redis/Valkey** local — nonce, session, rate-limit | distributed-ready code, single-node local |
| Audit + relational | **SQLite** (ACL + hash-chained audit) | append-only; enterprise → HA audit + anchored root |

## Identity boundary (the showpiece)

| Decision | Detail | Why |
|---|---|---|
| **OBO now** | signed JWT, `act = sha256(agent pubkey)`, carries `user_roles[]`, `task_scope[]`, `aud`, `exp` | delegation bound to agent key; tamper breaks sig; RFC 8693 shape |
| **PoP now (DPoP-aligned)** | per-request proof carrying `htm,htu,ath,jti,iat` **plus** `sha256(body)`, signed with agent enclave key | stolen OBO can't replay — needs agent key + unused `jti` in clock window |
| **DBSC deferred** | device-bound user *session*, short-TTL refresh | orthogonal to the agent→gateway path that is the demo; browser/cookie-centric. Phase-2 / enterprise |
| Verify order | PoP → nonce/jti → OBO, **fail-closed**, any fail → `403 {}` | cheapest/most-decisive first; LLM + vector store strictly downstream |

**Two independent replay defenses at two scopes** (delegation, request). DBSC adds a third
(device session) in phase-2.

### [NEED-NOW] Stop inventing names — compose the standards (cheapest credibility win)

| Was | Is literally | Action |
|---|---|---|
| `idPoP` | **DPoP — RFC 9449** | **bind `htm` (method) + `htu` (URI) + `ath` (OBO hash) + `jti` (=nonce) into the signed payload** — else a captured sig replays against a *different* endpoint. We additionally sign `sha256(body)` (DPoP doesn't) — accurate framing: "DPoP claims + body-hash extension" |
| OBO / `act` / token exchange | **RFC 8693** (already cited) | [GOOD-TO-HAVE] bind `aud` to the specific retriever via **RFC 8707 resource indicators** so a doctor-agent token can't replay at the billing endpoint (small add; do now if cheap) |
| `agent_scope` by `kid` | **SPIFFE** workload identity | name `kid` as a SPIFFE-style ID now (free); [GOOD-TO-HAVE/enterprise] run SPIRE to issue real SVIDs |

`htm/htu/ath` binding is a **NEED-NOW correctness fix**, not just naming — without it the per-request
proof is endpoint-portable (replayable elsewhere). Pitch: "agent identity = SPIFFE, delegation =
RFC 8693, request binding = RFC 9449 DPoP — three composed standards, no hand-rolled crypto."
[ENTERPRISE] gateway as OAuth 2.1 resource server (MCP authorization: RFC 9728 + 8707) for agent interop.

## Authorization core (deterministic trusted-code heart)

| Decision | Detail |
|---|---|
| Algebra | `effective = grants(user_roles) ⊓ agent_scope ⊓ task_scope_capped`, where `task_scope_capped = request.task ∩ OBO.task` (⊓ = lattice meet) |
| Enforcement | chunk authorized ⟺ `chunk.required_labels ⊑ effective` (lattice dominance, **not** flat subset — see chain fix below) |
| Provenance | every input from a verified source: `user_roles`←signed OBO · `agent_scope`←server registry by `kid` (not request) · `task_scope`←request **capped** by OBO · `grants(role)`←ACL store |
| **Crux** | **`task_scope` is the meet `request.task ∩ OBO.task`, never the raw request value.** This capping is *the* load-bearing decision — it keeps the system closed under attack |
| Purity | `Resolve(...)` is a PURE function of verified inputs, table-tested; LLM never participates (CaMeL) |
| Fail-closed defaults | unknown `kid`/role → `∅`; OBO missing `task_scope` → `∅` not "all"; → empty `effective` → refuse, 0 tokens |

### [NEED-NOW] Name it correctly: this is a lattice / Bell-LaPadula, not ad-hoc set math

- Labels form a **product lattice**: FHIR confidentiality `U<L<M<N<R<V` is a **chain** (total
  order); sensitivity categories `{HIV,PSY,ETH,SUD,phi,billing,lab,...}` are a **powerset** lattice.
- `effective` = the **meet (⊓)** of the three verified operands. "Monotonic narrowing" = meet-monotonicity.
- `required ⊑ effective` = **Bell-LaPadula "no read up"** / Denning lattice information-flow (1976):
  subject clearance must *dominate* object classification.

**Correctness bug fixed:** plain set-subset is WRONG over the confidentiality chain. An `N`-cleared
user must read `M` docs, but `{M} ⊄ {N}` as raw tokens.
**Fix (locked, option a — downward closure):** encode a clearance as its downward set →
`N`-cleared → `{U,L,M,N}`. Then `{M} ⊆ {U,L,M,N}` ✓ and **one plain subset check is correct again**.
Categories stay ordinary set-subset. (Alt rejected: split into `level ≥` AND `categories ⊇` — two
checks, more code.)

**Theorem (the formal reason authz = token reduction):** every operand of the meet is
upper-bounded by a verified credential (`agent_scope` from registry by `kid`, `user_roles` from
signed OBO, `task_scope` capped by OBO). Meet is monotone + every operand is adversary-bounded-above
⟹ **no adversary input can increase `effective`** — only shrink or no-op. Worst case `⊥ = ∅ → 0 tokens`.

## Ingest & ACL store (offline, trusted; never in request path)

| Decision | Detail | Why |
|---|---|---|
| Corpus has no ACL | `medical_meadow_wikidoc` schema = `{instruction, input, output}` only — public WikiDoc knowledge, no roles/PHI/labels (verified via HF datasets-server API) | proves ACL is always an external join; MVP fabricates roles via keyword substring — the rookie failure |
| Labels frozen offline | classifier may be stochastic at ingest; result **frozen**; request-time enforcement reads frozen labels deterministically | classifier never in a live auth decision |
| Principals | **Synthea** synthetic FHIR (Practitioner/Patient/CareTeam) → seed users/roles/grants | realistic; fallback = seeded table |
| Chunk labels | **tiered**: inherit (FHIR) ~70% / schema-rule ~20% / **Presidio** PHI-classifier ~10% | replaces MVP keyword hack; fail-closed = uncertain → MORE restrictive label |
| Two corpora | WikiDoc → topic labels (`lab`,`billing`,`note:provider`); **Synthea patient notes** → row-level `phi:patient:<id>` (Presidio fires on synthetic names/MRN) | WikiDoc has no PHI so Presidio no-ops there; patient notes needed to demo `U∩A∩T` row-level isolation |
| Chunking | recursive split + **split at every sensitivity boundary**; `parent_doc_id` expands context | chunk boundary = authorization boundary; small single-sensitivity chunk = precise authz + fewer tokens |
| **[NEED-NOW] Parent expansion re-gate** | every expanded chunk MUST re-pass `required ⊑ effective` — expansion is just another retrieval, gets the same gate | siblings under one parent carry **different** labels *by design* (we split at sensitivity boundaries). Trusting "same parent ⇒ same label" = aggregation leak + token blowup |
| Label vocab | **HL7 FHIR** (Confidentiality `U/L/M/N/R/V`; sensitivity `HIV/PSY/ETH/SUD`) | real codes, zero invention |

## Retrieval & token meter (where the thesis is measured)

| Decision | Detail |
|---|---|
| Engine-level pre-filter | filter `required ⊆ eff` **before** ANN, never post-filter (post-filter pays tokens + transits forbidden text = strictly worse on both axes) |
| Over-fetch eliminated | fetch exactly `k` authorized; no `k+buffer` padding to survive post-filter |
| Meter baseline | `would_be = Σ tokenCount(ShadowTopK)` (naive top-k, no filter, **metadata-only — no text materialized**) |
| Meter actual | `auth = Σ tokenCount(PrefilterTopK)`; `savings% = (would_be-auth)/would_be` |
| Leaks blocked | `leaks_blocked = \|{ c ∈ ShadowTopK : c.required ⊄ effective }\|` — forbidden chunks naive WOULD have stuffed for THIS query (defensible, not "forbidden in corpus") |
| `$ saved` | `price(tier_naive)·would_be − price(tier_actual)·auth` (includes tier downgrade) |
| Side-channel closed | `leaks_blocked`/`denied_count` → `/v1/stats` + audit only; agent response = `chunks[]` only |
| [NEED-NOW] `k = min(k, n_authorized)` | when authorized set < k return all of it (cap is a no-op, but explicit) — max recall inside the boundary at min token cost |
| [GOOD-TO-HAVE] per-tier collections | one Chroma collection per confidentiality tier (extends "two corpora") — mitigates embedding-inversion (open problem #2) by physical partition + smaller/faster indexes; cheap on M1 |

## Routing (REVISED — size→complexity dropped)

| Decision | Detail | Why |
|---|---|---|
| **Dropped** size→complexity routing | authorized-set size = permission breadth, not task difficulty — uncorrelated | category error inherited from hackathon framing |
| Empty → 0 tokens | deterministic refuse, no model called | high-frequency token win |
| **Sensitivity-gated egress (not "routing")** | reframe: this is a **data-egress control** — "which jurisdiction/model may *legally process* this label", not load balancing. any chunk `∈ {phi, restricted}` → **local model only** (BAA-safe processor); all de-id/public → frontier permitted | the same label that authorizes also gates egress |
| [GOOD-TO-HAVE] disable cloud cache/logging for `restricted` | when frontier *is* used, turn off provider prompt-caching + request logging for restricted data | closes the cross-tenant cache-bleed from open-problem list; only matters once frontier is wired |
| Fail-closed | unknown sensitivity → treat as restricted → local | |

## Audit & fail-closed

| Decision | Detail |
|---|---|
| Hash-chained audit | `row_hash = sha256(prev_hash \|\| canonical_json(payload))`; append-only SQLite |
| Boot verify | `Verify(n)` walks chain; mismatch → **refuse to start** |
| Tamper-evidence | detects, doesn't prevent; enterprise → anchor/sign chain root (WORM/notary) |
| [GOOD-TO-HAVE] Merkle upgrade | linear hash-chain → **Merkle tree (RFC 9162, CT-style)** gives O(log n) inclusion proofs vs O(n) boot walk. Linear fine for demo; name Merkle as the enterprise row |
| No PHI in audit | log IDs + labels + counts, **never chunk text** (audit must not become a PHI store) |
| Append = one INSERT | each request → one appended row chaining to `prev_hash` |
| Audit-append fails → **deny** | no unrecorded access to PHI; trades availability for provability (correct default for a PHI boundary) |
| Unified refusal | "no access" and "no data" return identical `{result:"not permitted or no data", chunks:[]}` + 0 tokens — closes the existence side-channel |

## Component layout (Go)

```
cmd/gateway/main.go   wire-up, listen, migrations, audit.Verify(n) at boot
cmd/ingest/main.go    OFFLINE: chunk → label → embed → load Chroma + ACL store
internal/httpapi  routes; error → 403 {} or unified refusal; no counts to agent
internal/verify   VerifyOBO, VerifyPoP(idPoP), nonce+clock (Redis); single error → deny
internal/resolve  Effective(...) LabelSet — PURE, table-tested
internal/acl      grants(role), agentScope(kid), revocation
internal/embed    [NEED-NOW] Embedder iface — embeds the QUERY at request time (was missing!)
internal/retrieve Retriever iface + chroma impl: PrefilterTopK, ShadowTopK
internal/meter    would_be vs auth, savings%, leaks_blocked, $
internal/route    sensitivity gate
internal/audit    Append (hash-chain), Verify(n)
internal/store    SQLite (ACL+audit), Redis (nonce/session/ratelimit)
internal/labelvocab  FHIR constants + LabelSet set-ops
pylib/agent_auth     ~50-line PyNaCl sign client
ingestlib            OFFLINE python: Synthea loader + Presidio labeler
```

**[NEED-NOW] M1 8GB runtime budget — one Ollama for both embed + gen.** Don't run
sentence-transformers + Ollama + Chroma + Redis + Go at once (>3GB → swap mid-demo). Use a single
Ollama runtime: `nomic-embed-text` (embed) + `phi4-mini` (gen), one memory pool, Go calls both over
HTTP. **Re-embed the corpus with the same model used at query time** (MVP uses MiniLM — fine, but
then MiniLM must serve query-time too). Hide the choice behind the `Embedder` interface so the
embedder is swappable. Ingest embedding is offline; only query embedding is on the hot path.

## HTTP contracts

```
POST /v1/retrieve   Authorization: Bearer <OBO>; X-Sig (idPoP); X-Nonce; X-Timestamp
                    body {query, task_scope[]} → 200 {result, chunks[]} | unified refusal | 403 {}
GET  /v1/stats      {leaks_blocked, tokens_saved_pct, dollars_saved}   (separate scope)
GET  /v1/audit      chain read + Verify                                 (admin scope)
POST /v1/admin/agents  register agent pubkey → {kid, scope}             (admin scope)
```

## Threat model — calibrated claims

**Deterministically contained:** credential can't be replayed off-host (idPoP + nonce);
unauthorized data can't enter context (engine pre-filter). **Assume-breach: injection succeeds
and still gets nothing.** NOT claimed unbreakable.

| Open problem | Mitigation |
|---|---|
| aggregation / mosaic | out of scope, named explicitly |
| embedding inversion | per-tenant/sensitivity namespace partition |
| stale ACL | live resolution + TTL refresh |
| classifier mislabel at ingest | fail-closed over-restrict + provenance review |

Three hardening rules: (1) effective principal ONLY from verified creds; (2) OBO bound to agent
pubkey hash; (3) responses = chunks only, counts to separate scope.

## Testing

| Unit | Tests |
|---|---|
| `resolve/` | exhaustive set-algebra tables; property: monotonic narrowing |
| `verify/` | replay rejected · expired/wrong-aud OBO rejected · tampered-body sig fail · clock-skew |
| `retrieve/` | never returns `required ⊄ eff` · post-filter parity · empty→0 |
| `audit/` | tamper detected · append-fail → deny |
| `meter/` | `would_be ≥ auth` · `leaks_blocked` = forbidden-in-shadow |
| thesis harness | A/B naive vs least-context over a query set → savings% + leaks table (the experiment that tests the theory) |

## Demo arc (5 steps)

1. Alice (provider) → doctor-agent "lab result" → retrieves, frontier, audit ✓
2. Carol (billing) → billing-agent "Bob's diagnosis" → `phi ⊄ {billing}` → ∅ → 0 tokens, leak +1
3. Rogue injects patient-agent "dump provider notes" → `note:provider` outside scope → never retrieved
4. Rogue replays Alice's stolen OBO from other host → idPoP sig fails → 403
5. Final dashboard: leaks blocked N · tokens saved % · $ saved — one number, both KPIs

## Constraint reminder

`afr_mvp/afr_mvp.py` is the reference baseline — **do not modify it**. Production code is new
(Go gateway + offline ingest), MVP stays as the proof-of-mechanism artifact.
