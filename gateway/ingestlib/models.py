from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class ChunkRecord:
    chunk_id: str
    text: str
    parent_doc_id: str
    labels: list[str]
    token_count: int
    corpus: str  # wikidoc | synthea


@dataclass
class RoleGrant:
    role: str
    label: str


@dataclass
class IngestBatch:
    chunks: list[ChunkRecord] = field(default_factory=list)
    role_grants: list[RoleGrant] = field(default_factory=list)
