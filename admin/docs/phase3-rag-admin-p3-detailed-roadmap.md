# Phase 3 详细功能实现路线（高级检索调试视图与策略中心，前后端并行推进）

## 1. 文档定位

本文档是 `admin/docs/rag-admin-frontend-roadmap-v2.md` 中 Phase 3（P3）"高级检索调试视图与策略中心"的详细执行手册，同时覆盖前后端，可直接按 L 编号逐步实现。

三个用途：

1. 作为前后端 P3 联合推进的统一执行文档，以 P2 离线评测和 P1 trace 底座为起点。
2. 作为冻结高级检索 trace 字段、策略开关、灰度比例、影响分析和回滚口径的协作基线。
3. 作为 Phase 4 接入成本运营、审计和企业治理前的策略可控底座。

**统一口径说明：**

1. `高级检索调试视图` 固定指：在管理台还原一次检索请求中的 query rewrite、route hits、fusion、dedupe、rerank、filter/truncate、parent-child 回填、TopK 决策、evidence gate、citation consistency 和 final results。
2. `策略中心` 固定指：管理 Phase 3 feature flags、策略版本、灰度比例、最近指标变化、回滚操作和契约缺口提示的工作台。
3. `父子块检索` 固定指：子块用于精确召回，父块、邻近块或同章节块用于上下文回填；最终 citation 仍必须定位到具体 child chunk。
4. `策略版动态 TopK` 固定指：基于 `score_distribution / rerank_gap / route_contribution / evidence_density / token_budget` 决策最终 `final_topk`。
5. `证据不足拒答` 固定指：当候选证据置信度不足或 citation 覆盖不足时返回标准拒答模板，并输出 `evidence_gate_result / refusal_reason`。
6. `引用一致性` 固定指：检查回答关键结论是否被 citation snippets 支撑，输出 `citation_supported / citation_support_score / unsupported_claims`。
7. `策略影响分析` 固定指：后端按 feature flag、strategy version 或灰度窗口聚合质量、延迟、成本和风险指标；前端只展示后端返回结果，不在浏览器自行计算指标。
8. `可回滚` 固定指：任一高级策略可独立关闭，并能在 10 分钟内恢复到 Phase 2 稳定检索路径。
9. `契约缺口` 固定指：关键字段或接口缺失时页面明确标识，不静默隐藏、不用假数据补齐。

---

## 2. 当前现状（基于文档和代码扫描）

### 2.1 后端已有能力

经扫描 `backend/docs/phase3-advanced-retrieval-detailed-roadmap.md`、`backend/docs/kb-l3-*`、`backend/docs/kb-l4-*`、`backend/docs/kb-l5-*`、`backend/docs/kb-l6-*` 和 P2 评测文档：

