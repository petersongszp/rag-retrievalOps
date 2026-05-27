# Phase 2 详细功能实现路线（检索质量与离线评测看板，前后端并行推进）

## 1. 文档定位

本文档是 `admin/docs/rag-admin-frontend-roadmap-v2.md` 中 Phase 2（P2）"检索质量与离线评测看板"的详细执行手册，同时覆盖前后端，可直接按 L 编号逐步实现。

三个用途：

1. 作为前后端 P2 联合推进的统一执行文档，以 P1 可观测性底座为起点。
2. 作为冻结评测集、评测运行、评测报告、失败样本、指标口径的协作基线。
3. 作为 Phase 3 接入高级检索调试视图与策略中心前的质量评测底座。

**统一口径说明：**

1. `离线评测` 固定指：后端基于固定评测集和策略 profile 执行检索回归，输出 Recall@K、MRR、nDCG、Citation Accuracy、P50/P95 延迟等指标；它不是在线流量 A/B，也不是用户答案评分。
2. `评测集` 固定指：由多条检索样本组成的数据集，单条样本字段以 `backend/internal/milvus/evaluation.DatasetCase` 为基线，最小包含 `id / query / top_k / relevant_ids / citation_targets / query_type / tags / kb_ids / collection / notes`。
3. `评测运行` 固定指：一次由后端创建并执行的离线任务，包含 `dataset_id / baseline_profile / candidate_profile / status / progress / started_at / finished_at / error_msg`。
4. `评测报告` 固定指：后端 `evaluation.Report` 结构的 HTTP 化返回，包含 `dataset_size / generated_at / results / contribution / comparison / gate / baseline / candidate`。
5. `失败样本` 固定指：从 query 级指标中下钻出的低质量样本，不由前端重新判断；后端需要返回 `failure_reason`，例如 `recall_miss / citation_miss / ndcg_drop / latency_regression / gate_failed`。
6. `Citation Accuracy` 固定沿用后端当前指标字段名；总路线文档中的 `Citation Precision` 在 P2 页面可作为展示文案别名，但 API 字段以 `citation_accuracy` 为准，避免两套口径。
7. `业务边界` 固定指：前端只做评测资产管理、运行触发、状态展示、报告可视化和 trace 下钻；不在浏览器计算 Recall/MRR/nDCG，不直接执行本地脚本，不决定门禁是否通过。
8. `契约缺口` 固定指：关键字段或接口缺失时页面明确标识，不静默隐藏、不用假数据补齐。

---

## 2. 当前现状（基于代码扫描）

### 2.1 后端已有能力

经扫描 `backend/docs/phase2-retrieval-quality-detailed-roadmap.md`、`backend/internal/milvus/evaluation/`、`backend/cmd/retrieval-eval/main.go`、`backend/scripts/evaluation/`：

1. **离线评测内核已存在**：`backend/internal/milvus/evaluation` 已有 `DatasetCase / StrategyProfile / QueryMetrics / AggregateMetrics / Report` 等核心结构。
2. **指标计算已存在**：`metrics.go` 已实现 Recall@K、MRR、nDCG、Citation Accuracy、P50/P95/Avg latency 聚合。
3. **评测 Runner 已存在**：`Runner.Run()` 支持多策略 profile 执行、baseline/candidate 对比、贡献度分析、门禁结果生成。
4. **报告导出已存在**：`io.go` 支持保存 JSON 和 Markdown 报告，Markdown 报告包含指标对比、Baseline vs Candidate、贡献度分析、门禁结果。
5. **CLI 入口已存在**：`backend/cmd/retrieval-eval/main.go` 可基于 dataset、profiles、gates 输出回归报告。
6. **样例配置已存在**：
   - `backend/scripts/evaluation/dataset.json`
   - `backend/scripts/evaluation/retrieval_strategy_profiles.example.json`
   - `backend/scripts/evaluation/retrieval_gate_thresholds.example.json`
7. **Phase 2 检索策略代码已有落点**：`backend/internal/milvus/retrieval/` 已有 hybrid、fusion、dedupe、rewrite、topk、reranker 等模块，P2 看板可消费这些策略输出的评测结果。
8. **P1 trace 底座已接入**：`/api/admin/kb/retrieve/audit` 和 `/trace-logs/retrieval` 已可按 `request_id` 下钻，P2 失败样本可复用此链路。

### 2.2 前端已有能力

经扫描 `admin/src/`：

1. **管理台骨架完整**：`AdminShell` 已有 `/dashboard / knowledge-bases / retrieval-lab / trace-logs / quality-monitor` 等导航。
2. **P1 页面已存在**：`dashboard-page.tsx`、`retrieval-logs-page.tsx`、`ingest-logs-page.tsx` 已具备监控、日志、trace 下钻基础。
3. **`/quality-monitor` 是 P2 承接位**：当前页面只展示 P2 预留提示，没有真实评测指标。
4. **`/evaluation` 导航仍为禁用态**：`admin-shell.tsx` 中已声明但未启用，也没有对应路由和页面组件。
5. **API 客户端可复用**：`admin/src/services/api/client.ts` 和 `admin/src/config/api.ts` 可继续扩展 P2 路径。
6. **P1 类型已存在**：`admin/src/types/kb.ts` 已有 `KBRetrieveLog / MetricsOverview / RetrieveResponse` 等类型，但缺少 P2 评测类型。
7. **检索实验室已有结果和 request_id**：`retrieval-lab-page.tsx` 能展示检索结果并跳转 trace，但没有"保存为评测样本"能力。

