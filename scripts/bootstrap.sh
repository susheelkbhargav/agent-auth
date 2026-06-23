#!/usr/bin/env bash
# Bootstrap demo on M1: keys → ingest → gateway (requires Ollama + CGO).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GW="$ROOT/gateway"
cd "$GW"

echo "==> keygen"
go run ./cmd/keygen -out ./demo/keys

echo "==> ingest deps"
pip install -r ingestlib/requirements.txt
python -m spacy download en_core_web_sm 2>/dev/null || true
echo "==> ingest (WikiDoc + Synthea + Presidio + Ollama)"
python ingestlib/ingest.py --app-db ./app.db --keys ./demo/keys --ollama "${OLLAMA_URL:-http://127.0.0.1:11434}"

echo "==> gateway"
export ISSUER_PUBKEY_PATH=./demo/keys/issuer_pub.raw
export APP_DB=./app.db
exec go run ./cmd/gateway
