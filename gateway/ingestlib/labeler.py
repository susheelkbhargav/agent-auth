from __future__ import annotations

import re
from typing import Iterable

# WikiDoc topic labels (schema-rule tier ~100% for public corpus; Presidio no-ops).
_WIKIDOC_RULES: list[tuple[str, tuple[str, ...]]] = [
    ("billing", ("billing", "insurance", "copay", "claim", "cost", "reimbursement", "payment")),
    ("note:provider", ("provider note", "private note", "clinician note", "provider-only")),
    ("lab", ("lab", "laboratory", "glucose", "blood count", "hemoglobin", "diagnostic test", "specimen")),
]

# Synthea schema-rule tier (~20%): document type → base labels before Presidio.
_SYNTHEA_DOC_RULES: list[tuple[str, tuple[str, ...]]] = [
    ("note:provider", ("provider note", "private note", "provider-only")),
    ("phi", ("diagnosis", "assessment", "condition", "prescription", "medication")),
]

_FHIR_CONF_MAP = {
    "U": "conf:U",
    "L": "conf:L",
    "M": "conf:M",
    "N": "conf:N",
    "R": "conf:R",
    "V": "conf:V",
}


def wikidoc_topic_labels(text: str) -> list[str]:
    lower = text.lower()
    for label, keywords in _WIKIDOC_RULES:
        if any(k in lower for k in keywords):
            return [label]
    # Default public clinical reference material to lab (sparse-regime diversity).
    return ["lab"]


def fhir_security_labels(codes: Iterable[str]) -> list[str]:
    out: list[str] = []
    for code in codes:
        key = code.strip().upper()
        if key in _FHIR_CONF_MAP:
            out.append(_FHIR_CONF_MAP[key])
    return out


def synthea_base_labels(text: str, patient_id: str, fhir_codes: Iterable[str]) -> list[str]:
    """Tiered labeling: FHIR inherit (~70%) + schema rules (~20%). Presidio adds ~10% later."""
    labels = set(fhir_security_labels(fhir_codes))
    lower = text.lower()
    for label, keywords in _SYNTHEA_DOC_RULES:
        if any(k in lower for k in keywords):
            labels.add(label)
    if patient_id:
        slug = _patient_slug(patient_id)
        labels.add(f"phi:patient:{slug}")
    if not labels:
        labels.add("phi")
    return sorted(labels)


def presidio_phi_labels(text: str, patient_id: str, analyzer) -> list[str]:
    """Presidio tier: row-level phi:patient when PERSON entities match patient name tokens."""
    if analyzer is None:
        return []
    slug = _patient_slug(patient_id)
    patient_tokens = set(re.split(r"[\W_]+", slug.lower()))
    try:
        results = analyzer.analyze(text=text, language="en")
    except Exception:
        return []
    extra: set[str] = set()
    for result in results:
        if result.entity_type != "PERSON":
            continue
        span = text[result.start : result.end].lower()
        tokens = set(re.split(r"[\W_]+", span))
        if tokens & patient_tokens:
            extra.add(f"phi:patient:{slug}")
        else:
            extra.add("phi")
    return sorted(extra)


def merge_labels(*groups: Iterable[str]) -> list[str]:
    merged: set[str] = set()
    for group in groups:
        merged.update(group)
    return sorted(merged)


def _patient_slug(patient_id: str) -> str:
    return patient_id.lower().replace("patient/", "").replace("patient-", "")
