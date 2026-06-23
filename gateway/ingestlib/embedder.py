from __future__ import annotations

import json
from pathlib import Path

import requests


class OllamaEmbedder:
    def __init__(self, base_url: str = "http://127.0.0.1:11434", model: str = "nomic-embed-text"):
        self.base_url = base_url.rstrip("/")
        self.model = model

    def embed(self, text: str) -> list[float]:
        resp = requests.post(
            f"{self.base_url}/api/embeddings",
            json={"model": self.model, "prompt": text},
            timeout=120,
        )
        resp.raise_for_status()
        data = resp.json()
        return data["embedding"]

    def embed_batch_ndjson(self, chunks, out_path: Path) -> int:
        out_path.parent.mkdir(parents=True, exist_ok=True)
        n = 0
        with out_path.open("w", encoding="utf-8") as fh:
            for chunk in chunks:
                vec = self.embed(chunk.text)
                rec = {"chunk_id": chunk.chunk_id, "embedding": vec}
                fh.write(json.dumps(rec) + "\n")
                n += 1
        return n
