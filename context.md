# Agent-Auth — Session Context

Handoff for new sessions. **Design of record:** [`IMPLEMENTATION.md`](IMPLEMENTATION.md).

## Thesis

One pre-retrieval authz filter blocks leaks **and** prunes context tokens. Measure via
`leaks_blocked`, `tokens_saved_pct`, `dollars_saved` (`ShadowTopK` vs `PrefilterTopK`).

**Target:** Mac M1 8 GB — one Ollama, no Redis, sqlite-vec, in-memory nonce.

## Workflow

- Branch: **`main`**, commit + push when done.
- Do **not** modify [`mvp/afr_mvp.py`](mvp/afr_mvp.py).

## Build

```bash
cd gateway && CGO_ENABLED=1 go test ./... && CGO_ENABLED=1 go build ./...
```

## Bootstrap (first run on M1)

```bash
# Terminal 1
ollama serve
ollama pull nomic-embed-text phi4-mini

# Terminal 2
cd gateway
pip install -r ingestlib/requirements.txt
python -m spacy download en_core_web_sm
go run ./cmd/keygen -out ./demo/keys
python ingestlib/ingest.py --app-db ./app.db --keys ./demo/keys
export ISSUER_PUBKEY_PATH=./demo/keys/issuer_pub.raw
export APP_DB=./app.db
go run ./cmd/gateway

# Terminal 3
pip install -r gateway/pylib/agent_auth/requirements.txt
python scripts/demo_arc.py --gateway http://127.0.0.1:8080
```

## Implemented

| Phase | Status |
|-------|--------|
| P0 | sqlite-vec, PrefilterTopK, ShadowTopK |
| P1 | StandardVerifier, MemNonceStore, pylib signer |
| P2 | acl SQLite, grants union, revocation |
| P3 | ingestlib: WikiDoc + Synthea + Presidio + Ollama; Go vecwrite |
| P4 | Ollama embed/gen, stats counters |
| P5 | HTTP routes, audit hash-chain |
| P6 | scripts/demo_arc.py |

## Stretch / not full DECISION scope

- B2 post-filter harness, parent-doc re-gate, Anthropic frontier
- Larger Synthea bundles / live HF pulls beyond M1 demo limits

## Env

`ISSUER_PUBKEY_PATH` (required), `APP_DB`, `OBO_AUD`, `OLLAMA_URL`, `LOCAL_MODEL`, `FRONTIER_MODEL`

## Last stop

End-to-end gateway wired. Run bootstrap + demo_arc on M1 with Ollama.
