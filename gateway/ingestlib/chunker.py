from __future__ import annotations


def estimate_tokens(text: str) -> int:
    words = len(text.split())
    return max(1, int(words * 1.3))


def split_text(text: str, chunk_size: int = 400, overlap: int = 50) -> list[str]:
    """Recursive split mirroring MVP chunking (paragraph → line → sentence → word)."""
    text = text.strip()
    if not text:
        return []
    if len(text) <= chunk_size:
        return [text]

    separators = ["\n\n", "\n", ". ", " "]
    for sep in separators:
        if sep not in text:
            continue
        parts = text.split(sep)
        chunks: list[str] = []
        buf = ""
        for part in parts:
            piece = part if sep == " " else part + sep.rstrip()
            if not piece.strip():
                continue
            candidate = (buf + piece).strip()
            if len(candidate) <= chunk_size:
                buf = candidate + (" " if sep == " " else "")
                continue
            if buf.strip():
                chunks.extend(split_text(buf.strip(), chunk_size, overlap))
            buf = piece
        if buf.strip():
            chunks.extend(split_text(buf.strip(), chunk_size, overlap))
        if chunks:
            return _merge_overlap(chunks, overlap)
    return [text[:chunk_size]]


def _merge_overlap(chunks: list[str], overlap: int) -> list[str]:
    if overlap <= 0 or len(chunks) <= 1:
        return chunks
    out = [chunks[0]]
    for chunk in chunks[1:]:
        prev = out[-1]
        tail = prev[-overlap:] if len(prev) > overlap else prev
        out.append((tail + " " + chunk).strip())
    return out
