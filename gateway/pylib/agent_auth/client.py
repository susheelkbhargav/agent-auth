"""Minimal agent client: mint OBO + DPoP and call /v1/retrieve."""

from __future__ import annotations

import base64
import hashlib
import json
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import jwt
import requests
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def kid(pub: bytes) -> str:
    return hashlib.sha256(pub).hexdigest()


def _load_ed25519(path: str) -> Ed25519PrivateKey:
    seed = Path(path).read_bytes()[:32]
    return Ed25519PrivateKey.from_private_bytes(seed)


@dataclass
class AgentClient:
    issuer_key: Ed25519PrivateKey
    agent_key: Ed25519PrivateKey
    gateway: str
    aud: str = "agent-auth"

    @classmethod
    def from_keys(cls, issuer_priv_path: str, agent_priv_path: str, gateway: str) -> "AgentClient":
        return cls(
            issuer_key=_load_ed25519(issuer_priv_path),
            agent_key=_load_ed25519(agent_priv_path),
            gateway=gateway.rstrip("/"),
        )

    def _agent_pub(self) -> bytes:
        return self.agent_key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)

    def mint_obo(self, sub: str, roles: list[str], task_scope: list[str], ttl_sec: int = 3600) -> str:
        now = int(time.time())
        payload = {
            "sub": sub,
            "aud": self.aud,
            "exp": now + ttl_sec,
            "iat": now,
            "user_roles": roles,
            "task_scope": task_scope,
            "act": kid(self._agent_pub()),
        }
        return jwt.encode(payload, self.issuer_key, algorithm="EdDSA")

    def mint_dpop(self, method: str, uri: str, obo: str, body: bytes, jti: str) -> str:
        now = int(time.time())
        headers = {
            "jwk": {
                "kty": "OKP",
                "crv": "Ed25519",
                "x": _b64url(self._agent_pub()),
            }
        }
        claims = {
            "htm": method,
            "htu": uri,
            "ath": _b64url(hashlib.sha256(obo.encode()).digest()),
            "jti": jti,
            "iat": now,
            "body_hash": _b64url(hashlib.sha256(body).digest()),
        }
        return jwt.encode(claims, self.agent_key, algorithm="EdDSA", headers=headers)

    def retrieve(self, query: str, task_scope: list[str], user_id: str, roles: list[str]) -> dict[str, Any]:
        path = "/v1/retrieve"
        body_obj = {"query": query, "task_scope": task_scope}
        body = json.dumps(body_obj).encode()
        obo = self.mint_obo(user_id, roles, task_scope)
        jti = hashlib.sha256(f"{time.time_ns()}".encode()).hexdigest()[:32]
        dpop = self.mint_dpop("POST", path, obo, body, jti)
        headers = {
            "Authorization": f"Bearer {obo}",
            "X-Sig": dpop,
            "X-Nonce": jti,
            "X-Timestamp": str(int(time.time())),
            "Content-Type": "application/json",
        }
        resp = requests.post(self.gateway + path, data=body, headers=headers, timeout=120)
        return {"status": resp.status_code, "body": resp.json() if resp.content else {}}
