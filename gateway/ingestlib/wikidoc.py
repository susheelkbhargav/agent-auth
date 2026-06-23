from __future__ import annotations

from ingestlib.chunker import estimate_tokens, split_text
from ingestlib.labeler import wikidoc_topic_labels
from ingestlib.models import ChunkRecord, IngestBatch


def load_wikidoc(limit: int = 50) -> IngestBatch:
    from datasets import load_dataset

    batch = IngestBatch()
    ds = load_dataset("medalpaca/medical_meadow_wikidoc", split=f"train[:{limit}]")
    for i, row in enumerate(ds):
        raw = f"{row.get('input', '')} {row.get('output', '')}".strip()
        if not raw:
            continue
        parent = f"wikidoc-{i}"
        topic_labels = wikidoc_topic_labels(raw)
        for j, piece in enumerate(split_text(raw)):
            batch.chunks.append(
                ChunkRecord(
                    chunk_id=f"{parent}-c{j}",
                    text=piece,
                    parent_doc_id=parent,
                    labels=list(topic_labels),
                    token_count=estimate_tokens(piece),
                    corpus="wikidoc",
                )
            )
    return batch