### 2.3 当前真实缺口

**后端缺口：**

1. 没有面向管理台的评测集 CRUD API。
2. 没有评测样本导入、导出、校验 API。
3. 没有评测运行创建、状态查询、取消 API。
4. 没有把 `evaluation.Report` 结构作为 HTTP 报告返回的 API。
5. 没有失败样本列表 API，前端无法按失败原因下钻。
6. 没有评测运行持久化模型，CLI 产物还没有变成管理台可查询的业务对象。
7. 没有从检索实验室保存样本到评测集的接口。

**前端缺口：**

1. `admin/src/types/kb.ts` 缺少评测集、评测样本、策略 profile、评测运行、评测报告、失败样本类型。
2. `admin/src/config/api.ts` 缺少 P2 评测相关 API 路径常量。
3. `/evaluation/datasets` 路由和页面组件不存在。
4. `/evaluation/runs` 路由和页面组件不存在。
5. `/evaluation/reports/[runId]` 路由和页面组件不存在。
6. `/quality-monitor` 仍是占位页，没有消费真实评测报告。
7. `retrieval-lab-page.tsx` 没有保存检索 query 与结果为评测样本的入口。

---

## 3. 范围边界与通过标准（Gate）

### 3.1 P2 必须完成

1. 管理台可创建、查看、导入、导出、校验评测集。
2. 管理台可创建评测运行，选择 dataset、baseline profile、candidate profile、gate thresholds。
3. 管理台可查看运行状态、进度、错误信息和历史运行列表。
4. 管理台可查看评测报告，展示 baseline vs candidate 指标对比和 delta。
5. 管理台可查看失败样本，并从失败样本跳转到对应 trace。
6. 检索实验室可把一次检索保存为评测样本，但只保存样本草稿或样本记录，不自动判定 relevant_ids。
7. 所有 P2 页面在后端字段缺失时展示契约缺口，不展示假指标。

### 3.2 P2 明确不做

1. 不在前端实现 Recall@K、MRR、nDCG、Citation Accuracy 的计算逻辑。
2. 不从前端直接执行 `go run ./cmd/retrieval-eval` 或本地 Python 脚本。
3. 不做线上真实流量 A/B 实验平台。
4. 不做策略开关管理、灰度比例修改、一键回滚；这些归 Phase 3 Strategy Center。
5. 不做父子块检索、证据拒答、复杂调试视图；这些归 Phase 3。
6. 不自动生成或猜测 `relevant_ids`、`citation_targets`；缺少标准答案时必须标为待补齐。
7. 不修改知识库文档内容，不把评测样本等同于训练数据。

### 3.3 Phase 2 通过标准（全满足）

1. `/evaluation/datasets` 可管理评测集和样本，样本校验错误可见。
2. `/evaluation/runs` 可创建一次评测运行，并展示 `pending / running / succeeded / failed / canceled` 状态。
3. `/evaluation/reports/[runId]` 可展示完整报告：Recall@K、MRR、nDCG、Citation Accuracy、P50/P95 延迟、Gate 结果、贡献度分析。
4. baseline vs candidate 的 delta 展示清晰，正收益与负收益可区分。
5. 失败样本可按失败原因筛选，并能跳转到 `/trace-logs/retrieval?request_id=xxx`；没有 trace 时明确展示"未生成 trace"。
6. 检索实验室可保存样本草稿到指定评测集，不影响原有检索测试和 trace 跳转。
7. TypeScript 编译无类型错误，接口失败时页面有可读错误提示，不白屏。
8. P0/P1 功能不回退：知识库管理、文档上传、检索测试、Dashboard、Trace Logs 均保持可用。

---

## 4. 实现路线总览（L0 -> L8）

Phase 2 按 9 条路线推进，按门禁顺序合流：

1. L0：P2 业务边界、指标口径与 API 路径冻结
2. L1：后端 - 评测集与样本管理 API
3. L2：后端 - 评测运行编排、状态与持久化
4. L3：后端 - 评测报告、失败样本与 trace 关联 API
5. L4：前端 - P2 类型契约、API 路径与导航激活
6. L5：前端 - 评测集页面（/evaluation/datasets）
7. L6：前端 - 评测运行页面（/evaluation/runs）
8. L7：前端 - 评测报告页与质量监控页接入
9. L8：检索实验室保存样本、回归验收、回滚预案与 Phase 3 交接

建议顺序：`L0 -> L1 + L2 + L3（并行） -> L4 -> L5 + L6 + L7（并行） -> L8`

---

## 5. 详细路线拆解

### 5.1 L0 P2 业务边界、指标口径与 API 路径冻结

#### 目标

在开发前冻结 P2 的业务对象、指标字段、API 路径和不做事项，避免把评测看板做成策略中心或在线实验平台。

#### 功能任务

