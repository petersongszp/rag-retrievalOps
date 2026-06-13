#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$ROOT_DIR"

if command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  COMPOSE="docker compose"
fi

export DOCLING_SERVE_IMAGE="${DOCLING_SERVE_IMAGE:-quay.io/docling-project/docling-serve-cpu:latest}"
export DOCLING_SERVE_PORT="${DOCLING_SERVE_PORT:-5001}"
export PARSER_PROVIDER_PORT="${PARSER_PROVIDER_PORT:-9000}"
export DOCLING_TIMEOUT_MS="${DOCLING_TIMEOUT_MS:-120000}"

$COMPOSE --profile parser up -d docling-serve parser-provider

cat <<EOF
Docling parser stack started.

Docling Serve UI:
  http://localhost:${DOCLING_SERVE_PORT}/ui

Parser Provider health:
  http://localhost:${PARSER_PROVIDER_PORT}/healthz

Use this for local backend runs:
  DOCUMENT_PARSER_ENDPOINT=http://localhost:${PARSER_PROVIDER_PORT}/parse

Use this for rag-server inside docker-compose:
  DOCUMENT_PARSER_ENDPOINT=http://parser-provider:9000/parse
EOF
