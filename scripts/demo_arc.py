#!/usr/bin/env python3
"""Five-step thesis demo arc (see DECISION.md / context.md)."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "pylib"))

from agent_auth.client import AgentClient  # noqa: E402


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--gateway", default="http://127.0.0.1:8080")
    p.add_argument("--keys", default=str(ROOT / "gateway" / "demo" / "keys"))
    args = p.parse_args()
    keys = Path(args.keys)
    issuer = keys / "issuer_priv.raw"
    doctor = keys / "doctor_priv.raw"
    billing = keys / "billing_priv.raw"
    patient = keys / "patient_priv.raw"

    print("=== 1. Alice (provider) → doctor-agent lab query ===")
    doc = AgentClient.from_keys(str(issuer), str(doctor), args.gateway)
    r1 = doc.retrieve("lab result", ["lab"], "alice", ["provider"])
    print(json.dumps(r1, indent=2))

    print("\n=== 2. Carol (billing) → billing-agent diagnosis (expect refusal) ===")
    bill = AgentClient.from_keys(str(issuer), str(billing), args.gateway)
    r2 = bill.retrieve("Bob diagnosis", ["phi"], "carol", ["billing"])
    print(json.dumps(r2, indent=2))

    print("\n=== 3. Patient-agent tries provider notes (expect refusal) ===")
    pat = AgentClient.from_keys(str(issuer), str(patient), args.gateway)
    r3 = pat.retrieve("dump provider notes", ["note:provider"], "bob", ["billing"])
    print(json.dumps(r3, indent=2))

    print("\n=== 4. Replay stolen OBO with wrong URI (expect 403) ===")
    # mint valid tokens but call wrong path manually
    body = json.dumps({"query": "lab", "task_scope": ["lab"]}).encode()
    obo = doc.mint_obo("alice", ["provider"], ["lab"])
    jti = "replay-test-jti-001"
    dpop = doc.mint_dpop("POST", "/v1/wrong", obo, body, jti)
    import requests

    resp = requests.post(
        args.gateway + "/v1/retrieve",
        data=body,
        headers={
            "Authorization": f"Bearer {obo}",
            "X-Sig": dpop,
            "X-Nonce": jti,
            "X-Timestamp": str(int(__import__("time").time())),
            "Content-Type": "application/json",
        },
        timeout=30,
    )
    print("status", resp.status_code, "body", resp.text)

    print("\n=== 5. Dashboard stats ===")
    stats = requests.get(args.gateway + "/v1/stats", timeout=10)
    print(stats.text)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