1. 统一 P2 管理台 API 前缀为 `/api/admin/kb/eval/*`，不再混用 `/evaluation/*`。
2. 冻结指标字段：
   - `recall_at_k`
   - `mrr`
   - `ndcg`
   - `citation_accuracy`
   - `p50_latency_ms`
   - `p95_latency_ms`
   - `avg_latency_ms`
   - `p95_latency_delta_ms`
   - `p95_latency_delta_ratio`
3. 冻结样本字段，以 `DatasetCase` 为基线：
   - `id / query / top_k / relevant_ids / citation_targets / query_type / tags / kb_ids / collection / notes`
4. 冻结策略 profile 字段，以 `StrategyProfile` 为基线：
   - `name / label / baseline / candidate / mode`
   - `enable_query_rewrite / enable_dynamic_topk / enable_advanced_rerank`
   - `candidate_top_k / dense_weight / sparse_weight / min_top_k / max_top_k / token_budget`
   - `rewrite_max_expansions / rerank_timeout_ms / rerank_model`
5. 冻结运行状态枚举：
   - `pending`
   - `running`
   - `succeeded`
   - `failed`
   - `canceled`
6. 冻结失败原因枚举：
   - `recall_miss`
   - `citation_miss`
   - `mrr_drop`
   - `ndcg_drop`
   - `latency_regression`
   - `gate_failed`
   - `trace_missing`
7. 冻结 P2 不可回退功能清单：
   - 评测集可管理
   - 评测运行可触发
   - 报告可查看
   - 失败样本可下钻
   - 检索实验室可保存样本
8. 记录 P2 不做清单：
   - 不做线上 A/B
   - 不做策略开关灰度
   - 不做前端指标计算
   - 不做自动 relevant 标注

#### 验收

1. 前后端对 P2 必做与不做边界达成一致。
2. API 路径、字段名和枚举以本文档为准。
3. 总路线文档中的 `Citation Precision` 与后端字段 `citation_accuracy` 已完成口径映射说明。

---

### 5.2 L1 后端 - 评测集与样本管理 API

#### 目标

把当前文件型 dataset 能力升级为管理台可管理的业务对象，让前端可以创建评测集、维护样本、导入导出、查看校验结果。

#### 功能任务

1. 新增评测集模型（命名可按项目习惯调整）：
   - `KBEvalDataset`
   - 字段：`id / name / description / kb_id / case_count / status / created_by / created_at / updated_at`
   - `status` 枚举：`draft / ready / invalid / archived`
2. 新增评测样本模型：
   - `KBEvalCase`
   - 字段：`id / dataset_id / case_key / query / top_k / relevant_ids / citation_targets / query_type / tags / kb_ids / collection / notes / validation_status / validation_errors / created_at / updated_at`
   - `case_key` 对应 `DatasetCase.ID`，同一 dataset 内唯一。
3. 新增列表接口：
   ```http
   GET /api/admin/kb/eval/datasets?kb_id=&status=&keyword=&page=1&page_size=20
   ```
   返回：
   ```json
   {
     "items": [
       {
         "id": 1,
         "name": "phase2-core-regression",
         "description": "核心检索回归集",
         "kb_id": 1,
         "case_count": 128,
         "status": "ready",
         "created_at": "2026-05-26T10:00:00Z",
         "updated_at": "2026-05-26T10:00:00Z"
       }
     ],
     "total": 1,
     "page": 1,
     "page_size": 20
   }
   ```
4. 新增创建接口：
   ```http
   POST /api/admin/kb/eval/datasets
   ```
   请求：
   ```json
   {
     "name": "phase2-core-regression",
     "description": "核心检索回归集",
     "kb_id": 1
   }
   ```
5. 新增样本列表接口：
   ```http
   GET /api/admin/kb/eval/datasets/{dataset_id}/items?query_type=&tag=&validation_status=&keyword=&page=1&page_size=20
   ```
6. 新增样本创建接口：
   ```http
   POST /api/admin/kb/eval/datasets/{dataset_id}/items
   ```
   请求字段对齐 `DatasetCase`。
7. 新增样本批量导入接口：
   ```http
   POST /api/admin/kb/eval/datasets/{dataset_id}/items/import
   ```
   支持 JSON 数组格式，字段对齐 `backend/scripts/evaluation/dataset.json`。
8. 新增样本导出接口：
   ```http
   GET /api/admin/kb/eval/datasets/{dataset_id}/items/export
   ```
   返回 JSON 文件或 JSON body，供离线脚本复用。
9. 新增样本校验接口：
   ```http
   POST /api/admin/kb/eval/datasets/{dataset_id}/validate
   ```
   校验规则：
   - `query` 非空
   - `top_k > 0`
   - `relevant_ids` 或 `citation_targets` 至少有一类可用于质量判断
   - `case_key` 不重复
   - `kb_ids` 与 dataset 的 `kb_id` 不冲突

#### 业务边界

1. 后端只校验样本结构和基础一致性，不自动判断哪些 chunk 是正确答案。
2. 导入失败时返回逐行错误，不部分静默丢弃。
3. 空评测集可以保存为 `draft`，但不能创建正式评测运行。

#### 验收

1. 可以创建评测集并返回 `dataset_id`。
2. 可以向评测集添加单条样本。
3. 可以导入 `backend/scripts/evaluation/dataset.json` 同结构的样本。
4. 校验接口能返回样本级错误列表。
5. 导出结果可被 `evaluation.LoadDataset()` 解析。

---