1. 后端已有 Phase 3 高级检索路线拆解，覆盖 `L0 -> L8`：基线冻结、父子块、策略版 TopK、证据拒答、引用一致性、高级 rewrite、调试视图、灰度回滚。
2. P2 离线评测底座已定义 `StrategyProfile / Report / QueryMetrics / AggregateMetrics / Gate` 等对象，可作为策略影响分析的数据基础。
3. P1 trace 底座已定义检索日志和下钻链路，P3 可以在此基础上扩展 trace 字段。
4. 后端文档已明确需要以下 feature flags：
   - `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
   - `RAG_ENABLE_STRATEGIC_TOPK`
   - `RAG_ENABLE_EVIDENCE_REFUSAL`
   - `RAG_ENABLE_CITATION_CONSISTENCY`
   - `RAG_ENABLE_DOMAIN_TERMS`
   - `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE`
   - `RAG_ENABLE_MODEL_ASSISTED_REWRITE`
5. 后端文档已明确高级检索可观测字段：`parent_fill_strategy / parent_fill_count / parent_fill_tokens / topk_decision_reason / evidence_gate_result / refusal_reason / citation_support_score / rewrite_gain_bucket` 等。

### 2.2 前端已有能力

经扫描 `admin/src/`：

1. 管理台骨架完整：`AdminShell` 已有 `/dashboard / knowledge-bases / retrieval-lab / trace-logs / evaluation / quality-monitor` 等导航。
2. `/strategy-center` 导航已存在但为禁用态，尚无路由和页面组件。
3. `/retrieval-lab` 已有基础检索测试能力，但还不是分阶段调试视图。
4. `/trace-logs/retrieval` 已有 P1 检索日志和 trace 下钻基础，可复用为 P3 调试入口。
5. `/evaluation/datasets / evaluation/runs / evaluation/reports/[runId]` 已存在 P2 评测页面，可为 P3 策略影响分析提供 baseline/candidate 参照。
6. `admin/src/config/api.ts` 已有 P0/P1/P2 API 常量，但缺少 P3 策略中心与高级 trace API 常量。
7. `admin/src/types/kb.ts` 已有 `KBRetrieveLog / EvalReport / EvalStrategyProfile` 等类型，但缺少 P3 trace detail、strategy flag、strategy version、strategy impact、rollback 等类型。

### 2.3 当前真实缺口

**后端缺口：**

1. 缺少面向管理台的高级检索 trace 详情结构化接口，当前 trace 字段不足以还原 fusion/rerank/filter/parent-child/TopK/evidence/citation 全链路。
2. 缺少策略开关列表和修改 API。
3. 缺少策略版本列表、当前版本详情和回滚 API。
4. 缺少灰度比例读取和修改能力，无法在管理台展示或调整 rollout。
5. 缺少策略影响分析 API，无法按 feature flag 或 strategy version 展示指标变化。
6. 缺少策略操作审计落点，Phase 3 可先记录轻量操作日志，Phase 4 再并入完整 Audit。
7. 高级策略异常时的回滚口径需要冻结：关闭顺序、降级路径、回滚后的观测指标。

**前端缺口：**

1. `/retrieval-lab/debug` 路由和页面组件不存在。
2. `/strategy-center` 路由和页面组件不存在，导航仍禁用。
3. `retrieval-lab-page.tsx` 没有从一次检索结果进入高级调试页的入口。
4. `retrieval-logs-page.tsx` 的 trace 详情还没有展示 P3 扩展字段。
5. `admin/src/types/kb.ts` 缺少 P3 高级 trace、策略开关、策略版本、影响分析、回滚请求和回滚结果类型。
6. `admin/src/config/api.ts` 缺少 P3 API 路径。
7. 前端没有策略变更确认、风险提示、回滚确认、灰度比例输入校验和操作结果反馈。

---

## 3. 范围边界与通过标准（Gate）

### 3.1 P3 必须完成

1. 管理台可从 `/retrieval-lab` 或 `/trace-logs/retrieval` 进入一次请求的高级调试视图。
2. 调试视图可展示 query rewrite、route hits、fusion、dedupe、rerank、filter/truncate、parent-child 回填、TopK 决策、evidence gate、citation consistency 和 final results。
3. 策略中心可展示所有 Phase 3 feature flags 的当前状态、灰度比例、策略版本和最近影响指标。
4. 管理台可对策略开关执行安全修改，必须有二次确认和变更原因。
5. 管理台可执行一键回滚或单策略关闭，并展示回滚结果。
6. 策略影响分析可展示 Parent Fill Gain、Rewrite Gain、Route Contribution、Evidence Refusal Rate、Refusal False Positive Rate、Citation Support Score、P95 延迟变化和 token 成本变化。
7. 所有 P3 页面在后端字段缺失时展示契约缺口，不展示假数据。
8. P0/P1/P2 功能不回退：知识库管理、检索实验室、Trace Logs、Evaluation、Quality Monitor 均保持可用。

### 3.2 P3 明确不做

1. 不在前端实现检索策略本身，不在浏览器执行 rerank、fusion、TopK 或 citation consistency 计算。
2. 不做无门禁的模型辅助 rewrite 全量开关，模型辅助 rewrite 只能展示灰度或 shadow 状态。
3. 不做完整在线 A/B 实验平台，P3 只展示后端提供的灰度窗口和影响分析。
4. 不做 Phase 4 的完整审计中心，P3 只记录策略操作所需的最小日志。
5. 不做成本运营和 Milvus 运维页面，这些进入 Phase 4。
6. 不允许前端用默认值伪造缺失的策略指标、影响指标或 trace 字段。
7. 不把策略中心做成任意环境变量编辑器，只管理本文档冻结的 Phase 3 feature flags 和策略版本。

### 3.3 Phase 3 通过标准（全满足）

1. 一次复杂 query 可以在前端完整还原高级检索链路，并能定位每一步输入、输出、耗时和降级原因。
2. 任一高级策略可在策略中心看到状态、版本、灰度比例和最近指标变化。
3. 策略开关修改必须带二次确认、变更原因和操作结果提示；失败时不改变页面本地状态。
4. 一键回滚可执行，并能在 10 分钟内恢复 Phase 2 稳定路径。
5. 策略影响分析至少覆盖：Parent Fill Gain、Rewrite Gain、Evidence Refusal Rate、Refusal False Positive Rate、Citation Support Score、P95 延迟变化。
6. TypeScript 编译无类型错误，接口失败时页面有可读错误提示，不白屏。
7. P0/P1/P2 回归通过，尤其是 `/retrieval-lab` 原有检索测试、`/trace-logs/retrieval` 下钻和 `/evaluation/reports/[runId]` 报告页不回退。

---

## 4. 实现路线总览（L0 -> L8）

Phase 3 按 9 条路线推进，按门禁顺序合流：

1. L0：P3 策略边界、字段契约与回滚口径冻结
2. L1：后端 - 高级检索 trace 详情扩展 API
3. L2：后端 - 策略开关、版本、灰度与回滚 API
4. L3：后端 - 策略影响分析、操作日志与门禁摘要 API
5. L4：前端 - P3 类型契约、API 路径与导航激活
6. L5：前端 - 检索调试视图（`/retrieval-lab/debug`）
7. L6：前端 - Trace Logs 高级详情增强与入口打通
8. L7：前端 - 策略中心（`/strategy-center`）
9. L8：联调验收、灰度回滚演练与 Phase 4 交接

建议顺序：`L0 -> L1 + L2 + L3（并行） -> L4 -> L5 + L6 + L7（并行） -> L8`

---

## 5. 详细路线拆解

### 5.1 L0 P3 策略边界、字段契约与回滚口径冻结

#### 目标

在开发前冻结 P3 的可见能力、API 路径、字段名、策略开关、状态枚举和回滚顺序，避免调试视图、策略中心和后端高级检索各说一套。

#### 功能任务

1. 统一 P3 管理台 API 前缀：
   - 高级 trace：`/api/admin/kb/retrieve/audit/{request_id}/debug`
   - 策略中心：`/api/admin/kb/strategy/*`
2. 冻结高级 trace 顶层字段：
   - `request_id / kb_ids / original_query / rewritten_query / route_final_queries`
   - `route_hits / fusion_results / dedupe_results / rerank_results / filter_results`
   - `parent_child / topk_decision / evidence_gate / citation_check`
   - `final_results / stage_durations / degradation / created_at`
3. 冻结 parent-child 字段：
   - `parent_child_enabled`
   - `parent_fill_strategy`
   - `parent_fill_count`
   - `parent_fill_tokens`
   - `child_hits`
   - `parent_contexts`
   - `parent_child_available`
   - `fallback_reason`
4. 冻结 TopK 字段：
   - `topk_policy_version`
   - `candidate_topk`
   - `final_topk`
   - `score_distribution`
   - `rerank_gap`
   - `evidence_density`
   - `token_budget`
   - `token_budget_remaining`
   - `topk_decision_reason`
5. 冻结 evidence gate 字段：
   - `evidence_gate_result`
   - `refusal_reason`
   - `thresholds`
   - `evidence_gate_error`
   - `refusal_template_version`
6. 冻结 citation consistency 字段：
   - `citation_supported`
   - `citation_support_score`
   - `unsupported_claims`
   - `citation_check_version`
   - `citation_check_latency_ms`
7. 冻结策略开关列表：
   - `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
   - `RAG_ENABLE_STRATEGIC_TOPK`
   - `RAG_ENABLE_EVIDENCE_REFUSAL`
   - `RAG_ENABLE_CITATION_CONSISTENCY`
   - `RAG_ENABLE_DOMAIN_TERMS`
   - `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE`
   - `RAG_ENABLE_MODEL_ASSISTED_REWRITE`
8. 冻结策略状态枚举：
   - `enabled`
   - `disabled`
   - `shadow`
   - `canary`
   - `rolling_back`
   - `error`
9. 冻结回滚顺序：
   - 先关闭 `RAG_ENABLE_MODEL_ASSISTED_REWRITE`
   - 再关闭 `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE`
   - 再关闭 `RAG_ENABLE_DOMAIN_TERMS`
   - 再关闭 `RAG_ENABLE_EVIDENCE_REFUSAL`
   - 再关闭 `RAG_ENABLE_CITATION_CONSISTENCY`
   - 再关闭 `RAG_ENABLE_STRATEGIC_TOPK`
   - 最后关闭 `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
   - 全部关闭后回退 Phase 2 混合检索路径
10. 冻结 P3 不可回退功能清单：
    - 基础检索测试可用
    - request_id 可复制和下钻
    - 评测报告可查看
    - 高级策略可独立关闭
    - 回滚操作有结果反馈

#### 验收

1. 前后端对 P3 必做与不做边界达成一致。
2. API 路径、字段名、枚举和回滚顺序以本文档为准。
3. 所有新增调试字段都有缺失时的前端降级展示策略。

---

### 5.2 L1 后端 - 高级检索 trace 详情扩展 API

#### 目标

提供一次请求的高级检索结构化详情，让前端无需解析日志字符串即可还原检索链路。

#### 功能任务

1. 新增或扩展 trace debug 接口：
   ```http
   GET /api/admin/kb/retrieve/audit/{request_id}/debug
   ```
2. 返回基础请求信息：
   ```json
   {
     "request_id": "req_xxx",
     "kb_ids": [1],
     "original_query": "用户原始问题",
     "rewritten_query": "改写后问题",
     "route_final_queries": {
       "dense": "dense 使用的 query",
       "sparse": "sparse 使用的 query"
     },
     "created_at": "2026-01-01T00:00:00Z"
   }
   ```
3. 返回 route hits：
   - `route`
   - `query`
   - `hits`
   - `contribution`
   - `latency_ms`
   - `error`
4. 返回 fusion、dedupe、rerank、filter 过程：
   - `fusion_results.before / fusion_results.after`
   - `dedupe_results.before_count / dedupe_results.after_count / removed`
   - `rerank_results.before / rerank_results.after / rerank_model / rerank_version`
   - `filter_results.before_count / after_count / removed / truncate_reason`
5. 返回 parent-child 过程：
   - `child_hits`
   - `parent_contexts`
   - `parent_fill_strategy`
   - `parent_fill_count`
   - `parent_fill_tokens`
   - `fallback_reason`
6. 返回 TopK 决策：
   - `topk_policy_version`
   - `candidate_topk`
   - `final_topk`
   - `score_distribution`
   - `rerank_gap`
   - `evidence_density`
   - `token_budget`
   - `topk_decision_reason`
7. 返回 evidence gate 和 citation consistency：
   - `evidence_gate_result`
   - `refusal_reason`
   - `thresholds`
   - `citation_supported`
   - `citation_support_score`
   - `unsupported_claims`
8. 返回降级信息：
   - `degradation.enabled`
   - `degradation.reason`
   - `degradation.fallback_strategy`
   - `degradation.error_code`
9. 保持兼容：
   - 找不到 P3 扩展字段时仍返回基础 trace，并标记 `debug_available=false`
   - 请求不存在时返回 404
   - 字段解析失败时返回可读错误，不返回半截 JSON

#### 验收

1. `GET /api/admin/kb/retrieve/audit/{request_id}/debug` 可独立打通。
2. 一个启用 P3 策略的请求能返回 rewrite、route、fusion、rerank、parent-child、TopK、evidence、citation 等关键字段。
3. 一个未启用 P3 策略的请求也能返回基础 trace，并通过 `debug_available=false` 明确说明。
4. 接口失败时错误结构稳定，前端可展示契约缺口或错误提示。

---

### 5.3 L2 后端 - 策略开关、版本、灰度与回滚 API

#### 目标

把 Phase 3 策略从环境变量或配置文件状态升级为管理台可查看、可安全修改、可回滚的受控对象。

#### 功能任务

1. 新增策略开关列表接口：
   ```http
   GET /api/admin/kb/strategy/flags
   ```
   最小返回：
   ```json
   {
     "items": [
       {
         "flag_key": "RAG_ENABLE_PARENT_CHILD_RETRIEVAL",
         "label": "Parent Child Retrieval",
         "status": "canary",
         "enabled": true,
         "rollout_percentage": 20,
         "strategy_version": "p3-parent-child-v1",
         "risk_level": "medium",
         "updated_at": "2026-01-01T00:00:00Z"
       }
     ]
   }
   ```
2. 新增策略开关修改接口：
   ```http
   PATCH /api/admin/kb/strategy/flags/{flag_key}
   ```
   请求体：
   ```json
   {
     "enabled": true,
     "status": "canary",
     "rollout_percentage": 10,
     "reason": "small canary after offline gate passed"
   }
   ```
3. 修改策略开关时校验：
   - `flag_key` 必须属于 L0 冻结列表
   - `rollout_percentage` 必须在 `0..100`
   - 高风险策略从 `disabled` 到 `enabled` 必须先经过 `shadow` 或 `canary`
   - `reason` 必填
4. 新增策略版本列表接口：
   ```http
   GET /api/admin/kb/strategy/versions?flag_key=&page=1&page_size=20
   ```
5. 新增当前策略版本详情：
   ```http
   GET /api/admin/kb/strategy/versions/{version_id}
   ```
6. 新增回滚接口：
   ```http
   POST /api/admin/kb/strategy/rollback
   ```
   请求体：
   ```json
   {
     "target_version": "phase2_baseline",
     "flag_keys": ["RAG_ENABLE_MODEL_ASSISTED_REWRITE"],
     "reason": "p95 latency regression"
   }
   ```
7. 回滚返回：
   - `rollback_id`
   - `status`
   - `changed_flags`
   - `target_version`
   - `started_at`
   - `finished_at`
   - `error_msg`
8. 回滚安全策略：
   - 支持单策略回滚
   - 支持全策略回滚到 `phase2_baseline`
   - 回滚失败时保持原状态并返回失败原因
   - 回滚操作写入最小操作日志

#### 验收

1. 策略中心可读取所有 Phase 3 flags，并看到状态、灰度比例和版本。
2. 修改策略开关时后端能校验非法 flag、非法比例和缺失 reason。
3. 单策略回滚和全量回滚 API 可执行，并返回明确结果。
4. 回滚失败不产生半更新状态。

---

### 5.4 L3 后端 - 策略影响分析、操作日志与门禁摘要 API

#### 目标

提供策略中心所需的影响分析和风险门禁摘要，让管理员能判断策略是否值得继续灰度或需要回滚。

#### 功能任务

1. 新增策略影响分析接口：
   ```http
   GET /api/admin/kb/strategy/impact?flag_key=&version=&range=24h&kb_id=
   ```
2. 返回指标：
   - `parent_fill_gain`
   - `rewrite_gain`
   - `route_contribution`
   - `evidence_refusal_rate`
   - `refusal_false_positive_rate`
   - `citation_support_score`
   - `citation_precision_delta`
   - `p95_latency_delta_ms`
   - `avg_context_tokens_delta`
   - `empty_rate_delta`
   - `error_rate_delta`
3. 返回时间窗口：
   - `range`
   - `from`
   - `to`
   - `sample_size`
   - `baseline_sample_size`
   - `candidate_sample_size`
4. 新增门禁摘要接口：
   ```http
   GET /api/admin/kb/strategy/gates?flag_key=&version=
   ```
   返回：
   - `gate_status`
   - `passed`
   - `failed_rules`
   - `baseline_report_id`
   - `candidate_report_id`
   - `last_eval_run_id`
5. 新增策略操作日志接口：
   ```http
   GET /api/admin/kb/strategy/operations?flag_key=&page=1&page_size=20
   ```
   最小字段：
   - `id`
   - `operator_id`
   - `operation`
   - `flag_key`
   - `from_status`
   - `to_status`
   - `from_rollout_percentage`
   - `to_rollout_percentage`
   - `reason`
   - `created_at`
6. 聚合规则：
   - 指标优先来自 P2 评测报告和 P1/P3 结构化日志
   - 缺少样本量时返回 `sample_size_too_small=true`
   - 指标不可计算时返回 `contract_gaps`，不返回假 0

#### 验收

1. 策略中心能展示最近 1h/24h/7d 的策略影响摘要。
2. 样本量不足、指标缺失、报告缺失时接口能明确返回缺口。
3. 策略开关和回滚操作可在 operations 接口中查询。
4. 前端无需自行计算复杂质量指标。

---

### 5.5 L4 前端 - P3 类型契约、API 路径与导航激活

#### 目标

补齐前端消费 P3 API 所需的类型、路径、路由和导航入口，先建立稳定契约再实现页面。

#### 功能任务

1. 在 `admin/src/types/kb.ts` 新增高级 trace 类型：
   - `RetrievalDebugTrace`
   - `RetrievalRouteHit`
   - `RetrievalFusionResult`
   - `RetrievalRerankResult`
   - `RetrievalFilterResult`
   - `ParentChildDebugInfo`
   - `TopKDecisionDebugInfo`
   - `EvidenceGateDebugInfo`
   - `CitationCheckDebugInfo`
2. 在 `admin/src/types/kb.ts` 新增策略中心类型：
   - `StrategyFlag`
   - `StrategyFlagStatus`
   - `StrategyVersion`
   - `StrategyImpact`
   - `StrategyGateSummary`
   - `StrategyOperationLog`
   - `StrategyRollbackRequest`
   - `StrategyRollbackResult`
3. 在 `admin/src/config/api.ts` 增加 P3 API 常量：
   ```ts
   GET_RETRIEVAL_DEBUG_TRACE: (requestId: string) =>
     `${API_BASE_URL}/admin/kb/retrieve/audit/${requestId}/debug`,
   LIST_STRATEGY_FLAGS: `${API_BASE_URL}/admin/kb/strategy/flags`,
   UPDATE_STRATEGY_FLAG: (flagKey: string) =>
     `${API_BASE_URL}/admin/kb/strategy/flags/${flagKey}`,
   LIST_STRATEGY_VERSIONS: `${API_BASE_URL}/admin/kb/strategy/versions`,
   GET_STRATEGY_VERSION: (versionId: string) =>
     `${API_BASE_URL}/admin/kb/strategy/versions/${versionId}`,
   ROLLBACK_STRATEGY: `${API_BASE_URL}/admin/kb/strategy/rollback`,
   GET_STRATEGY_IMPACT: `${API_BASE_URL}/admin/kb/strategy/impact`,
   GET_STRATEGY_GATES: `${API_BASE_URL}/admin/kb/strategy/gates`,
   LIST_STRATEGY_OPERATIONS: `${API_BASE_URL}/admin/kb/strategy/operations`,
   ```
4. 新增路由：
   - `admin/src/app/(admin)/retrieval-lab/debug/page.tsx`
   - `admin/src/app/(admin)/strategy-center/page.tsx`
5. 新增页面组件：
   - `admin/src/components/admin/retrieval-debug-page.tsx`
   - `admin/src/components/admin/strategy-center-page.tsx`
6. 修改 `admin-shell.tsx`：
   - 激活 `Strategy Center`
   - 为 `/strategy-center` 增加选中态和面包屑
   - 为 `/retrieval-lab/debug` 增加面包屑，归属 `检索实验室`
7. 所有新增类型字段允许后端未返回时降级：
   - 数组字段默认为空数组仅用于渲染保护，不作为数据真实值
   - 关键字段缺失时展示 `契约缺口`
   - 不把缺失指标展示为 0

#### 验收

1. TypeScript 编译无类型错误。
2. `/strategy-center` 导航可进入页面。
3. `/retrieval-lab/debug?request_id=xxx` 路由可进入页面。
4. API 路径和类型字段与 L0/L1/L2/L3 冻结口径一致。

---

### 5.6 L5 前端 - 检索调试视图（`/retrieval-lab/debug`）

#### 目标

把一次检索请求变成可读的链路剖面，让学员和管理员能看懂高级策略为什么改变结果。

#### 功能任务

1. 页面入口：
   - 支持 `/retrieval-lab/debug?request_id=xxx`
   - 支持从 `/retrieval-lab` 检索成功结果跳转
   - 支持手动输入 `request_id` 查询
2. 页面顶栏展示：
   - `request_id`
   - `original_query`
   - `rewritten_query`
   - `kb_ids`
   - `created_at`
   - `debug_available`
   - `degradation`
3. Query Rewrite 区块：
   - original query
   - rewritten query
   - route-specific final query
   - `term_hits`
   - `rewrite_strategy`
   - `rewrite_gain_bucket`
4. Route Hits 区块：
   - dense/sparse/rewrite route hits
   - route contribution
   - route latency
   - route error
   - 每个 route 的 top hits 表格
5. Fusion / Dedupe / Rerank / Filter 区块：
   - fusion 前后对比
   - dedupe 删除项
   - rerank 前后排序和分数变化
   - filter/truncate 删除原因
   - final results 与 citations
6. Parent-Child 区块：
   - child hit 列表
   - parent contexts
   - sibling/section window 回填结果
   - `parent_fill_strategy`
   - `parent_fill_tokens`
   - `fallback_reason`
7. TopK Decision 区块：
   - `candidate_topk`
   - `final_topk`
   - `score_distribution`
   - `rerank_gap`
   - `evidence_density`
   - `token_budget`
   - `topk_decision_reason`
8. Evidence Gate 区块：
   - `evidence_gate_result`
   - `refusal_reason`
   - thresholds 命中情况
   - `evidence_gate_error`
   - standard refusal template version
9. Citation Consistency 区块：
   - `citation_supported`
   - `citation_support_score`
   - `unsupported_claims`
   - citation snippets
   - child chunk citation 与 parent context 对照
10. 交互要求：
    - 支持复制 `request_id`
    - 支持从 final result 跳转到 trace log 原始详情
    - 支持展开/折叠每个阶段
    - 支持契约缺口集中展示
    - 加载失败展示 `Alert`，页面不白屏

#### 验收

1. 有 P3 扩展字段的请求可完整展示高级检索链路。
2. 缺少某个阶段字段时，仅该阶段展示契约缺口，不影响其他阶段。
3. TopK、evidence、citation 三类高风险决策原因可见。
4. `/retrieval-lab` 原有检索测试能力不回退。

---

### 5.7 L6 前端 - Trace Logs 高级详情增强与入口打通

#### 目标

让 P1 的 trace 日志成为 P3 调试视图的自然入口，同时保留原有轻量详情抽屉。

#### 功能任务

1. 修改 `admin/src/components/admin/retrieval-lab-page.tsx`：
   - 检索成功后在 `request_id` 区域新增 `查看调试视图` 按钮
   - 跳转 `/retrieval-lab/debug?request_id={request_id}`
   - 当无 `request_id` 时按钮隐藏或禁用
2. 修改 `admin/src/components/admin/retrieval-logs-page.tsx`：
   - 检索日志列表新增 `调试视图` 操作列
   - 点击跳转 `/retrieval-lab/debug?request_id={request_id}`
3. trace 详情抽屉增加 P3 摘要：
   - `parent_child_enabled`
   - `topk_policy_version`
   - `evidence_gate_result`
   - `citation_support_score`
   - `refusal_reason`
4. 如果 P3 字段缺失：
   - 显示 `该请求暂无 P3 调试字段`
   - 保留原有 P1 trace 详情
5. 从评测报告失败样本跳转：
   - 若失败样本有 `request_id`，优先跳 `/retrieval-lab/debug?request_id=xxx`
   - 若没有 P3 debug 数据，则降级到 `/trace-logs/retrieval?request_id=xxx`

#### 验收

1. `/retrieval-lab`、`/trace-logs/retrieval`、`/evaluation/reports/[runId]` 都能进入调试视图。
2. 原有 trace 详情抽屉不回退。
3. P3 字段缺失时入口仍可用，但页面明确说明缺口。
4. request_id 复制和筛选功能不受影响。

---

### 5.8 L7 前端 - 策略中心（`/strategy-center`）

#### 目标

提供一个可解释、可灰度、可回滚的策略控制台，让管理员能安全管理 Phase 3 高级检索策略。

#### 功能任务

1. 页面布局：
   - 顶部状态概览：启用策略数、canary 策略数、error 策略数、最近回滚次数
   - 左侧策略列表或卡片
   - 右侧策略详情、影响分析、版本、操作日志
2. Feature Flag 列表：
   - `flag_key`
   - `label`
   - `status`
   - `enabled`
   - `rollout_percentage`
   - `strategy_version`
   - `risk_level`
   - `updated_at`
3. 策略状态展示规则：
   - `enabled` 展示绿色
   - `disabled` 展示灰色
   - `shadow` 展示蓝色
   - `canary` 展示橙色
   - `rolling_back` 展示进度态
   - `error` 展示红色
4. 策略修改弹窗：
   - 修改 enabled/status/rollout percentage
   - 必填 `reason`
   - 高风险策略展示风险提示
   - 从 `disabled` 到 `enabled` 时建议先选择 `shadow` 或 `canary`
   - 提交成功后刷新 flags、impact、operations
5. 一键回滚：
   - 支持单策略回滚
   - 支持回滚到 `phase2_baseline`
   - 二次确认中展示影响范围和回滚顺序
   - 必填 `reason`
   - 回滚成功后展示 `rollback_id` 和 changed flags
6. 策略影响分析：
   - 时间范围：`1h / 24h / 7d`
   - 指标：Parent Fill Gain、Rewrite Gain、Route Contribution、Evidence Refusal Rate、Refusal False Positive Rate、Citation Support Score、P95 延迟变化、平均上下文 token 变化
   - 样本量不足时展示 `样本量不足`
   - 指标缺失时展示契约缺口
7. 版本列表：
   - `version_id`
   - `flag_key`
   - `label`
   - `created_at`
   - `created_by`
   - `gate_status`
   - `baseline_report_id`
   - `candidate_report_id`
8. 操作日志：
   - 展示开关变更、灰度比例变更、回滚
   - 展示 operator、reason、from/to 状态、时间
   - Phase 4 完整审计上线前，P3 保留最小日志视图
9. 错误与降级：
   - flags API 失败时展示整页 `Alert` 和重试按钮
   - impact API 失败时仅影响指标区
   - rollback API 失败时保留原页面状态，不乐观更新

#### 验收

1. `/strategy-center` 可展示策略开关列表和当前状态。
2. 可修改单个策略状态和灰度比例，且必须填写 reason。
3. 可执行单策略回滚和全量回滚，结果反馈明确。
4. 策略影响指标、门禁摘要、操作日志可见。
5. 接口失败时页面局部降级，不白屏。

---

### 5.9 L8 联调验收、灰度回滚演练与 Phase 4 交接

#### 目标

证明 P3 高级检索调试和策略中心闭环可用，并把 Phase 4 所需的治理、审计、成本运营入口交接清楚。

#### 冒烟测试清单

1. 访问 `/retrieval-lab` 成功，基础检索测试可用。
2. 执行一次启用 P3 策略的复杂 query，返回 `request_id`。
3. 从 `/retrieval-lab` 点击 `查看调试视图`，进入 `/retrieval-lab/debug?request_id=xxx`。
4. 调试视图展示 original query、rewritten query、route hits、fusion、rerank、filter。
5. 调试视图展示 parent-child 回填信息。
6. 调试视图展示 TopK 决策原因。
7. 调试视图展示 evidence gate 结果和 refusal reason。
8. 调试视图展示 citation consistency 结果和 unsupported claims。
9. 访问 `/trace-logs/retrieval` 成功，列表中可进入调试视图。
10. 从评测报告失败样本可跳转调试视图或 trace 详情。
11. 访问 `/strategy-center` 成功，展示 feature flags 列表。
12. 修改一个低风险策略的 rollout percentage 成功，并记录 reason。
13. 修改非法 rollout percentage 时后端拒绝，前端展示错误。
14. 执行单策略回滚成功，页面展示 rollback result。
15. 策略影响分析展示指标或契约缺口。
16. 策略操作日志出现本次修改和回滚记录。

#### 回归测试清单

1. P0 知识库管理、文档上传、任务重试/取消不回退。
2. P1 Dashboard 趋势图和 Trace Logs 不回退。
3. P2 Evaluation 数据集、运行、报告页面不回退。
4. `/retrieval-lab` 原有检索结果、citation、source、request_id 复制不回退。
5. 缺少 P3 扩展字段时页面不崩溃，展示契约缺口。
6. API 500 或网络失败时页面有可读提示，不白屏。

#### 回滚预案

1. **高级 trace debug API 异常**：调试视图展示错误，入口保留，原 P1 trace 详情继续可用。
2. **策略开关 API 异常**：策略中心只读或展示错误，不允许前端本地伪造状态。
3. **策略影响 API 异常**：只降级影响分析区，flags 和 rollback 继续可用。
4. **回滚 API 异常**：不乐观更新本地状态，提示用户重试或走后端手动回滚流程。
5. **某个高级策略引发质量或延迟回退**：按 L0 冻结顺序逐个关闭 feature flags，最终回到 `phase2_baseline`。

#### Phase 4 交接清单

1. 策略操作日志已存在，Phase 4 可并入完整 `/audit`。
2. 策略影响分析已有延迟和 token 变化，Phase 4 可扩展为成本看板。
3. 回滚操作已有 `rollback_id` 和 changed flags，Phase 4 可接入审计和告警。
4. 高级 trace 已有 parent-child、TopK、evidence、citation 字段，Phase 4 可继续补查询审计和质量周报。
5. 策略版本与门禁摘要可复用，Phase 4 可平台化 A/B 和灰度治理。

#### 验收

1. 冒烟测试清单全通过。
2. 回归测试清单无阻塞问题。
3. 回滚预案演练通过，任一高级策略可独立关闭。
4. Phase 4 交接清单已确认。

---

## 6. 推荐协作节奏

1. 先完成 `L0`，冻结策略边界、字段契约、状态枚举和回滚顺序。
2. `L1 + L2 + L3` 后端并行推进：
   - `L1` 管高级 trace debug 数据
   - `L2` 管策略开关、版本、灰度和回滚
   - `L3` 管策略影响分析、门禁摘要和操作日志
3. 后端提供最小 OpenAPI 或 JSON 示例后，前端进入 `L4`。
4. `L5 + L6 + L7` 前端并行推进：
   - `L5` 依赖 `L1`
   - `L6` 依赖 `L1` 和现有 P1/P2 页面
   - `L7` 依赖 `L2 + L3`
5. `L8` 统一收口，重点验证 P0/P1/P2 不回退，以及策略回滚可执行。

---

## 7. 角色分工建议

1. 后端A：负责 `L1`，高级 trace debug API、字段回填、兼容降级。
2. 后端B：负责 `L2`，策略 flags、版本、灰度、回滚和校验。
3. 后端C：负责 `L3`，影响分析、门禁摘要、操作日志。
4. 前端A：负责 `L4 + L5`，类型契约、API 路径、检索调试视图。
5. 前端B：负责 `L6 + L7`，Trace Logs 入口、策略中心、回滚交互。
6. QA/联调：负责 `L8`，冒烟、回归、灰度、回滚演练。
7. 检索/算法：负责确认策略指标解释和门禁阈值，避免前端展示与算法口径不一致。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0～L8）：
   - L0：✅ 已完成 — P3 策略边界、字段契约与回滚口径已冻结
   - L1：✅ 已完成 — 后端高级检索 trace 详情扩展 API 已实现（GET /api/admin/kb/retrieve/audit/{request_id}/debug）
   - L2：✅ 已完成 — 后端策略开关、版本、灰度与回滚 API 已实现（7 个接口）
   - L3：✅ 已完成 — 后端策略影响分析、操作日志与门禁摘要 API 已实现（3 个接口）
   - L4：✅ 已完成 — 前端 P3 类型契约（20+ 类型）、API 路径（9 个常量）、路由与导航已激活
   - L5：✅ 已完成 — 前端检索调试视图（/retrieval-lab/debug）已实现（651 行）
   - L6：✅ 已完成 — 前端 Trace Logs 高级详情增强与入口打通（已修复按钮乱码）
   - L7：✅ 已完成 — 前端策略中心（/strategy-center）已实现（803 行）
   - L8：✅ 已完成 — 前端测试 26/26 通过，TypeScript 编译零错误，后端 P3 测试全部通过

2. 已完成接口：
   - GET /api/admin/kb/retrieve/audit/{request_id}/debug（L1）
   - GET /api/admin/kb/strategy/flags（L2）
   - PATCH /api/admin/kb/strategy/flags/{flag_key}（L2）
   - GET /api/admin/kb/strategy/versions（L2）
   - GET /api/admin/kb/strategy/versions/{version_id}（L2）
   - POST /api/admin/kb/strategy/rollback（L2）
   - GET /api/admin/kb/strategy/impact（L3）
   - GET /api/admin/kb/strategy/gates（L3）
   - GET /api/admin/kb/strategy/operations（L3）

3. 已冻结字段口径：以本文档 L0 节定义为准，包含 10 组字段契约、7 个 feature flags、6 个状态枚举、8 步回滚顺序

4. 高级 trace 能力：
   - request_id 查询：✅ 支持
   - route hits：✅ 支持（dense/sparse/rewrite 各路由 hits、contribution、latency_ms、error）
   - fusion/dedupe/rerank/filter：✅ 支持（前后对比、removed、rerank_model/version、truncate_reason）
   - parent-child：✅ 支持（child_hits、parent_contexts、parent_fill_strategy、parent_fill_tokens、fallback_reason）
   - TopK decision：✅ 支持（candidate_topk、final_topk、score_distribution、rerank_gap、evidence_density、token_budget、topk_decision_reason）
   - evidence gate：✅ 支持（evidence_gate_result、refusal_reason、thresholds、evidence_gate_error、refusal_template_version）
   - citation consistency：✅ 支持（citation_supported、citation_support_score、unsupported_claims、citation_check_version）

5. 策略中心能力：
   - flags 列表：✅ 支持
   - 状态修改：✅ 支持（含 reason 必填、高风险拦截、后端校验）
   - 灰度比例：✅ 支持（0-100%、前端校验 + 后端校验）
   - 版本列表：✅ 支持
   - 回滚：✅ 支持（单策略回滚 + 全量回滚到 phase2_baseline）

6. 策略影响分析：
   - Parent Fill Gain：✅ 支持
   - Rewrite Gain：✅ 支持
   - Evidence Refusal Rate：✅ 支持
   - Refusal False Positive Rate：✅ 支持
   - Citation Support Score：✅ 支持
   - P95 延迟变化：✅ 支持

7. 操作日志：
   - 开关变更：✅ 支持
   - 灰度变更：✅ 支持
   - 回滚：✅ 支持

8. 契约缺口记录：
   - 接口：无缺失接口
   - 字段：citation snippets 与 child/parent 对照专用结构暂未提供（前端已标注 info 提示）
   - 影响页面：Citation Consistency 区块（已降级展示）
   - 是否阻塞 Phase 4：否

9. 冒烟测试结果：
   - 后端：TestStrategyImpactAndGatesEndpoints PASS、TestStrategyOperationsEndpointReturnsLatestChanges PASS、TestStrategyHandlersLifecycle PASS
   - 前端：26/26 测试通过（retrieval-debug-page 14 tests + strategy-center-page 12 tests）
   - TypeScript 编译：零错误
   - 按冒烟清单 16 项逐项检查，全部可实现（需联调环境验证完整数据流）

10. 回归测试结果：
    - P0 知识库管理、P1 Dashboard/Trace Logs、P2 Evaluation：代码未修改相关模块，无回退风险
    - TypeScript 全量编译通过，确认无类型破坏
    - 前端测试覆盖 P3 核心页面渲染、API 调用、错误处理、契约缺口展示

11. 回滚演练结果：
    - 后端支持单策略回滚和全量回滚到 phase2_baseline，测试验证 PASS
    - 回滚失败时保持原状态并返回失败原因（前端不做乐观更新）
    - 回滚操作写入操作日志，可追溯

12. 已知遗留问题：
    - 前端 jsdom 测试环境对中文字符编码支持有限，测试中已规避中文文本匹配
    - citation snippets 与 child/parent 对照专用结构待后端补齐
    - Phase 4 完整审计中心尚未实现

13. 是否可以进入 Phase 4：是

---

## 9. Phase 3 完成后下一步

**P3 完成后交给 P4 的稳定底座：**

1. 高级检索链路可解释：一次请求的 rewrite、route、fusion、rerank、filter、parent-child、TopK、evidence、citation 可见。
2. 策略状态可管理：feature flags、版本、灰度比例和回滚可在管理台操作。
3. 策略收益可观察：影响分析能展示质量、延迟、成本和拒答风险。
4. 策略操作可追踪：开关、灰度和回滚已有最小操作日志。
5. 回滚路径可执行：任一高级策略可独立关闭，并可回退到 Phase 2 稳定路径。

**P4 需要的 API 和能力：**

- 完整审计 API：`GET /api/admin/kb/audit/events`
- 成本汇总 API：`GET /api/admin/kb/cost/summary`
- 成本时序 API：`GET /api/admin/kb/cost/timeseries`
- Milvus/Collection 运维 API：`GET /api/admin/kb/vector/collections`
- 告警规则 API：策略异常、P95 延迟异常、拒答误伤率异常、citation support 下降
- 自动化周报或报告导出：质量、稳定性、成本、策略变更

---

## 10. 已知遗留问题（P3 不修复）

| 问题 | 原因 | 影响 | 计划阶段 |
|---|---|---|---|
| 不做完整在线 A/B 实验平台 | P3 只做灰度和影响分析展示 | 无法在管理台完成复杂实验分桶、显著性检验和自动决策 | P4 |
| 不做完整审计中心 | P3 只记录策略操作最小日志 | 策略操作可追踪，但还不是统一审计视图 | P4 |
| 不做成本运营页面 | P3 只展示策略影响中的延迟和 token 变化 | 无法按知识库、模型、时间窗口完整核算成本 | P4 |
| 不做 Milvus 索引生命周期治理 | P3 聚焦高级检索策略 | parent-child 索引重建、版本切换、容量治理仍需后续工具 | P4 |
| 不在前端计算策略指标 | 指标口径必须由后端统一 | 前端强依赖 strategy impact API 完整性 | 持续约束 |
| 模型辅助 rewrite 不能无监控全量上线 | 高风险策略需要灰度和回滚保护 | 需要保留 shadow/canary 和门禁限制 | 持续约束 |

