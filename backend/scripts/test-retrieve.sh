#!/bin/bash
# 测试 RAG 检索接口

BASE_URL="${RAG_BASE_URL:-http://localhost:8081}"

echo "=== Test 1: Legacy app_id ==="
curl -s -X POST "$BASE_URL/v1/retrieve" \
  -H "Content-Type: application/json" \
  -d '{"app_id":"interview-agent","query":"什么是 JVM？","kb_ids":[1]}' | jq .

echo ""
echo "=== Test 2: API Key (需要先创建) ==="
# curl -s -X POST "$BASE_URL/v1/retrieve" \
#   -H "Content-Type: application/json" \
#   -H "Authorization: Bearer rag_xxxx" \
#   -d '{"query":"什么是 JVM？","kb_ids":[1]}' | jq .

echo ""
echo "=== Test 3: 无认证（应返回 401） ==="
curl -s -X POST "$BASE_URL/v1/retrieve" \
  -H "Content-Type: application/json" \
  -d '{"query":"什么是 JVM？","kb_ids":[1]}' | jq .