### 5.3 L2 后端 - 评测运行编排、状态与持久化

#### 目标

把当前 CLI 型离线评测能力包装为可由管理台触发和查询的后台任务，但执行主体仍由后端控制。

#### 功能任务

1. 新增评测运行模型：
   - `KBEvalRun`
   - 字段：`id / run_id / dataset_id / baseline_profile / candidate_profile / gate_thresholds / status / progress / case_total / case_finished / report_path / error_msg / created_by / started_at / finished_at / created_at / updated_at`
2. 新增创建运行接口：
   ```http
   POST /api/admin/kb/eval/runs
   ```
   请求：
   ```json
   {
     "dataset_id": 1,
     "baseline_profile": "phase1_baseline",
     "candidate_profile": "phase2_hybrid_rewrite_topk",
     "profiles": [
       { "name": "phase1_baseline", "baseline": true, "mode": "dense" },
       { "name": "phase2_hybrid_rewrite_topk", "candidate": true, "mode": "hybrid", "enable_query_rewrite": true, "enable_dynamic_topk": true }
     ],
     "gate_thresholds": {
       "min_recall_delta": 0.08,
       "max_p95_latency_regression_ratio": 0.2
     }
   }
   ```
3. 后端创建运行后异步执行：
   - 从数据库读取 dataset cases
   - 转换为 `evaluation.DatasetCase`
   - 构建 strategy profiles 和 gate thresholds
   - 调用 `evaluation.Runner.Run()`
   - 保存 `Report` JSON
   - 更新运行状态和进度
4. 新增运行列表接口：
   ```http
   GET /api/admin/kb/eval/runs?dataset_id=&status=&page=1&page_size=20
   ```
5. 新增运行详情接口：
   ```http
   GET /api/admin/kb/eval/runs/{run_id}
   ```
6. 可选新增取消接口：
   ```http
   POST /api/admin/kb/eval/runs/{run_id}/cancel
   ```
   如果后端暂不支持取消，前端只展示状态，不提供取消按钮。

#### 业务边界

1. 前端只发起运行，不直接执行 CLI 或脚本。
2. 同一 dataset 可以有多次 run，历史报告不可被新运行覆盖。
3. 运行失败必须落库 `error_msg`，便于页面展示。
4. P2 运行进度允许粗粒度，最小可按 `case_finished / case_total` 展示；不强制 SSE。

#### 验收

1. draft 或 invalid dataset 不能创建正式运行，返回 400 和可读错误。
2. 创建运行后返回稳定 `run_id`。
3. 运行状态可从 `pending` 变为 `running`，最终进入 `succeeded` 或 `failed`。
4. 运行失败时可以从详情接口看到 `error_msg`。
5. 运行成功后可以查询到 `report_path` 或报告摘要。

---

### 5.4 L3 后端 - 评测报告、失败样本与 trace 关联 API

#### 目标

把 `evaluation.Report` 转换为前端可直接展示的报告结构，并提供失败样本下钻能力。

#### 功能任务

1. 新增报告详情接口：
   ```http
   GET /api/admin/kb/eval/runs/{run_id}/report
   ```
   返回结构对齐 `evaluation.Report`，建议保留原字段：
   ```json
   {
     "dataset_size": 128,
     "generated_at": "2026-05-26T10:00:00Z",
     "baseline": "phase1_baseline",
     "candidate": "phase2_hybrid_rewrite_topk",
     "results": [],
     "contribution": [],
     "comparison": {},
     "gate": {}
   }
   ```
2. 新增失败样本接口：
   ```http
   GET /api/admin/kb/eval/runs/{run_id}/cases?failure_reason=&query_type=&tag=&page=1&page_size=20
   ```
   返回：
   ```json
   {
     "items": [
       {
         "case_id": "q001",
         "query": "golang channel close panic",
         "query_type": "entity",
         "tags": ["golang", "channel"],
         "failure_reason": "recall_miss",
         "baseline_metrics": { "recall_at_k": 0.5, "mrr": 0.25, "ndcg": 0.4, "citation_accuracy": 0.5, "latency_ms": 120 },
         "candidate_metrics": { "recall_at_k": 0, "mrr": 0, "ndcg": 0, "citation_accuracy": 0, "latency_ms": 180 },
         "delta": { "recall_delta": -0.5, "mrr_delta": -0.25, "ndcg_delta": -0.4, "citation_accuracy_delta": -0.5, "latency_delta_ms": 60 },
         "baseline_request_id": "eval-phase1_baseline-q001",
         "candidate_request_id": "eval-phase2_hybrid_rewrite_topk-q001"
       }
     ],
     "total": 1,
     "page": 1,
     "page_size": 20
   }
   ```
3. 后端生成失败原因：
   - relevant 未命中：`recall_miss`
   - MRR 明显下降：`mrr_drop`
   - nDCG 明显下降：`ndcg_drop`
   - Citation Accuracy 下降：`citation_miss`
   - P95 或单 query latency 超阈值：`latency_regression`
   - gate 未通过：`gate_failed`
4. 关联 trace：
   - 使用 `cmd/retrieval-eval` 已有 request_id 规则：`eval-{profile.Name}-{item.ID}`
   - 如果后端没有写入 `KBRetrieveLog`，返回 `trace_available=false` 或不返回 request_id
   - 前端只在 request_id 存在时提供跳转
