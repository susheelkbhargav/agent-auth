#!/usr/bin/env python3
"""Comprehensive demo arc for all agents."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "gateway" / "pylib"))

from agent_auth.client import AgentClient  # noqa: E402


def check_403(agent_client: AgentClient, user: str, roles: list[str], scope: list[str], gateway_url: str):
    """Test 403 Forbidden by replaying an OBO token to the wrong URI."""
    body = json.dumps({"query": "test", "task_scope": scope}).encode()
    obo = agent_client.mint_obo(user, roles, scope)
    jti = "replay-test-" + user
    dpop = agent_client.mint_dpop("POST", "/v1/wrong", obo, body, jti)
    import requests
    resp = requests.post(
        gateway_url + "/v1/retrieve",
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
    return resp.status_code, resp.text


def print_case(title: str, query: str, scope: list[str], resp: dict):
    print(f"\n{title}")
    print(f"  Query: '{query}' | Scope: {scope}")
    print(f"  Status: {resp.get('status')}")
    body = resp.get("body", {})
    res_text = body.get("result", "")
    if res_text:
        print(f"  Result: {res_text[:200]}..." if len(res_text) > 200 else f"  Result: {res_text}")
    chunks = body.get("chunks", [])
    if chunks:
        print(f"  Retrieved {len(chunks)} chunks. Context snippet:")
        snippet = chunks[0].get('text', '').replace('\n', ' ')
        print(f"  -> {snippet[:150]}...")
    else:
        print("  Retrieved 0 chunks (Pre-filter blocked or no data).")


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--gateway", default="http://127.0.0.1:8080")
    p.add_argument("--keys", default=str(ROOT / "gateway" / "demo" / "keys"))
    args = p.parse_args()
    keys = Path(args.keys)
    issuer = keys / "issuer_priv.raw"
    
    # Reset cumulative stats so this run reports clean, non-accumulated numbers.
    import requests
    try:
        requests.post(args.gateway + "/v1/stats/reset", timeout=10)
    except requests.RequestException as e:
        print(f"  (warning: stats reset failed: {e})")

    # Initialize agents
    doc = AgentClient.from_keys(str(issuer), str(keys / "doctor_priv.raw"), args.gateway)
    bill = AgentClient.from_keys(str(issuer), str(keys / "billing_priv.raw"), args.gateway)
    pat = AgentClient.from_keys(str(issuer), str(keys / "patient_priv.raw"), args.gateway)

    print("=== ALICE (PROVIDER AGENT) ===")
    r1 = doc.retrieve("lab result", ["lab"], "alice", ["provider"])
    print_case("[SUCCESS] Alice requests lab results", "lab result", ["lab"], r1)

    r2 = doc.retrieve("diabetes diagnosis", ["phi"], "alice", ["provider"])
    print_case("[SUCCESS] Alice requests diagnosis", "diabetes diagnosis", ["phi"], r2)

    r3 = doc.retrieve("billing and insurance", ["billing"], "alice", ["provider"])
    print_case("[FAILURE] Alice requests billing records", "billing and insurance", ["billing"], r3)

    s, b = check_403(doc, "alice", ["provider"], ["lab"], args.gateway)
    print(f"\n[403 FORBIDDEN] Alice stolen token replay on wrong URI\n  Status: {s}")

    print("\n=== CAROL (BILLING AGENT) ===")
    r4 = bill.retrieve("insurance and copay", ["billing"], "carol", ["billing"])
    print_case("[SUCCESS] Carol requests billing records", "insurance and copay", ["billing"], r4)

    r5 = bill.retrieve("Bob diagnosis", ["phi"], "carol", ["billing"])
    print_case("[FAILURE] Carol requests clinical diagnosis", "Bob diagnosis", ["phi"], r5)

    s, b = check_403(bill, "carol", ["billing"], ["billing"], args.gateway)
    print(f"\n[403 FORBIDDEN] Carol stolen token replay on wrong URI\n  Status: {s}")

    print("\n=== BOB (PATIENT AGENT) ===")
    r6 = pat.retrieve("Bob patient chart", ["phi:patient:bob", "conf:R"], "bob", ["patient"])
    print_case("[SUCCESS] Bob requests his own chart", "Bob patient chart", ["phi:patient:bob", "conf:R"], r6)

    r7 = pat.retrieve("dump provider notes", ["note:provider"], "bob", ["patient"])
    print_case("[FAILURE] Bob requests provider notes", "dump provider notes", ["note:provider"], r7)

    s, b = check_403(pat, "bob", ["patient"], ["phi:patient:bob"], args.gateway)
    print(f"\n[403 FORBIDDEN] Bob stolen token replay on wrong URI\n  Status: {s}")

    print("\n=== DASHBOARD STATS ===")
    import requests
    stats = requests.get(args.gateway + "/v1/stats", timeout=10)
    print(stats.text)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
