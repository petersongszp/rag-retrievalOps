# Phase 2 L0 Contract Freeze

本文档冻结 RAG Admin Phase 2（P2）“检索质量与离线评测看板”的业务边界、接口路径、字段口径和非目标范围，作为 L1-L8 的唯一实现基线。

## 1. 目标与边界

P2 的目标是把现有离线检索评测能力包装成管理台可管理、可触发、可查看、可追踪的业务闭环。

P2 必须完成：

1. 管理台可创建、查看、导入、导出、校验评测集。
2. 管理台可创建评测运行，选择 dataset、baseline profile、candidate profile 和 gate thresholds。
3. 管理台可查看运行状态、进度、错误信息和历史运行列表。
4. 管理台可查看评测报告，展示 baseline vs candidate 指标对比、delta、贡献度和 gate 结果。
5. 管理台可查看失败样本，并从失败样本跳转到对应 retrieval trace。
6. Retrieval Lab 可把一次检索结果保存为评测样本草稿。
7. 页面遇到接口失败或字段缺口时必须显式展示，不得伪造数据。

P2 明确不做：

1. 不在前端计算 Recall@K、MRR、nDCG、Citation Accuracy。
2. 不从前端直接执行 `go run ./cmd/retrieval-eval` 或 Python 脚本。
3. 不做线上真实流量 A/B 实验平台。
4. 不做策略开关、灰度发布、策略回滚控制台。
5. 不做高级检索解释视图（fusion/rerank/filter 细粒度解释）。
6. 不自动生成或猜测 `relevant_ids`、`citation_targets`。
7. 不修改知识库文档内容，不把评测样本当训练数据管理。

## 2. 统一术语

1. `离线评测`：后端基于固定评测集和策略 profile 执行离线检索回归任务，产出标准报告。
2. `评测集`：由多条检索样本组成的数据集，单条样本以 `backend/internal/milvus/evaluation.DatasetCase` 为语义基线。
3. `评测运行`：一次由后端创建并执行的后台任务，关联 dataset、profile 和 gate thresholds。
4. `评测报告`：`evaluation.Report` 的 HTTP 化结果，供前端直接展示。
5. `失败样本`：后端根据 query 级指标下钻出的低质量样本，不由前端二次推断。
6. `合同缺口`：关键字段缺失或接口不可用时，页面必须显式提示的状态。

## 3. 冻结的 API 前缀

P2 后端管理台接口统一挂在：

`/api/admin/kb/eval/*`

禁止在 P2 中新增独立的 `/api/admin/evaluation/*` 或 `/evaluation/*` API 前缀。

## 4. 冻结的数据对象

### 4.1 Eval Dataset

状态枚举固定为：

- `draft`
- `ready`
- `invalid`
- `archived`

字段基线：

- `id`
- `name`
- `description`
- `kb_id`
- `case_count`
- `status`
- `created_by`
- `created_at`
- `updated_at`

规则：

1. 空评测集允许保存为 `draft`。
2. 只有满足校验条件的评测集才能进入 `ready`。
3. `invalid` 表示最近一次校验未通过。
4. `archived` 在 P2 仅作为保留状态，不要求提供页面操作入口。

### 4.2 Eval Case

字段基线：

- `id`
- `dataset_id`
- `case_key`
- `query`
- `top_k`
- `relevant_ids`
- `citation_targets`
- `query_type`
- `tags`
- `kb_ids`
- `collection`
- `notes`
- `validation_status`
- `validation_errors`
- `created_at`
- `updated_at`

其中：

1. `case_key` 对应 `evaluation.DatasetCase.ID`。
2. `validation_status` 枚举固定为 `valid | invalid | unchecked`。
3. `citation_targets` 结构固定为：
   - `document_id`
   - `chunk_id`
   - `file_name`

校验规则固定为：

1. `query` 非空。
2. `top_k > 0`。
3. `relevant_ids` 与 `citation_targets` 至少一类可用于质量判断。
4. 同一 `dataset_id` 内 `case_key` 唯一。
5. `kb_ids` 如有值，不得与 dataset 绑定的 `kb_id` 语义冲突。

### 4.3 Eval Strategy Profile

P2 前后端共识字段：

- `name`
- `label`
- `baseline`
- `candidate`
- `mode`
- `enable_query_rewrite`
- `enable_dynamic_topk`
- `enable_advanced_rerank`
- `candidate_top_k`
- `dense_weight`
- `sparse_weight`
- `min_top_k`
- `max_top_k`
- `token_budget`
- `rewrite_max_expansions`
- `rerank_timeout_ms`
- `rerank_model`

说明：

1. 上述字段对齐 `backend/internal/milvus/evaluation.StrategyProfile` 的 P2 使用子集。
2. 后端内部如保留额外字段，不要求 P2 前端暴露。
3. 前端只做 JSON 结构校验，业务合法性由后端裁定。

### 4.4 Eval Run

状态枚举固定为：

