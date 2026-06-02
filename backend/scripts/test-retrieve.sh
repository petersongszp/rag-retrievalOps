#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${RAG_BASE_URL:-http://localhost:8081}"
KB_ID="${KB_ID:-1}"
QUERY="${QUERY:-知识库里关于 Go 并发的内容是什么？}"

echo "== Phase 4 Retrieve Smoke =="
echo "BASE_URL=$BASE_URL"
echo "KB_ID=$KB_ID"

if [[ -n "${RAG_API_KEY:-}" ]]; then
  echo ""
  echo "== Test: API Key retrieve =="
  curl -sS -X POST "$BASE_URL/v1/retrieve" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $RAG_API_KEY" \
    -d "{
      \"query\": \"$QUERY\",
      \"kb_ids\": [$KB_ID],
      \"top_k\": 5
    }" | jq .
else
  echo ""
  echo "RAG_API_KEY 未设置，跳过 API Key smoke。"
fi

echo ""
echo "== Test: legacy app_id retrieve =="
curl -sS -X POST "$BASE_URL/v1/retrieve" \
  -H "Content-Type: application/json" \
  -d "{
    \"app_id\": \"interview-agent\",
    \"query\": \"$QUERY\",
    \"kb_ids\": [$KB_ID],
    \"top_k\": 3
  }" | jq .

echo ""
echo "== Test: unauthenticated retrieve should fail =="
curl -sS -X POST "$BASE_URL/v1/retrieve" \
  -H "Content-Type: application/json" \
  -d "{
    \"query\": \"$QUERY\",
    \"kb_ids\": [$KB_ID]
  }" | jq .
