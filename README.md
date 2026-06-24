# agent-auth

**Principle of Least Context** — authorization as a token-reduction primitive. One
pre-retrieval authz filter minimizes both the attack surface (lethal-trifecta data leg)
and cost (context tokens). Getting more secure makes you cheaper.

> The forbidden chunk you don't retrieve is the leak you don't suffer **and** the token you don't pay.

## Repo layout

| Path | What |
|---|---|---|
| [`mvp/`](mvp/) | **proof-of-mechanism** — Python RAG with engine-level pre-filter 
| [`gateway/`](gateway/) | **production build** — Go identity gateway |

## The two halves

- **`mvp/`** proves the *mechanism*: ChromaDB `where={}` pre-filters before ANN scoring,
  empty authorized set → 0 LLM tokens. It does **not** verify identity (uses a trusted
  string) and does **not** measure tokens. See [`mvp/README.md`](mvp/README.md) to run it.
- **`gateway/`** is the production design: cryptographic identity boundary (OBO + DPoP),
  deterministic `U ∩ A ∩ T` authz in trusted code, engine-level pre-filter, token meter,
  hash-chained audit, fail-closed. See [`gateway/README.md`](gateway/README.md).


## Running Locally

### Prerequisites

Before running, ensure you have the following installed and running:

* **[Go 1.25+](https://go.dev/doc/install)** (with CGO enabled, as the `sqlite-vec` extension requires it)
* **[Python & pip](https://www.python.org/downloads/)** (for the ingest scripts)
* **[Ollama](https://ollama.com/)** running locally on your machine (the ingest script defaults to `http://127.0.0.1:11434`)

### Automated Bootstrap (Recommended)

You can run the entire setup (keys, data ingest, gateway start) in one go using the bootstrap script from the root of the repository:

```bash
./scripts/bootstrap.sh
```

### Manual Setup

If you prefer to run the steps individually, run them from the `gateway` directory:

1. **Generate Keys:**
   ```bash
   cd gateway
   go run ./cmd/keygen -out ./demo/keys
   ```

2. **Install Ingest Dependencies:**
   ```bash
   pip install -r ingestlib/requirements.txt
   python -m spacy download en_core_web_sm
   ```

3. **Run Data Ingest:**
   ```bash
   python ingestlib/ingest.py --app-db ./app.db --keys ./demo/keys --ollama "http://127.0.0.1:11434"
   ```

4. **Start the Gateway:**
   ```bash
   export ISSUER_PUBKEY_PATH=./demo/keys/issuer_pub.raw
   export APP_DB=./app.db
   CGO_ENABLED=1 go run ./cmd/gateway
   ```

