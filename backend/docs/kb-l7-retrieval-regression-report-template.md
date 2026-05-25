# L7 检索离线回归报告模板

## 1. 变更说明

- 变更摘要：
- 涉及策略：
- 发布目标：
- 评测数据集版本：

## 2. Baseline vs Candidate

- Baseline：
- Candidate：
- Recall@K Delta：
- MRR Delta：
- nDCG Delta：
- Citation Accuracy Delta：
- P95 Latency Delta：

## 3. 各策略结果

| Strategy | Recall@K | MRR | nDCG | Citation Accuracy | P50(ms) | P95(ms) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| dense_only |  |  |  |  |  |  |
| hybrid |  |  |  |  |  |  |
| hybrid_rewrite |  |  |  |  |  |  |
| hybrid_rewrite_dynamic_topk |  |  |  |  |  |  |

## 4. 贡献度分析

- `hybrid` 相对 `dense_only`：
- `hybrid_rewrite` 相对 `hybrid`：
- `hybrid_rewrite_dynamic_topk` 相对 `hybrid_rewrite`：

## 5. 风险结论

- 质量风险：
- 延迟风险：
- 数据集覆盖盲区：
- 是否建议进入灰度：

## 6. 回滚策略

1. 关闭 `RAG_ENABLE_DYNAMIC_TOPK`
2. 关闭 `RAG_ENABLE_QUERY_REWRITE`
3. 关闭 `RAG_ENABLE_HYBRID_RETRIEVAL`
4. 回退到 Phase 1 基线路径
