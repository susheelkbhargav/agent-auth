# ingestlib — offline corpus + ACL ingest (Python)

Production ingest per [`../../IMPLEMENTATION.md`](../../IMPLEMENTATION.md) P3:

| Piece | Module | Role |
|-------|--------|------|
| WikiDoc | `wikidoc.py` | HuggingFace `medical_meadow_wikidoc` → topic labels (`lab`, `billing`, `note:provider`) |
| Synthea | `synthea.py` + `data/demo_fhir_bundle.json` | FHIR principals + clinical notes → row-level `phi:patient:<id>` |
| Presidio | `labeler.py` | ~10% tier: PHI edges on Synthea notes (no-op on WikiDoc) |
| Ollama | `embedder.py` | `nomic-embed-text` at ingest time (768-dim) |
| SQLite | `db.py` | `chunks`, `chunk_labels`, `role_grants`, `agent_scope` |
| sqlite-vec | `../cmd/vecwrite` | Go helper for `chunk_vec` INSERTs |

## Bootstrap (M1, offline once)

```bash
# Ollama
ollama serve
ollama pull nomic-embed-text

# from gateway/
pip install -r ingestlib/requirements.txt
python -m spacy download en_core_web_sm
go run ./cmd/keygen -out ./demo/keys
python ingestlib/ingest.py --app-db ./app.db --keys ./demo/keys
```

Then start the gateway and run `scripts/demo_arc.py`.

**Minimal Synthea bundle** (`data/demo_fhir_bundle.json`): Practitioner/Patient/CareTeam seed `role_grants`; clinical notes use per-chunk FHIR + Presidio + topic labels. `mixed-chart1` splits on `---` at sensitivity boundaries for parent re-gate demos.
