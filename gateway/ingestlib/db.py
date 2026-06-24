from __future__ import annotations

import hashlib
import sqlite3
from pathlib import Path

from ingestlib.models import ChunkRecord, IngestBatch, RoleGrant


def write_sqlite(app_db: Path, batch: IngestBatch) -> None:
    conn = sqlite3.connect(app_db)
    try:
        conn.execute("PRAGMA foreign_keys = ON")
        _clear_corpus(conn)
        _write_role_grants(conn, batch.role_grants)
        for chunk in batch.chunks:
            _insert_chunk(conn, chunk)
        conn.commit()
    finally:
        conn.close()


def seed_agent_scopes(app_db: Path, keys_dir: Path) -> None:
    """Register demo agent scopes from Ed25519 pub keys (same as legacy Go ingest)."""
    scopes = {
        "doctor": ["phi", "prescription", "lab", "note:provider", "scheduling", "phi:patient:bob", "conf:R", "conf:V"],
        "billing": ["billing", "scheduling"],
        "patient": ["phi:patient:bob", "scheduling", "billing:patient:bob", "conf:R"],
    }
    conn = sqlite3.connect(app_db)
    try:
        conn.execute("PRAGMA foreign_keys = ON")
        for agent, labels in scopes.items():
            pub_path = keys_dir / f"{agent}_pub.raw"
            kid = _kid_from_pub(pub_path.read_bytes())
            conn.execute("DELETE FROM agent_scope WHERE kid = ?", (kid,))
            for label in labels:
                conn.execute(
                    "INSERT INTO agent_scope (kid, label) VALUES (?, ?)",
                    (kid, label),
                )
        conn.commit()
    finally:
        conn.close()


def _clear_corpus(conn: sqlite3.Connection) -> None:
    conn.execute("DELETE FROM chunk_labels")
    conn.execute("DELETE FROM chunks")


def _write_role_grants(conn: sqlite3.Connection, grants: list[RoleGrant]) -> None:
    conn.execute("DELETE FROM role_grants")
    for grant in grants:
        conn.execute(
            "INSERT OR IGNORE INTO role_grants (role, label) VALUES (?, ?)",
            (grant.role, grant.label),
        )


def _insert_chunk(conn: sqlite3.Connection, chunk: ChunkRecord) -> None:
    conn.execute(
        """
        INSERT OR REPLACE INTO chunks (id, text, parent_doc_id, token_count, corpus)
        VALUES (?, ?, ?, ?, ?)
        """,
        (chunk.chunk_id, chunk.text, chunk.parent_doc_id, chunk.token_count, chunk.corpus),
    )
    conn.execute("DELETE FROM chunk_labels WHERE chunk_id = ?", (chunk.chunk_id,))
    for label in chunk.labels:
        conn.execute(
            "INSERT INTO chunk_labels (chunk_id, label) VALUES (?, ?)",
            (chunk.chunk_id, label),
        )


def _kid_from_pub(pub: bytes) -> str:
    return hashlib.sha256(pub).hexdigest()
