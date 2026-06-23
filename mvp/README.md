# MVP — Proof of Mechanism

`afr_mvp.py` is the **frozen** proof-of-mechanism. **Do not modify it.** It demonstrates the
load-bearing fact behind the thesis: a metadata pre-filter applied *before* vector scoring means
forbidden chunks never enter the candidate set — so an empty authorized set costs **0 LLM tokens**.

## What it does

1. Loads 50 records of `medalpaca/medical_meadow_wikidoc` (public medical encyclopedia text).
2. Chunks them, assigns a single role per chunk by keyword (`billing_admin` / `doctor` / `general_staff`).
3. Embeds with `all-MiniLM-L6-v2`, stores in a local ChromaDB at `./afr_local_db`.
4. For each role, runs an authorized + an unauthorized query, pre-filtering with
   `where={"allowed_role": role}` **before** ANN scoring.
5. 0 chunks → deterministic refusal (0 tokens); else → local Phi-4-mini via Ollama.

> Note: identity here is an unverified string and tokens are not metered — those gaps are exactly
> what the production `gateway/` build addresses. See [`../DECISION.md`](../DECISION.md).

## Prerequisites

- **Python 3.10+**
- **[Ollama](https://ollama.com)** running locally with the `phi4-mini` model:
  ```bash
  ollama serve            # must be listening on localhost:11434
  ollama pull phi4-mini
  ```

## Install

```bash
cd mvp
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
```

First run also downloads (cached after): the `all-MiniLM-L6-v2` embedding model (~90 MB) and the
medical dataset (50 records).

## Run

```bash
# from inside mvp/ — the script writes its DB to ./afr_local_db
python3 afr_mvp.py
```

You'll see, per role, an authorized query answered via Phi-4-mini and an unauthorized query
refused at the database level with **0 tokens consumed**.

## Dependencies (derived from the file's imports)

| Import | pip package | Purpose |
|---|---|---|
| `chromadb` | `chromadb` | local vector store + engine-level metadata pre-filter |
| `sentence_transformers` | `sentence-transformers` | `all-MiniLM-L6-v2` embeddings |
| `langchain_text_splitters` | `langchain-text-splitters` | recursive chunking |
| `datasets` | `datasets` | load the HuggingFace medical corpus |
| `requests` | `requests` | call the Ollama HTTP API |
| `os` | stdlib | DB path |
| — (HTTP) | **Ollama** (`phi4-mini`) | local generation at `localhost:11434` |

`./afr_local_db/` is regenerated on each run (cleared + re-added) and is gitignored.
