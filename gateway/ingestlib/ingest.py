#!/usr/bin/env python3
"""Offline ingest: seeds demo corpus via Go cmd/ingest (sqlite-vec writes).

Optional WikiDoc fetch can extend this script; demo path uses bundled seed chunks.
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path


def main() -> int:
    p = argparse.ArgumentParser(description="agent-auth ingestlib")
    p.add_argument("--app-db", default="./app.db")
    p.add_argument("--ollama", default="http://127.0.0.1:11434")
    p.add_argument("--keys", default="./demo/keys")
    p.add_argument("--gateway-root", default=str(Path(__file__).resolve().parents[1]))
    args = p.parse_args()

    cmd = [
        "go", "run", "./cmd/ingest",
        "-app-db", args.app_db,
        "-ollama", args.ollama,
        "-keys", args.keys,
    ]
    print("running:", " ".join(cmd))
    return subprocess.call(cmd, cwd=args.gateway_root)


if __name__ == "__main__":
    sys.exit(main())
