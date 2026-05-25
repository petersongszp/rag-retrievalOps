# scripts/evaluation

这个目录现在同时承载两类评测：

1. `L7 检索离线回归`：用于 `Recall@K / MRR / nDCG / Citation Accuracy`、门禁和贡献度分析。
2. `Ragas 问答评估`：用于 legacy 的回答质量打分。

## 1. 检索回归数据集

默认数据集是 [dataset.json](./dataset.json)。

每条记录建议至少包含：

- `id`
- `query`
- `top_k`
- `relevant_ids`

推荐补充：

- `citation_targets`
- `query_type`
- `tags`
- `kb_ids`
- `question/context/ground_truth`

后面三项让同一个文件也能兼容 `Ragas` 模式。

## 2. 检索回归运行方式

直接跑 Python 入口：

```bash
python scripts/evaluation/evaluate.py
```

它默认会调用：

```bash
go run ./cmd/retrieval-eval \
  -config config.yaml \
  -dataset scripts/evaluation/dataset.json \
  -profiles scripts/evaluation/retrieval_strategy_profiles.example.json \
  -gates scripts/evaluation/retrieval_gate_thresholds.example.json \
  -output docs/retrieval-regression-report
```

常用参数：

```bash
python scripts/evaluation/evaluate.py \
  --baseline dense_only \
  --candidate hybrid_rewrite_dynamic_topk
```

直接跑 Go 命令也可以：

```bash
go run ./cmd/retrieval-eval \
  -config ./config.yaml \
  -dataset ./scripts/evaluation/dataset.json \
  -profiles ./scripts/evaluation/retrieval_strategy_profiles.example.json \
  -gates ./scripts/evaluation/retrieval_gate_thresholds.example.json \
  -output ./docs/retrieval-regression-report
```

产物：

- `docs/retrieval-regression-report.json`
- `docs/retrieval-regression-report.md`

退出码：

- `0`：评测和门禁都通过
- `2`：评测完成，但门禁失败
- `非 0/2`：执行失败

## 3. 策略贡献分析

默认策略链如下：

1. `dense_only`
2. `hybrid`
3. `hybrid_rewrite`
4. `hybrid_rewrite_dynamic_topk`

报告会自动输出每一跳相对上一跳的增益，方便回答：

- 混合检索到底有没有带来收益
- 改写是增益还是噪声
- 动态 TopK 是否只省 token 还是也伤了质量

## 4. 门禁阈值

默认阈值文件是 [retrieval_gate_thresholds.example.json](./retrieval_gate_thresholds.example.json)，当前默认口径：

- `Recall@K delta >= 0.08`
- `MRR delta >= 0`
- `nDCG delta >= 0`
- `Citation Accuracy delta >= 0`
- `P95 latency regression ratio <= 0.20`

如果要接 CI，推荐直接在流水线里判断命令退出码。

## 5. Legacy Ragas 模式

保留原来的问答评估能力：

```bash
python scripts/evaluation/evaluate.py --mode ragas --no-api
python scripts/evaluation/evaluate.py --mode ragas
```

产物：

- `scripts/evaluation/evaluation_report.json`
