#!/usr/bin/env python3
"""Offline ingest: WikiDoc + Synthea + Presidio labels, Ollama embed, app.db writes.

Python owns chunks + chunk_labels + ACL seeding. Go cmd/vecwrite owns chunk_vec INSERTs
(sqlite-vec extension). Run once offline before starting the gateway.
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

# Allow `python ingestlib/ingest.py` from gateway/ without installing the package.
_PKG = Path(__file__).resolve().parent
if str(_PKG.parent) not in sys.path:
    sys.path.insert(0, str(_PKG.parent))

from ingestlib.db import seed_agent_scopes, write_sqlite  # noqa: E402
from ingestlib.embedder import OllamaEmbedder  # noqa: E402
from ingestlib.models import IngestBatch  # noqa: E402
from ingestlib.synthea import load_synthea  # noqa: E402
from ingestlib.wikidoc import load_wikidoc  # noqa: E402


def _load_presidio():
    try:
        from presidio_analyzer import AnalyzerEngine

        return AnalyzerEngine()
    except Exception as exc:  # pragma: no cover - optional at dev time
        print(f"warning: Presidio unavailable ({exc}); schema/FHIR labels only", file=sys.stderr)
        return None


def _run_go(cmd: list[str], cwd: Path) -> None:
    proc = subprocess.run(cmd, cwd=cwd, check=False)
    if proc.returncode != 0:
        raise SystemExit(proc.returncode)


def main() -> int:
    p = argparse.ArgumentParser(description="agent-auth offline ingest (WikiDoc + Synthea + Presidio)")
    p.add_argument("--app-db", default="./app.db")
    p.add_argument("--ollama", default="http://127.0.0.1:11434")
    p.add_argument("--embed-model", default="nomic-embed-text")
    p.add_argument("--keys", default="./demo/keys", help="demo agent pub keys for agent_scope seed")
    p.add_argument("--wikidoc-limit", type=int, default=50)
    p.add_argument("--fhir-bundle", default="", help="optional Synthea FHIR bundle JSON path")
    p.add_argument("--gateway-root", default=str(Path(__file__).resolve().parents[1]))
    p.add_argument("--skip-embed", action="store_true", help="reuse existing vectors file")
    p.add_argument("--vectors-file", default="", help="NDJSON path (default: <app-db>.vectors.ndjson)")
    args = p.parse_args()

    gw = Path(args.gateway_root)
    app_db = Path(args.app_db)
    keys_dir = Path(args.keys)
    vectors = Path(args.vectors_file) if args.vectors_file else app_db.with_suffix(".vectors.ndjson")
    bundle = Path(args.fhir_bundle) if args.fhir_bundle else None

    print("==> dbinit (Go migrations + sqlite-vec)")
    _run_go(["go", "run", "./cmd/dbinit", "-app-db", str(app_db)], gw)

    print("==> load WikiDoc (HuggingFace medical_meadow_wikidoc)")
    batch = IngestBatch()
    batch.chunks.extend(load_wikidoc(args.wikidoc_limit).chunks)

    print("==> load Synthea FHIR + Presidio labeling")
    presidio = _load_presidio()
    synthea = load_synthea(bundle, presidio)
    batch.chunks.extend(synthea.chunks)
    batch.role_grants.extend(synthea.role_grants)

    print(f"==> write corpus + ACL ({len(batch.chunks)} chunks, {len(batch.role_grants)} grants)")
    write_sqlite(app_db, batch)
    if keys_dir.exists():
        seed_agent_scopes(app_db, keys_dir)
    else:
        print(f"warning: keys dir missing ({keys_dir}); agent_scope not seeded", file=sys.stderr)

    if not args.skip_embed:
        print(f"==> Ollama embed ({args.embed_model})")
        embedder = OllamaEmbedder(args.ollama, args.embed_model)
        n = embedder.embed_batch_ndjson(batch.chunks, vectors)
        print(f"    wrote {n} vectors to {vectors}")
    elif not vectors.exists():
        print(f"error: --skip-embed but {vectors} missing", file=sys.stderr)
        return 1

    print("==> vecwrite (Go sqlite-vec INSERTs)")
    _run_go(
        ["go", "run", "./cmd/vecwrite", "-app-db", str(app_db), "-in", str(vectors)],
        gw,
    )
    print(f"ingest complete: {app_db} ({len(batch.chunks)} chunks)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