5. 新增报告导出接口：
   ```http
   GET /api/admin/kb/eval/runs/{run_id}/report/export?format=json|markdown
   ```

#### 业务边界

1. 失败原因由后端根据指标和阈值生成，前端不二次推断。
2. Gate 是否通过由后端返回，前端只展示。
3. trace 缺失是允许状态，页面展示缺口，不合成 request_id。

#### 验收

1. 成功运行的 report 接口可返回完整 `Report`。
2. failure cases 可以按失败原因筛选。
3. request_id 存在时可通过 P1 trace 接口查到检索日志。
4. JSON 和 Markdown 报告可导出。

---

### 5.5 L4 前端 - P2 类型契约、API 路径与导航激活

#### 目标

在 `admin/src/types/kb.ts` 和 `admin/src/config/api.ts` 中补齐 P2 所需类型与路径，并激活 `/evaluation` 导航。

#### 功能任务

1. 在 `admin/src/types/kb.ts` 新增评测类型：
   ```ts
   export type EvalDatasetStatus = 'draft' | 'ready' | 'invalid' | 'archived';
   export type EvalRunStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'canceled';
   export type EvalFailureReason =
     | 'recall_miss'
     | 'citation_miss'
     | 'mrr_drop'
     | 'ndcg_drop'
     | 'latency_regression'
     | 'gate_failed'
     | 'trace_missing';

   export interface CitationTarget {
     document_id?: number;
     chunk_id?: string;
     file_name?: string;
   }

   export interface EvalDataset {
     id: number;
     name: string;
     description?: string;
     kb_id?: number;
     case_count: number;
     status: EvalDatasetStatus;
     created_at: string;
     updated_at: string;
   }

   export interface EvalCase {
     id: number;
     dataset_id: number;
     case_key: string;
     query: string;
     top_k: number;
     relevant_ids: string[];
     citation_targets?: CitationTarget[];
     query_type?: string;
     tags?: string[];
     kb_ids?: number[];
     collection?: string;
     notes?: string;
     validation_status: 'valid' | 'invalid' | 'unchecked';
     validation_errors?: string[];
   }

   export interface EvalStrategyProfile {
     name: string;
     label?: string;
     baseline?: boolean;
     candidate?: boolean;
     mode: string;
     enable_query_rewrite?: boolean;
     enable_dynamic_topk?: boolean;
     enable_advanced_rerank?: boolean;
     candidate_top_k?: number;
     dense_weight?: number;
     sparse_weight?: number;
     min_top_k?: number;
     max_top_k?: number;
     token_budget?: number;
     rewrite_max_expansions?: number;
     rerank_timeout_ms?: number;
     rerank_model?: string;
   }

   export interface EvalAggregateMetrics {
     recall_at_k: number;
     mrr: number;
     ndcg: number;
     citation_accuracy: number;
     p50_latency_ms: number;
     p95_latency_ms: number;
     avg_latency_ms: number;
   }

   export interface EvalRun {
     id: number;
     run_id: string;
     dataset_id: number;
     baseline_profile: string;
     candidate_profile: string;
     status: EvalRunStatus;
     progress: number;
     case_total: number;
     case_finished: number;
     error_msg?: string;
     started_at?: string;
     finished_at?: string;
     created_at: string;
   }
   ```
2. 补充报告相关类型：
   - `EvalStrategyResult`
   - `EvalStrategyDelta`
   - `EvalComparisonSummary`
   - `EvalGateCheck`
   - `EvalGateResult`
   - `EvalReport`
   - `EvalFailureCase`
3. 在 `admin/src/config/api.ts` 的 `KB_ADMIN_API` 中补充 P2 路径：
   ```ts
   LIST_EVAL_DATASETS: `${API_BASE_URL}/admin/kb/eval/datasets`,
   CREATE_EVAL_DATASET: `${API_BASE_URL}/admin/kb/eval/datasets`,
   LIST_EVAL_CASES: (datasetId: number | string) =>
     `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/items`,
   CREATE_EVAL_CASE: (datasetId: number | string) =>
     `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/items`,
   IMPORT_EVAL_CASES: (datasetId: number | string) =>
     `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/items/import`,
   EXPORT_EVAL_CASES: (datasetId: number | string) =>
     `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/items/export`,
   VALIDATE_EVAL_DATASET: (datasetId: number | string) =>
     `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/validate`,
   LIST_EVAL_RUNS: `${API_BASE_URL}/admin/kb/eval/runs`,
   CREATE_EVAL_RUN: `${API_BASE_URL}/admin/kb/eval/runs`,
   GET_EVAL_RUN: (runId: string) => `${API_BASE_URL}/admin/kb/eval/runs/${runId}`,
   GET_EVAL_REPORT: (runId: string) => `${API_BASE_URL}/admin/kb/eval/runs/${runId}/report`,
   LIST_EVAL_FAILURE_CASES: (runId: string) =>
     `${API_BASE_URL}/admin/kb/eval/runs/${runId}/cases`,
   ```
4. 修改 `admin-shell.tsx`：
   - 激活 `/evaluation`
   - 新增子导航：
     - 评测集：`/evaluation/datasets`
     - 评测运行：`/evaluation/runs`
   - 保留 `/quality-monitor`，P2 后接入最近一次评测报告摘要或跳转到 Evaluation。
