# L6 索引参数优化基准报告模板

## 背景

- 数据集版本：
- collection：
- 向量维度：
- 评测样本量：
- 执行日期：

## baseline

- Profile：
- 索引类型：
- 参数：
- Recall@K：
- MRR：
- nDCG：
- P50 / P95：
- CPU / 内存：

## candidate 对比

| Profile | Family | Recall@K | MRR | nDCG | P50(ms) | P95(ms) | CPU User(ms) | HeapAlloc(MB) |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |

## 最终推荐

- 推荐 profile：
- 推荐原因：
- 风险说明：

## 回滚清单

1. 释放当前 collection。
2. 切回 baseline profile 并重建索引。
3. 重新 load collection。
4. 运行离线 smoke benchmark 验证 Recall@K 与 P95。

## 执行命令

```bash
go run ./cmd/retrieval-benchmark \
  -config ./config.yaml \
  -dataset ./scripts/evaluation/index_benchmark_dataset.example.json \
  -profiles ./scripts/evaluation/index_scan_profiles.json \
  -output ./docs/index-benchmark-report
```
