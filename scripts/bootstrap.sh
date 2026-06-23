#!/usr/bin/env bash
# Bootstrap demo on M1: keys → ingest → gateway (requires Ollama + CGO).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GW="$ROOT/gateway"
cd "$GW"

echo "==> keygen"
go run ./cmd/keygen -out ./demo/keys

echo "==> ingest (needs ollama + nomic-embed-text)"
go run ./cmd/ingest -app-db ./app.db -keys ./demo/keys -ollama "${OLLAMA_URL:-http://127.0.0.1:11434}"

echo "==> gateway"
export ISSUER_PUBKEY_PATH=./demo/keys/issuer_pub.raw
export APP_DB=./app.db
exec go run ./cmd/gateway