5. 新建 `/evaluation/page.tsx`，重定向到 `/evaluation/datasets`。

#### 验收

1. TypeScript 编译无类型错误。
2. `/evaluation` 导航可点击，并默认进入 `/evaluation/datasets`。
3. API 路径统一使用 `KB_ADMIN_API`，页面中不硬编码 URL。
4. `/quality-monitor` 与 `/evaluation` 的职责区分清晰：前者展示质量摘要，后者管理评测闭环。

---

### 5.6 L5 前端 - 评测集页面（/evaluation/datasets）

#### 目标

新建评测集页面，支持评测集列表、样本管理、导入导出、校验状态展示。

#### 功能任务

1. 新建路由文件 `admin/src/app/(admin)/evaluation/datasets/page.tsx`，渲染 `EvaluationDatasetsPage`。
2. 新建 `admin/src/components/admin/evaluation-datasets-page.tsx`：
   - 顶部筛选：知识库、状态、关键词
   - 数据集表格列：`name / kb_id / case_count / status / updated_at / actions`
   - 操作：查看样本、校验、导出、创建运行
3. 新建创建数据集弹窗：
   - 字段：`name / description / kb_id`
   - 创建成功后刷新列表
4. 样本管理区域：
   - 选择数据集后展示样本表格
   - 列：`case_key / query / query_type / tags / top_k / relevant_ids_count / citation_targets_count / validation_status`
   - 支持按 query type、tag、validation status、keyword 筛选
5. 新增样本弹窗：
   - 必填：`case_key / query / top_k`
   - 可选：`relevant_ids / citation_targets / query_type / tags / kb_ids / collection / notes`
   - `relevant_ids` 支持一行一个 ID
   - `citation_targets` 支持 JSON 输入或结构化表单，优先简单可控
6. 导入：
   - 上传 JSON 文件
   - 调用 import API
   - 展示导入成功数、失败数、错误明细
7. 校验：
   - 调用 validate API
   - 更新 dataset status 和样本 validation 状态
   - 错误样本在表格中可展开查看 `validation_errors`
8. 导出：
   - 调用 export API
   - 下载 JSON

#### 业务边界

1. 页面不自动补 `relevant_ids`。
2. 页面不把检索结果自动当作 golden answer；从检索实验室保存过来的样本默认 `validation_status=unchecked` 或 `invalid`，直到人工补齐。
3. 样本 query 可以和知识库无关，但创建运行前后端校验必须拦住不可评测样本。

#### 接口依赖

- `GET /api/admin/kb/eval/datasets`
- `POST /api/admin/kb/eval/datasets`
- `GET /api/admin/kb/eval/datasets/{dataset_id}/items`
- `POST /api/admin/kb/eval/datasets/{dataset_id}/items`
- `POST /api/admin/kb/eval/datasets/{dataset_id}/items/import`
- `GET /api/admin/kb/eval/datasets/{dataset_id}/items/export`
- `POST /api/admin/kb/eval/datasets/{dataset_id}/validate`

#### 验收

1. `/evaluation/datasets` 可访问并展示数据集列表。
2. 可以创建数据集并添加样本。
3. 可以导入 JSON 样本，错误明细可见。
4. 可以校验数据集并看到样本级错误。
5. 可以导出评测集 JSON。
6. 接口失败时展示 `Alert` 或 message，不白屏。

---

### 5.7 L6 前端 - 评测运行页面（/evaluation/runs）

#### 目标

新建评测运行页面，支持创建运行、查看历史运行、轮询运行状态、跳转报告。

#### 功能任务

1. 新建路由文件 `admin/src/app/(admin)/evaluation/runs/page.tsx`，渲染 `EvaluationRunsPage`。
2. 新建 `admin/src/components/admin/evaluation-runs-page.tsx`：
   - 顶部筛选：数据集、状态、时间范围
   - 运行表格列：`run_id / dataset_id / baseline_profile / candidate_profile / status / progress / case_finished / case_total / started_at / finished_at / actions`
   - 操作：查看报告、查看错误、取消（仅后端支持时显示）
3. 创建运行弹窗：
   - 选择 dataset
   - 选择 baseline profile 和 candidate profile
   - 填写或选择 profiles JSON
   - 填写 gate thresholds
   - 提交前做基本 JSON 解析校验
4. 状态轮询：
   - 对 `pending/running` 的运行，每 3-5 秒轮询一次详情或列表
   - 运行结束后停止轮询
   - 页面离开时清理 timer
5. 状态展示：
   - `pending` 灰色
   - `running` 蓝色进度条
   - `succeeded` 绿色
   - `failed` 红色并展示错误
   - `canceled` 默认色
6. 创建成功后跳转或高亮该 run。

#### 业务边界

1. 前端只提交 profile 和 thresholds，不执行评测。
2. profile JSON 只做语法校验，业务合法性由后端决定。
3. 如果后端没有 SSE，P2 使用轮询即可，不新增实时通道。

#### 接口依赖

- `GET /api/admin/kb/eval/runs`
- `POST /api/admin/kb/eval/runs`
- `GET /api/admin/kb/eval/runs/{run_id}`
- `POST /api/admin/kb/eval/runs/{run_id}/cancel`（可选）

