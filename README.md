# agent-auth

**Principle of Least Context** — authorization as a token-reduction primitive. One
pre-retrieval authz filter minimizes both the attack surface (lethal-trifecta data leg)
and cost (context tokens). Getting more secure makes you cheaper.

> The forbidden chunk you don't retrieve is the leak you don't suffer **and** the token you don't pay.

## Repo layout

| Path | What | Status |
|---|---|---|
| [`DESIGN.md`](DESIGN.md) | original thesis + hackathon design | reference |
| [`DECISION.md`](DECISION.md) | production design decisions (all axes), triaged | reference |
| [`mvp/`](mvp/) | **proof-of-mechanism** — Python RAG with engine-level pre-filter | working, frozen |
| [`gateway/`](gateway/) | **production build** — Go identity gateway per `DECISION.md` | P0–P6 wired |

## The two halves

- **`mvp/`** proves the *mechanism*: ChromaDB `where={}` pre-filters before ANN scoring,
  empty authorized set → 0 LLM tokens. It does **not** verify identity (uses a trusted
  string) and does **not** measure tokens. See [`mvp/README.md`](mvp/README.md) to run it.
- **`gateway/`** is the production design: cryptographic identity boundary (OBO + DPoP),
  deterministic `U ∩ A ∩ T` authz in trusted code, engine-level pre-filter, token meter,
  hash-chained audit, fail-closed. See [`gateway/README.md`](gateway/README.md).

Start with `DECISION.md` for the full rationale.