- `pending`
- `running`
- `succeeded`
- `failed`
- `canceled`

字段基线：

- `id`
- `run_id`
- `dataset_id`
- `baseline_profile`
- `candidate_profile`
- `gate_thresholds`
- `status`
- `progress`
- `case_total`
- `case_finished`
- `report_path`
- `error_msg`
- `created_by`
- `started_at`
- `finished_at`
- `created_at`
- `updated_at`

规则：

1. `run_id` 是稳定可查询主键，前端详情页和报告页统一使用它。
2. 同一 dataset 允许存在多次运行。
3. 成功运行生成的历史报告不得被后续运行覆盖。
4. P2 允许使用轮询更新状态，不强制 SSE。

### 4.5 Eval Report

报告对象必须保留 `evaluation.Report` 的核心语义：

- `dataset_size`
- `generated_at`
- `results`
- `contribution`
- `comparison`
- `gate`
- `baseline`
- `candidate`

前端不得自行重算以下聚合指标：

- `recall_at_k`
- `mrr`
- `ndcg`
- `citation_accuracy`
- `p50_latency_ms`
- `p95_latency_ms`
- `avg_latency_ms`
- `p95_latency_delta_ms`
- `p95_latency_delta_ratio`

备注：

1. 文案上允许把 `citation_accuracy` 展示为 “Citation Precision” 或中文说明，但 API 字段名固定为 `citation_accuracy`。
2. Gate 是否通过以报告里的 `gate` 为准，前端不得自行判断。

### 4.6 Failure Case

失败原因枚举固定为：

- `recall_miss`
- `citation_miss`
- `mrr_drop`
- `ndcg_drop`
- `latency_regression`
- `gate_failed`
- `trace_missing`

失败样本响应至少包含：

- `case_id`
- `query`
- `query_type`
- `tags`
- `failure_reason`
- `baseline_metrics`
- `candidate_metrics`
- `delta`
- `baseline_request_id`
- `candidate_request_id`

规则：

1. 失败原因由后端生成，前端只展示和筛选。
2. `request_id` 缺失是允许状态，前端需展示“未生成 trace”。
3. trace 跳转路径统一为 `/trace-logs/retrieval?request_id=...`。

## 5. 冻结的 API 清单

### 5.1 Datasets

1. `GET /api/admin/kb/eval/datasets`
2. `POST /api/admin/kb/eval/datasets`
3. `GET /api/admin/kb/eval/datasets/{dataset_id}/items`
4. `POST /api/admin/kb/eval/datasets/{dataset_id}/items`
5. `POST /api/admin/kb/eval/datasets/{dataset_id}/items/import`
6. `GET /api/admin/kb/eval/datasets/{dataset_id}/items/export`
7. `POST /api/admin/kb/eval/datasets/{dataset_id}/validate`

### 5.2 Runs

1. `GET /api/admin/kb/eval/runs`
2. `POST /api/admin/kb/eval/runs`
3. `GET /api/admin/kb/eval/runs/{run_id}`
4. `POST /api/admin/kb/eval/runs/{run_id}/cancel`（可选实现，可选展示）

### 5.3 Reports

1. `GET /api/admin/kb/eval/runs/{run_id}/report`
2. `GET /api/admin/kb/eval/runs/{run_id}/cases`
3. `GET /api/admin/kb/eval/runs/{run_id}/report/export?format=json|markdown`

## 6. 冻结的页面职责

1. `/evaluation/datasets`
   - 管理评测集
   - 管理样本
   - 导入导出
   - 校验并查看样本级错误
2. `/evaluation/runs`
   - 创建运行
   - 查看运行列表
   - 轮询运行状态
   - 跳转报告页
3. `/evaluation/reports/[runId]`
   - 展示完整评测报告
   - 展示失败样本和 trace 跳转
   - 导出 JSON/Markdown 报告
4. `/quality-monitor`
   - 只展示最近一次成功评测的摘要
   - 不承担评测集和运行管理职责
5. `/retrieval-lab`
   - 新增“保存为评测样本”入口
   - 仅保存草稿样本，不自动补 golden answer

## 7. 非回退要求

P2 开发过程中不得回退以下能力：

1. P0 知识库创建、文档上传、文档删除。
2. P0/P1 入库任务列表、重试、取消。
3. Retrieval Lab 检索、复制 request_id、跳转 trace。
4. Dashboard 既有统计与趋势页。
5. Trace Logs 检索链路查看。

## 8. L0 验收标准

L0 完成即视为以下条件全部满足：

1. 前后端实现统一以本文档作为 P2 契约基线。
2. P2 API 前缀统一为 `/api/admin/kb/eval/*`。
3. 关键枚举、指标字段、页面职责和非目标范围均已冻结。
4. `citation_accuracy` 与展示文案之间的映射已明确，不再产生双口径。
5. 后续 L1-L8 的实现与验收均以本文档字段命名和边界为准。