#### 验收

1. `/evaluation/runs` 可访问并展示运行列表。
2. 可以创建评测运行。
3. running 状态有进度展示，并会自动刷新。
4. succeeded 后可点击进入报告页。
5. failed 时错误信息可见。

---

### 5.8 L7 前端 - 评测报告页与质量监控页接入

#### 目标

新建评测报告页，展示完整 A/B 对比、贡献度、Gate、失败样本，并让 `/quality-monitor` 展示 P2 质量摘要。

#### 功能任务

1. 新建动态路由 `admin/src/app/(admin)/evaluation/reports/[runId]/page.tsx`，渲染 `EvaluationReportPage`。
2. 新建 `admin/src/components/admin/evaluation-report-page.tsx`：
   - 顶部展示：run_id、dataset_size、generated_at、baseline、candidate、Gate 状态
   - 指标卡片：Recall@K、MRR、nDCG、Citation Accuracy、P50/P95 延迟
   - baseline vs candidate 对比表：
     - baseline value
     - candidate value
     - delta
     - delta 标色
   - 贡献度分析表：
     - `strategy`
     - `compared_to`
     - `recall_delta`
     - `mrr_delta`
     - `ndcg_delta`
     - `citation_accuracy_delta`
     - `p95_latency_delta_ms`
   - Gate 检查列表：
     - `name / actual / expected / passed / message`
3. 失败样本表格：
   - 筛选：failure_reason、query_type、tag
   - 列：`case_id / query / failure_reason / recall_delta / mrr_delta / ndcg_delta / citation_delta / latency_delta_ms / trace`
   - trace 操作：
     - baseline request_id 存在时跳转 `/trace-logs/retrieval?request_id=...`
     - candidate request_id 存在时跳转 `/trace-logs/retrieval?request_id=...`
     - request_id 缺失时展示契约缺口
4. 报告导出：
   - JSON
   - Markdown
5. 修改 `quality-monitor-page.tsx`：
   - 从占位页升级为质量摘要页
   - 展示最近一次 succeeded run 的核心指标和 Gate 状态
   - 提供跳转到 `/evaluation/reports/{runId}` 的入口
   - 若没有成功报告，展示空态，不展示假数据

#### 业务边界

1. 图表和表格只展示后端报告，不在前端补算指标。
2. 颜色只表达 delta 方向，不直接代表是否可上线；是否可上线以 Gate 结果为准。
3. 质量监控页只展示摘要，不替代评测集和运行管理。

#### 接口依赖

- `GET /api/admin/kb/eval/runs/{run_id}/report`
- `GET /api/admin/kb/eval/runs/{run_id}/cases`
- `GET /api/admin/kb/eval/runs/{run_id}/report/export?format=json|markdown`
- `GET /api/admin/kb/eval/runs?status=succeeded&page=1&page_size=1`（用于质量监控最近报告）

#### 验收

1. `/evaluation/reports/[runId]` 可展示完整报告。
2. 指标对比和 delta 与后端 report 一致。
3. Gate 失败项清晰可见。
4. 失败样本可筛选、可分页、可跳转 trace。
5. `/quality-monitor` 不再是纯占位页，有真实报告摘要或明确空态。

---

### 5.9 L8 检索实验室保存样本、回归验收、回滚预案与 Phase 3 交接

#### 目标

打通从检索实验室沉淀评测样本的入口，完成 P2 回归验收，并把 Phase 3 所需的策略解释能力交接清楚。

#### 功能任务

**检索实验室保存样本：**

1. 修改 `admin/src/components/admin/retrieval-lab-page.tsx`：
   - 检索成功后显示"保存为评测样本"按钮
   - 点击后弹出保存样本弹窗
2. 保存样本弹窗字段：
   - 选择 dataset
   - `case_key`
   - `query`
   - `top_k`
   - `query_type`
   - `tags`
   - `notes`
   - 可选复制当前结果 chunk_id 到 `relevant_ids` 草稿区，但默认不勾选为 golden
3. 调用：
   ```http
   POST /api/admin/kb/eval/datasets/{dataset_id}/items
   ```
4. 保存成功后提示用户去评测集页面补齐标准答案并校验。

**冒烟测试清单：**

1. 访问 `/evaluation/datasets` 成功，展示评测集列表。
2. 创建 dataset 成功。
3. 添加样本成功。
4. 导入 JSON 样本成功，导入错误可见。
5. 校验 dataset 后状态更新。
6. 访问 `/evaluation/runs` 成功，创建运行成功。
7. running 状态有进度展示。
8. succeeded run 可进入报告页。
9. 报告页展示 metrics、comparison、contribution、gate。
10. 失败样本可跳转 trace。
11. 检索实验室可保存样本到 dataset。
12. `/quality-monitor` 展示最近报告摘要或空态。

**回归测试清单：**

1. P0 知识库管理、文档上传、任务重试/取消不回退。
2. P1 Dashboard 趋势图不受 P2 页面影响。
3. P1 Trace Logs 仍可按 request_id 查询。
4. 检索实验室原有检索、复制 request_id、跳转 trace 功能不回退。
5. API 500 或网络失败时页面有可读提示，不白屏。

**回滚预案：**

