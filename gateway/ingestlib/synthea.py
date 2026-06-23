from __future__ import annotations

import base64
import json
from pathlib import Path

from ingestlib.chunker import estimate_tokens, split_text
from ingestlib.labeler import merge_labels, presidio_phi_labels, synthea_base_labels
from ingestlib.models import ChunkRecord, IngestBatch, RoleGrant

_DATA = Path(__file__).resolve().parent / "data" / "demo_fhir_bundle.json"

# Demo arc role grants (Synthea-derived; matches cmd/keygen demo agents).
_ROLE_LABELS: dict[str, list[str]] = {
    "provider": ["phi", "prescription", "lab", "note:provider"],
    "billing": ["billing", "scheduling"],
}


def load_synthea(bundle_path: Path | None = None, presidio_analyzer=None) -> IngestBatch:
    path = bundle_path or _DATA
    bundle = json.loads(path.read_text())
    batch = IngestBatch()

    for role, labels in _ROLE_LABELS.items():
        for label in labels:
            batch.role_grants.append(RoleGrant(role=role, label=label))

    for entry in bundle.get("entry", []):
        resource = entry.get("resource", {})
        rtype = resource.get("resourceType")
        if rtype == "DocumentReference":
            batch.chunks.extend(
                _chunks_from_document(resource, presidio_analyzer)
            )
    return batch


def _chunks_from_document(doc: dict, presidio_analyzer) -> list[ChunkRecord]:
    doc_id = doc.get("id", "doc")
    subject = doc.get("subject", {}).get("reference", "")
    patient_id = subject.split("/")[-1] if subject else ""
    fhir_codes = [s.get("code", "") for s in doc.get("meta", {}).get("security", [])]

    out: list[ChunkRecord] = []
    for content in doc.get("content", []):
        attachment = content.get("attachment", {})
        raw_b64 = attachment.get("data", "")
        if not raw_b64:
            continue
        text = base64.b64decode(raw_b64).decode("utf-8")
        base = synthea_base_labels(text, patient_id, fhir_codes)
        presidio = presidio_phi_labels(text, patient_id, presidio_analyzer)
        labels = merge_labels(base, presidio)
        parent = f"synthea-{doc_id}"
        for i, piece in enumerate(split_text(text)):
            out.append(
                ChunkRecord(
                    chunk_id=f"{parent}-c{i}",
                    text=piece,
                    parent_doc_id=parent,
                    labels=labels,
                    token_count=estimate_tokens(piece),
                    corpus="synthea",
                )
            )
    return out
