#!/bin/bash
# RAG 边界依赖检查脚本
# 用于检查 RAG 模块与业务模块之间的违规导入
# 用法: bash scripts/check_rag_boundaries.sh

set -e

cd "$(dirname "$0")/.."

echo "=== Checking RAG -> Business imports ==="
violations=0

for rag_dir in "internal/milvus" "internal/rag" "api/handler/kb" "internal/model/kb_" "internal/service/kb"; do
    for biz_pkg in "internal/agents" "internal/service/interview" "internal/service/resume" "internal/service/prediction" "internal/payment" "internal/model/interview_" "internal/model/resume" "internal/model/prediction" "internal/model/payment_" "internal/model/subscription"; do
        if grep -r "\"interview-agents/$biz_pkg" "$rag_dir" 2>/dev/null; then
            echo "VIOLATION: RAG package $rag_dir imports business package $biz_pkg"
            violations=$((violations + 1))
        fi
    done
done

echo ""
echo "=== Checking Business -> RAG internal imports ==="
for biz_dir in "internal/agents" "internal/service/interview" "internal/service/resume" "internal/service/prediction" "internal/payment"; do
    for rag_pkg in "internal/milvus" "internal/rag" "api/handler/kb"; do
        if grep -r "\"interview-agents/$rag_pkg" "$biz_dir" 2>/dev/null; then
            echo "VIOLATION: Business package $biz_dir imports RAG package $rag_pkg"
            violations=$((violations + 1))
        fi
    done
done

echo ""
if [ $violations -gt 0 ]; then
    echo "Found $violations boundary violations!"
    exit 1
fi
echo "No boundary violations found."