1. **评测集 API 异常**：前端保留 `/evaluation/datasets` 页面，但展示 `Alert` 和空态，不影响 P0/P1。
2. **评测运行 API 异常**：隐藏创建运行入口或置灰，历史列表可继续展示。
3. **报告 API 异常**：报告页展示错误提示，保留返回运行列表入口。
4. **失败样本 API 异常**：报告主指标继续可见，失败样本区域单独降级。
5. **检索实验室保存样本异常**：只影响保存入口，不影响检索测试。

**Phase 3 交接清单：**

1. baseline/candidate 指标对比可用，Phase 3 策略中心可复用报告摘要。
2. 失败样本与 trace 链路可用，Phase 3 调试视图可在此基础上补 fusion/rerank/filter 细节。
3. strategy profile 已在评测运行中结构化保存，Phase 3 可映射到 feature flag 和策略版本。
4. Gate 结果已结构化，Phase 3 可用于灰度和回滚决策展示。

#### 验收

1. 冒烟测试清单全通过。
2. 回归测试清单无阻塞问题。
3. 回滚预案可执行。
4. Phase 3 交接清单已确认。

---

## 6. 推荐协作节奏

1. 先完成 `L0`，冻结评测业务对象、指标字段、API 路径和边界。
2. `L1 + L2 + L3` 后端并行推进：
   - `L1` 管评测资产
   - `L2` 管运行编排
   - `L3` 管报告和失败样本
3. 后端提供最小 OpenAPI 或 JSON 示例后，前端进入 `L4`。
4. `L5 + L6 + L7` 前端并行推进：
   - `L5` 依赖 `L1`
   - `L6` 依赖 `L2`
   - `L7` 依赖 `L3`
5. `L8` 统一收口，重点验证 P0/P1 不回退。

---

## 7. 角色分工建议

1. 后端A：负责 `L1`，评测集、样本、导入导出、校验。
2. 后端B：负责 `L2`，评测运行、异步执行、状态持久化。
3. 后端C：负责 `L3`，报告、失败样本、trace 关联、导出。
4. 前端A：负责 `L4 + L5`，类型契约、导航、评测集页面。
5. 前端B：负责 `L6 + L7`，运行页面、报告页面、质量监控摘要。
6. 联调/QA：负责 `L8`，冒烟、回归、回滚预案演练。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0～L8）：
2. 已完成接口：
3. 已冻结字段口径：
4. 评测集能力：
   - 创建：
   - 样本新增：
   - 导入：
   - 导出：
   - 校验：
5. 评测运行能力：
   - 创建：
   - 状态轮询：
   - 失败错误展示：
6. 评测报告能力：
   - 指标展示：
   - delta 展示：
   - Gate 展示：
   - 导出：
7. 失败样本下钻：
   - 支持的失败原因：
   - trace 跳转：
   - trace 缺失处理：
8. 检索实验室保存样本：
   - 可选 dataset：
   - 保存字段：
   - 缺失 golden 处理：
9. 契约缺口记录：
   - 接口：
   - 字段：
   - 影响页面：
   - 是否阻塞 Phase 3：
10. 冒烟测试结果：
11. 回归测试结果：
12. 已知遗留问题：
13. 是否可以进入 Phase 3：是/否

---

## 9. Phase 2 完成后下一步

**P2 完成后交给 P3 的稳定底座：**

1. 离线评测资产可管理：dataset 和 cases 已成为管理台业务对象。
2. 策略质量可量化：baseline/candidate 报告、delta、Gate 可见。
3. 失败样本可下钻：从报告进入 trace 的路径可用。
4. 质量摘要可复用：`/quality-monitor` 已能展示最近评测结果。
5. strategy profile 结构化：Phase 3 可以把 profile 与 feature flag、策略版本、灰度配置关联。

**P3 需要的 API 和能力：**

- 策略开关列表和修改 API
- 策略版本列表和回滚 API
- 检索 trace 扩展字段：route hits、fusion results、rerank results、filter results
- 策略影响分析 API：按 feature flag 或 strategy version 聚合指标变化
- 操作审计：策略开关、回滚、报告导出

---

## 10. 已知遗留问题（P2 不修复）

| 问题 | 原因 | 影响 | 计划阶段 |
|---|---|---|---|
| 不做线上 A/B 实验平台 | P2 只做离线评测闭环 | 无法直接按真实流量分桶比较策略 | P3 |
| 不做策略开关和灰度控制 | 策略中心归 Phase 3 | 报告能证明收益，但不能在 P2 页面直接灰度发布 | P3 |
| 不做父子块和证据拒答调试视图 | 高级检索调试归 Phase 3 | 失败样本只能跳 trace，不能完整解释高级链路 | P3 |
| 不在前端计算指标 | 指标口径必须由后端统一 | 前端依赖 report API 完整性 | 持续约束 |
| 不自动标注 relevant_ids | golden 数据需要人工或外部流程确认 | 从检索实验室保存的样本需要人工补齐后才能进入 ready | 持续约束 |
| 无实时运行推送 | P2 使用轮询即可满足管理台需求 | 运行状态有数秒延迟 | P3/P4 |
| trace 可能缺失 | 离线评测不一定写入 `KBRetrieveLog` | 失败样本可能只能看指标，不能跳转链路 | P3 |

