from __future__ import annotations

import base64
import json
from pathlib import Path

from ingestlib.chunker import estimate_tokens, split_at_boundaries
from ingestlib.labeler import merge_labels, presidio_phi_labels, synthea_base_labels, wikidoc_topic_labels
from ingestlib.models import ChunkRecord, IngestBatch, RoleGrant

_DATA = Path(__file__).resolve().parent / "data" / "demo_fhir_bundle.json"

# External join: FHIR gives role names; policy maps role → granted labels (EHR ACL export).
_ROLE_GRANT_POLICY: dict[str, list[str]] = {
    "provider": ["phi", "prescription", "lab", "note:provider"],
    "billing": ["billing", "scheduling"],
    "patient": ["phi:patient:bob", "conf:R"],
}


def load_synthea(bundle_path: Path | None = None, presidio_analyzer=None) -> IngestBatch:
    path = bundle_path or _DATA
    bundle = json.loads(path.read_text())
    batch = IngestBatch()
    batch.role_grants.extend(_role_grants_from_bundle(bundle))

    for entry in bundle.get("entry", []):
        resource = entry.get("resource", {})
        rtype = resource.get("resourceType")
        if rtype == "DocumentReference":
            batch.chunks.extend(_chunks_from_document(resource, presidio_analyzer))
    return batch


def _roles_from_bundle(bundle: dict) -> set[str]:
    roles: set[str] = set()
    for entry in bundle.get("entry", []):
        resource = entry.get("resource", {})
        rtype = resource.get("resourceType")
        if rtype == "Practitioner":
            for qual in resource.get("qualification", []):
                role = qual.get("code", {}).get("text", "").strip().lower()
                if role:
                    roles.add(role)
        elif rtype == "CareTeam":
            for participant in resource.get("participant", []):
                for role_obj in participant.get("role", []):
                    role = role_obj.get("text", "").strip().lower()
                    if role:
                        roles.add(role)
        elif rtype == "Patient":
            roles.add("patient")
    return roles


def _role_grants_from_bundle(bundle: dict) -> list[RoleGrant]:
    grants: list[RoleGrant] = []
    for role in sorted(_roles_from_bundle(bundle)):
        for label in _ROLE_GRANT_POLICY.get(role, []):
            grants.append(RoleGrant(role=role, label=label))
    return grants


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
        parent = f"synthea-{doc_id}"
        # Per-chunk labels (split at sensitivity boundaries within the same parent_doc_id).
        for i, piece in enumerate(split_at_boundaries(text)):
            base = synthea_base_labels(piece, patient_id, fhir_codes)
            presidio = presidio_phi_labels(piece, patient_id, presidio_analyzer)
            labels = merge_labels(base, presidio)
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
