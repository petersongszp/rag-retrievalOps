# 后台管理页面《策略中心》使用说明

## 1. 文档范围与代码依据

本文只基于当前仓库中的真实代码整理，不按产品想象补充未落地能力。

本次核对的主要代码如下：

- 前端页面与路由
  - `admin/src/app/(admin)/strategy-center/page.tsx`
  - `admin/src/components/admin/strategy-center-page.tsx`
  - `admin/src/components/admin/admin-shell.tsx`
- 前端 API 与类型
  - `admin/src/config/api.ts`
  - `admin/src/services/api/client.ts`
  - `admin/src/types/kb.ts`
- 后端路由与 handler
  - `backend/api/router/custom_kb.go`
  - `backend/api/handler/kb/handler_strategy.go`
  - `backend/api/handler/kb/handler_strategy_insights.go`
- 后端策略状态、配置与模型
  - `backend/internal/rag/phase3/contract.go`
  - `backend/internal/rag/phase3admin/state.go`
  - `backend/internal/config/config.go`
  - `backend/internal/model/kb_retrieve_log.go`
  - `backend/internal/model/kb_eval_run.go`
  - `backend/internal/model/kb_audit_event.go`
- 关联的评测模块
  - `admin/src/components/admin/evaluation-datasets-page.tsx`
  - `admin/src/components/admin/evaluation-runs-page.tsx`
  - `admin/src/components/admin/evaluation-report-page.tsx`
  - `backend/api/handler/kb/handler_eval_dataset.go`
  - `backend/api/handler/kb/handler_eval_run.go`
  - `backend/api/handler/kb/handler_eval_report.go`
  - `backend/internal/milvus/evaluation/types.go`
  - `backend/internal/milvus/evaluation/profiles.go`
  - `backend/internal/milvus/evaluation/gate.go`

说明：

- 文中凡写到“代码中未找到直接依据”或“需后端确认”，都表示当前仓库里没有足够证据，不能编造。
- 当前仓库里确实存在一个独立的“评测”模块，但没有看到名字就叫“测试中心”或“测中心”的独立页面。

## 2. 策略中心是什么

策略中心是后台管理系统中专门管理 **Phase 3 检索策略开关** 的页面，路由为 `/strategy-center`，菜单名为“策略中心”。

它解决的问题不是“写策略代码”，而是“把已经实现的检索策略安全地上线、灰度、观察、回滚”：

1. 看当前受管的 Phase 3 策略有哪些。
2. 看每个策略当前是否开启、处于什么状态、风险等级如何。
3. 看策略影响、评测 Gate、版本记录和操作日志。
4. 在需要时修改开关或回滚到 Phase 2 基线。

适合使用的人：

1. 后台管理员。
2. 负责 RAG 检索策略的研发同学。
3. 需要观察上线效果和回滚风险的运维/值班同学。
4. 需要了解“当前系统到底开了哪些策略”的产品或运营同学。

需要特别说明的是：策略中心是一个 **治理台**，不是策略逻辑本身。真正的检索逻辑仍然在后端检索实现里，例如：

- `backend/internal/milvus/retrieval/parent_child.go`
- `backend/internal/milvus/retrieval/topk_policy.go`
- `backend/internal/milvus/retrieval/evidence_gate.go`
- `backend/internal/milvus/retrieval/citation_consistency.go`
- `backend/internal/milvus/retrieval/rewrite.go`

## 3. 页面入口与关联模块

### 3.1 页面入口

- 路由文件：`admin/src/app/(admin)/strategy-center/page.tsx`
- 页面组件：`admin/src/components/admin/strategy-center-page.tsx`
- 菜单配置：`admin/src/components/admin/admin-shell.tsx`

### 3.2 后端接口入口

后端接口统一注册在 `backend/api/router/custom_kb.go`，并且在 `adminOnly` 分支下，说明它是 **管理员接口**。

当前页面实际使用的接口：

1. `GET /api/admin/kb/strategy/flags`
2. `PATCH /api/admin/kb/strategy/flags/:flag_key`
3. `GET /api/admin/kb/strategy/versions`
4. `POST /api/admin/kb/strategy/rollback`
5. `GET /api/admin/kb/strategy/impact`
6. `GET /api/admin/kb/strategy/gates`
7. `GET /api/admin/kb/strategy/operations`

### 3.3 与“评测”模块的关系

代码中没有看到名为“测试中心/测中心”的独立页面；当前最接近、并且和策略中心直接相关的，是“评测”模块：

1. `/evaluation/datasets`：管理评测数据集与样本。
2. `/evaluation/runs`：创建和查看评测运行。
3. `/evaluation/reports/[runId]`：查看评测报告与失败样例。

策略中心顶部按钮“查看评测运行”会直接跳到 `/evaluation/runs`。

更重要的是：

- `Impact` 数据依赖 `kb_retrieve_log` 检索日志，并会尝试叠加最近一次相关的成功评测结果。
- `Gate 摘要` 直接依赖最近一次相关的成功评测报告。

也就是说，**没有评测数据集、没有评测运行、没有成功报告时，策略中心的监控和 Gate 会明显变弱甚至为空。**

## 4. 当前页面的核心功能

### 4.1 已落地能力

1. 查看受管策略列表，并单选某个策略查看详情。
2. 查看顶部概览卡片：
   - 启用策略数
   - Canary 策略数
   - Error 策略数
   - 最近回滚次数
3. 查看策略详情：
   - 基本信息
   - Impact 分析
   - Gate 摘要
   - 版本列表
   - 操作日志
4. 修改策略：
   - `enabled`
   - `status`
   - `rollout_percentage`
   - `reason`
5. 回滚：
   - 回滚当前策略到 `phase2_baseline`
   - 按冻结顺序把全部 Phase 3 策略回滚到 `phase2_baseline`
6. 刷新页面数据。

### 4.2 当前没有在页面上落地，但后端已有能力

1. 后端支持按 `version` 查看某个策略版本的 Gate/Impact 关联，但当前前端没有传 `version` 参数。
2. 后端的 `rollback` 支持回滚到某个保存过的 `target_version`，但当前前端没有提供从版本列表中选择目标版本的 UI。
3. 后端 `impact` 支持 `kb_id` 过滤，但当前前端没有提供知识库筛选。

### 4.3 当前实现里的关键限制

1. **绝大多数策略的 `status` 和 `rollout_percentage` 目前更像治理元数据，而不是真正的流量控制器。**
2. 真正和运行时行为绑定最明显的，当前只有 `RAG_ENABLE_MODEL_ASSISTED_REWRITE`，它会联动 `RAG.Phase3.ModelRewriteShadowRatio`。
3. 版本列表和操作日志目前存放在 `backend/internal/rag/phase3admin/state.go` 的进程内内存里，不是数据库持久化记录。

## 5. 当前受管策略清单

受管策略定义在 `backend/internal/rag/phase3/contract.go`，展示标签、风险等级和内置版本定义在 `backend/internal/rag/phase3admin/state.go`。

| 页面展示名 | `flag_key` | 风险等级 | 内置 `strategy_version` | 当前仓库 `backend/config.yaml` 默认开关 |
| --- | --- | --- | --- | --- |
| Parent Child Retrieval | `RAG_ENABLE_PARENT_CHILD_RETRIEVAL` | `medium` | `p3-parent-child-v1` | `true` |
| Strategic TopK | `RAG_ENABLE_STRATEGIC_TOPK` | `medium` | `p3-strategic-topk-v1` | `false` |
| Evidence Refusal | `RAG_ENABLE_EVIDENCE_REFUSAL` | `medium` | `p3-evidence-refusal-v1` | `false` |
| Citation Consistency | `RAG_ENABLE_CITATION_CONSISTENCY` | `medium` | `p3-citation-consistency-v1` | `false` |
| Domain Terms | `RAG_ENABLE_DOMAIN_TERMS` | `low` | `p3-domain-terms-v1` | `false` |
| Route Specific Rewrite | `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE` | `medium` | `p3-route-specific-rewrite-v1` | `false` |
| Model Assisted Rewrite | `RAG_ENABLE_MODEL_ASSISTED_REWRITE` | `high` | `p3-model-assisted-rewrite-v1` | `false` |

注意：

1. 这里的“默认开关”只代表仓库中的 `backend/config.yaml`。
2. 实际运行态还会受环境变量覆盖，例如 `RAG_ENABLE_*` 与 `RAG_MODEL_REWRITE_SHADOW_RATIO`。
3. 页面最终看到的是运行时 `config.Global` 加上 `phase3admin` 内存状态，不一定等于仓库静态文件。

## 6. 字段说明

下面的字段说明只列出 **当前策略中心页面上真实出现** 的字段和操作。

### 6.1 顶部概览与筛选

| 页面字段 | 前端来源 | 后端来源 | 含义 | 取值/默认/是否必填 |
| --- | --- | --- | --- | --- |
| 启用策略数 | `flags.filter(item => item.enabled).length` | `GET /strategy/flags` 返回的 `items[].enabled` | 当前被标记为启用的策略数量 | 只读；不是后端直接返回字段 |
| Canary 策略数 | `flags.filter(item => item.status === 'canary').length` | `items[].status` | 当前状态为 `canary` 的策略数量 | 只读；不是后端直接返回字段 |
| Error 策略数 | `flags.filter(item => item.status === 'error').length` | `items[].status` | 当前状态为 `error` 的策略数量 | 只读；不是自动故障检测结果 |
| 最近回滚次数 | `overviewOperations.filter(item => item.operation === 'rollback').length` | `GET /strategy/operations?page=1&page_size=50` | 最近加载到前端的 50 条操作日志里，`rollback` 的条数 | 只读；不是全量历史回滚次数 |
| 时间范围 | `selectedRange` | `GET /strategy/impact?range=` | Impact 分析的时间窗口 | 枚举：`1h` / `24h` / `7d`；默认 `24h` |

### 6.2 策略列表、详情和修改表单

| 页面字段 | API 字段 | 后端结构 / 配置 / 数据库 | 含义 | 取值/默认/是否必填 |
| --- | --- | --- | --- | --- |
| 策略名称 | `label` | `phase3admin.FlagState.Label`；来自 `flagLabels` 映射 | 页面友好名称 | 只读 |
| 策略键 | `flag_key` | `phase3admin.FlagState.FlagKey`；来自 `phase3.ManagedFeatureFlags()` | 策略唯一标识，也是更新与回滚的主键 | 只读 |
| 状态 | `status` | `phase3admin.FlagState.Status` | 页面治理状态 | 枚举：`enabled` / `disabled` / `shadow` / `canary` / `rolling_back` / `error` |
| 是否启用 | `enabled` | `phase3admin.FlagState.Enabled`；同时映射到 `config.RAG.FeatureFlags.*` | 是否打开该策略开关 | 布尔值；修改时必填 |
| 灰度百分比 | `rollout_percentage` | `phase3admin.FlagState.RolloutPercentage` | 当前页面显示的 rollout 值 | `0-100`；修改时必填 |
| 内置策略版本 | `strategy_version` | `phase3admin.FlagState.StrategyVersion`；来自 `flagVersions` 映射 | 当前 flag 默认绑定的版本标识 | 只读 |
| 风险等级 | `risk_level` | `phase3admin.FlagState.RiskLevel`；来自 `flagRiskLevels` 映射 | 用于提示操作风险 | 当前代码里有 `low` / `medium` / `high` |
| 更新时间 | `updated_at` | `phase3admin.FlagState.UpdatedAt` | 最近一次内存状态更新时间 | 只读 |
| 修改原因 | `reason` | `PATCH /strategy/flags/:flag_key` body 中的 `reason` | 记录这次调整原因，同时会写入操作日志和审计事件 | 字符串；必填 |

关于这些字段，需要特别注意：

1. `enabled` 才是最直接影响运行时 feature flag 的字段。
2. `status` 和 `rollout_percentage` 并不总是等价于真实流量控制。
3. 当前代码里，`rolling_back` 和 `error` 虽然是允许值，但没有看到自动流转到这两个状态的完整闭环逻辑，更多是“可写可显”的治理状态。
4. 高风险策略 `RAG_ENABLE_MODEL_ASSISTED_REWRITE` 从 `disabled` 直接改到 `enabled` 时，后端会拒绝；必须先走 `shadow` 或 `canary`。

### 6.3 Impact 分析区

| 页面字段 | API 字段 | 后端结构 / 数据来源 | 含义 | 取值/默认/是否必填 |
| --- | --- | --- | --- | --- |
| 样本量 | `sample_size` | `strategyImpactResponse.SampleSize`；来自 `kb_retrieve_log` 时间窗内日志数量 | 当前时间范围内纳入分析的总日志量 | 只读 |
| 样本量不足告警 | `sample_size_too_small` | `strategyImpactResponse.SampleSizeTooSmall` | 样本过少时提示不要过度解读结果 | 阈值写死在后端：比较型分析时 baseline 或 candidate 任一 `< 5` |
| `parent_fill_gain` | `parent_fill_gain` | 优先来自最近相关评测报告 `report.comparison.parent_fill_gain_delta`；缺失时回退为日志均值差 | Parent/Child 补全文本带来的增益 | 只读 |
| `rewrite_gain` | `rewrite_gain` | 优先来自评测报告；缺失时回退为日志中 rewrite gain bucket 的候选增益比例差 | Rewrite 带来的增益 | 只读 |
| `evidence_refusal_rate` | `evidence_refusal_rate` | 候选日志里 `EvidenceGateResult == refused` 的比例 | 证据拒答率 | 只读 |
| `refusal_false_positive_rate` | `refusal_false_positive_rate` | 最近相关评测报告中的比较结果 | 误拒绝率 | 只读；如果没有评测报告，前端会显示 `Contract gap` |
| `citation_support_score` | `citation_support_score` | 候选日志 `CitationSupportScore` 平均值 | 引用支撑分数 | 只读 |
| `p95_latency_delta_ms` | `p95_latency_delta_ms` | 优先来自评测报告；缺失时回退为候选/基线日志 P95 耗时差 | 相对基线的 P95 延迟变化 | 正数通常表示更慢 |
| `avg_context_tokens_delta` | `avg_context_tokens_delta` | 后端字段存在，但当前 `GetStrategyImpact` 固定把它记为 contract gap | 平均上下文 token 变化 | 当前页面基本会显示 `Contract gap` |
| `route_contribution.dense` | `route_contribution.dense` | 候选/基线日志 `DenseContribution` 的均值差 | dense 路由贡献变化 | 只读 |
| `route_contribution.sparse` | `route_contribution.sparse` | 候选/基线日志 `SparseContribution` 的均值差 | sparse 路由贡献变化 | 只读 |
| Contract gap 列表 | `contract_gaps` | 后端根据缺失数据拼装 | 哪些指标当前没有可靠依据 | 只读 |

说明：

1. 当前前端没有展示后端返回的 `baseline_sample_size`、`candidate_sample_size`、`citation_precision_delta`、`empty_rate_delta`、`error_rate_delta`。
2. 当前前端也没有传 `kb_id`，所以 Impact 默认是全局时间范围视角，不是单知识库视角。

### 6.4 Gate 摘要区

| 页面字段 | API 字段 | 后端结构 / 数据来源 | 含义 | 取值/默认/是否必填 |
| --- | --- | --- | --- | --- |
| `gate_status` | `gate_status` | `strategyGateSummaryResponse.GateStatus` | 最近相关评测 Gate 的状态 | 可能值：`passed` / `failed` / `pending` / `report_missing` / `selection_mismatch` |
| `passed` | `passed` | `strategyGateSummaryResponse.Passed` | Gate 是否通过 | 布尔值 |
| `failed_rules` | `failed_rules` | 评测报告 `Gate.Checks` 中未通过检查项的 `name` | 哪些门禁规则失败 | 字符串数组 |
| `baseline_report_id` | `baseline_report_id` | 后端拼接：`runID:baselineProfile` | 对应的基线报告标识 | 只读 |
| `candidate_report_id` | `candidate_report_id` | 后端拼接：`runID:candidateProfile` | 对应的候选报告标识 | 只读 |
| `last_eval_run_id` | `last_eval_run_id` | 最近相关成功评测的 `run_id` | 这份 Gate 结果对应哪个评测运行 | 只读 |

说明：

1. Gate 不是看策略表单是否合法，而是看最近一次相关评测是否达到质量门槛。
2. 前端当前没有展示后端返回的 `contract_gaps`。
3. 前端当前也没有传 `version`，所以这里是“按 flag 找最近相关评测”，不是“按版本找 Gate”。

### 6.5 版本列表、操作日志与回滚表单

| 页面字段 | API 字段 | 后端结构 / 数据来源 | 含义 | 取值/默认/是否必填 |
| --- | --- | --- | --- | --- |
| 版本 ID | `version_id` | `phase3admin.VersionRecord.VersionID` | 版本快照标识，格式类似 `flag_key:vN` | 只读 |
| 创建人 | `created_by` | `VersionRecord.CreatedBy` | 版本由谁产生 | 当前实现是 `system` 或 `user:{id}` |
| 版本 Gate | `gate_status` | `VersionRecord.GateStatus` | 版本记录上的 Gate 状态 | 当前代码默认写入 `pending`，未看到评测完成后回写逻辑 |
| 版本创建时间 | `created_at` | `VersionRecord.CreatedAt` | 版本快照创建时间 | 只读 |
| 操作类型 | `operation` | `phase3admin.OperationRecord.Operation` | 操作日志类型 | 当前代码里主要是 `update_flag` / `rollback` |
| 变更前状态 | `from_status` | `OperationRecord.FromStatus` | 操作前状态 | 只读 |
| 变更后状态 | `to_status` | `OperationRecord.ToStatus` | 操作后状态 | 只读 |
| 操作原因 | `reason` | `OperationRecord.Reason` | 当时提交的原因 | 只读 |
| 操作时间 | `created_at` | `OperationRecord.CreatedAt` | 操作日志时间 | 只读 |
| 回滚原因 | `reason` | `POST /strategy/rollback` body 中的 `reason` | 回滚原因 | 字符串；必填 |
| 回滚目标版本 | `target_version` | `rollbackStrategyRequest.TargetVersion` | 后端支持回滚到具体版本 | 当前前端固定传 `phase2_baseline`，页面不可编辑 |
| 回滚目标 flag 列表 | `flag_keys` | `rollbackStrategyRequest.FlagKeys` | 后端支持只回滚部分 flag | 当前前端只支持“当前选中 flag”或“全部 flag”两种模式 |

这里最容易误解的点有两个：

1. “回滚当前策略”按钮当前 **不是** 回到最近一个历史版本，而是把当前选中 flag 回滚到 `phase2_baseline`。
2. 版本列表虽然存在，但当前页面没有“从版本列表中选择某个版本回滚”的能力。

## 7. 监控功能说明

策略中心里的“监控”并不是 Prometheus 图表型大盘，而是围绕单个策略的 **治理监控视图**。当前它主要由三块组成：

### 7.1 Impact 分析

它回答的问题是：**这个策略相对基线，看起来有没有收益，有没有副作用。**

当前监控的核心指标包括：

1. 样本量是否足够。
2. Parent/Child 补全是否带来收益。
3. Rewrite 是否带来收益。
4. 证据拒答率是否升高。
5. 误拒绝率是否偏高。
6. 引用支撑分数是否足够。
7. P95 延迟是否变差。
8. dense/sparse 路由贡献是否变化。

如何解读：

1. `sample_size_too_small=true` 时，任何增益和回归都不要轻易下结论。
2. `p95_latency_delta_ms > 0` 通常表示比基线更慢。
3. `refusal_false_positive_rate` 高，说明拒答可能“过严”。
4. `route_contribution` 这里显示的是 **候选相对基线的均值差**，不是绝对占比。
5. `Contract gap` 越多，说明这块数据越不能作为上线依据。

### 7.2 Gate 摘要

它回答的问题是：**最近一次相关评测，是否满足上线门槛。**

当前 Gate 的规则来源于评测模块，默认阈值定义在 `backend/internal/milvus/evaluation/gate.go`：

1. `Recall@K delta >= 0.08`
2. `MRR delta >= 0`
3. `nDCG delta >= 0`
4. `Citation Accuracy delta >= 0`
5. `P95 latency regression ratio <= 0.20`
6. `Refusal false positive rate <= 0.05`
7. 如果候选使用 model rewrite，还会检查 `rewrite_gain_delta`
8. 如果配置了 `MaxP95LatencyRegressionMS`，还会检查 `p95_latency_regression_ms`

如何解读：

1. `passed=false` 时，优先看 `failed_rules`，再进入评测报告页定位。
2. `report_missing` 说明不是“通过了”，而是根本没有对应成功评测报告。
3. Gate 摘要只展示结论，不展示全部明细；要看细项和失败样本，需要去 `/evaluation/reports/[runId]`。

### 7.3 版本与操作日志

它回答的问题是：**最近都改过什么，是否频繁回滚。**

如何解读：

1. 频繁出现 `rollback`，通常意味着策略不稳定或上线流程不充分。
2. `created_by=system` 说明是初始化快照，不一定是人工发布。
3. 这两块目前是进程内内存，不是权威审计台账；权威持久化审计更接近 `kb_audit_event`。

## 8. 与后端、配置、数据库的关联

### 8.1 页面能力到后端实现的总览

| 页面能力 | 前端入口 | API | 后端 handler | 核心结构 / 模型 / 配置 | 持久化情况 |
| --- | --- | --- | --- | --- | --- |
| 拉取策略列表 | `loadFlags()` | `GET /strategy/flags` | `ListStrategyFlags` | `phase3admin.ListFlags` + `phase3admin.FlagState` + `config.Global.RAG.FeatureFlags` | 运行时配置 + 进程内状态 |
| 修改策略 | `submitEdit()` | `PATCH /strategy/flags/:flag_key` | `UpdateStrategyFlag` | `phase3admin.UpdateFlag`，并修改 `config.Global.RAG.FeatureFlags`；高风险时联动 `config.Global.RAG.Phase3.ModelRewriteShadowRatio` | 运行时配置；另写 `kb_audit_event` |
| 拉取版本列表 | `loadDetail()` | `GET /strategy/versions` | `ListStrategyVersions` | `phase3admin.ListVersions` + `VersionRecord` | 仅进程内内存 |
| 回滚 | `submitRollback()` | `POST /strategy/rollback` | `RollbackStrategy` | `phase3admin.Rollback`，按 `phase3.RollbackOrder()` 执行 | 运行时配置；另写 `kb_audit_event` |
| Impact 分析 | `loadDetail()` | `GET /strategy/impact` | `GetStrategyImpact` | `strategyImpactResponse` + `KBRetrieveLogDao.ListByCreatedAt` + 最近相关 `KBEvalRun` 报告 | `kb_retrieve_log` + `kb_eval_run` + 报告 JSON |
| Gate 摘要 | `loadDetail()` | `GET /strategy/gates` | `GetStrategyGates` | `strategyGateSummaryResponse` + 最近相关 `KBEvalRun` 报告 | `kb_eval_run` + 报告 JSON |
| 操作日志 | `loadFlags()` / `loadDetail()` | `GET /strategy/operations` | `ListStrategyOperations` | `phase3admin.ListOperations` + `OperationRecord` | 仅进程内内存 |
| 查看评测运行 | 顶部按钮跳转 | `GET /eval/runs` 等 | `ListEvalRuns` / `GetEvalRun` / `GetEvalReport` | `KBEvalRun`、评测报告 | `kb_eval_run` + 报告 JSON |

### 8.2 前端字段到后端字段的重点映射

| 前端字段 | API 字段 | 后端结构体/数据库字段 | 说明 |
| --- | --- | --- | --- |
| `flag_key` | `flag_key` | `phase3admin.FlagState.FlagKey` | 策略唯一标识 |
| `enabled` | `enabled` | `FlagState.Enabled` -> `config.RAG.FeatureFlags.*` | 真正影响 flag 开关的核心字段 |
| `status` | `status` | `FlagState.Status` | 治理状态；不是所有状态都有运行时语义 |
| `rollout_percentage` | `rollout_percentage` | `FlagState.RolloutPercentage` | 当前主要对 `ModelRewriteShadowRatio` 有真实副作用 |
| `strategy_version` | `strategy_version` | `FlagState.StrategyVersion` | 当前 flag 绑定的版本标识 |
| `risk_level` | `risk_level` | `FlagState.RiskLevel` | 风险提示 |
| `updated_at` | `updated_at` | `FlagState.UpdatedAt` | 进程内状态更新时间 |
| `version_id` | `version_id` | `phase3admin.VersionRecord.VersionID` | 版本快照 ID |
| `created_by` | `created_by` | `VersionRecord.CreatedBy` | `system` 或 `user:{id}` |
| `operation` | `operation` | `phase3admin.OperationRecord.Operation` | `update_flag` 或 `rollback` |
| `from_status` | `from_status` | `OperationRecord.FromStatus` | 操作前状态 |
| `to_status` | `to_status` | `OperationRecord.ToStatus` | 操作后状态 |
| `sample_size` | `sample_size` | `strategyImpactResponse.SampleSize` | 时间窗口内纳入分析的总日志量 |
| `citation_support_score` | `citation_support_score` | `kb_retrieve_log.citation_support_score` 聚合 | 候选日志平均引用支撑分 |
| `evidence_refusal_rate` | `evidence_refusal_rate` | `kb_retrieve_log.evidence_gate_result` 聚合 | 候选日志拒答率 |
| `p95_latency_delta_ms` | `p95_latency_delta_ms` | 评测报告比较结果，或 `kb_retrieve_log.duration_ms` 聚合 | 候选相对基线的 P95 延迟变化 |
| `gate_status` | `gate_status` | `strategyGateSummaryResponse.GateStatus` | 来自最近相关评测的结论，不是表单校验状态 |
| `last_eval_run_id` | `last_eval_run_id` | `kb_eval_run.run_id` | Gate 对应的评测运行 |

### 8.3 哪些数据不在数据库里

当前代码里，下面这些并 **不是数据库表**：

1. 策略版本列表 `VersionRecord`
2. 策略操作日志 `OperationRecord`
3. 策略页面当前状态缓存 `flagStates`

它们都保存在 `backend/internal/rag/phase3admin/state.go` 的包级内存中。

真正落库的，主要是：

1. 检索日志：`kb_retrieve_log`
2. 评测运行：`kb_eval_run`
3. 审计事件：`kb_audit_event`

## 9. 使用流程

这里给一个从“准备上线某个策略”到“观察与回滚”的完整建议流程。

### 9.1 准备评测数据

1. 进入 `/evaluation/datasets`。
2. 创建评测数据集，填写：
   - `name`
   - `description`
   - 可选 `kb_id`
3. 往数据集中新增或导入评测样本，关键字段至少包括：
   - `case_key`
   - `query`
   - `top_k`
   - `relevant_ids`
4. 点击“校验”，确保数据集状态变成 `ready`。

### 9.2 创建评测运行

1. 进入 `/evaluation/runs`，或在策略中心右上角点“查看评测运行”。
2. 创建评测运行，选择：
   - `dataset_id`
   - `baseline_profile`
   - `candidate_profile`
   - `profiles`
   - 可选 `gate_thresholds`
3. 等待运行完成，状态变成 `succeeded`。
4. 打开评测报告页，看对比结果和失败样本。

### 9.3 在策略中心调整开关

1. 进入 `/strategy-center`。
2. 在左侧列表选择目标策略。
3. 先看当前 `risk_level`、`status`、`enabled`、`rollout_percentage`。
4. 点击“修改策略”。
5. 对高风险策略 `RAG_ENABLE_MODEL_ASSISTED_REWRITE`：
   - 先走 `shadow` 或 `canary`
   - 不要直接从 `disabled` 切到 `enabled`
6. 填写必填 `reason` 后提交。

### 9.4 观察效果

1. 切换 `1h / 24h / 7d` 看不同窗口下的 Impact。
2. 看 `sample_size_too_small` 是否告警。
3. 看 `p95_latency_delta_ms` 是否明显变差。
4. 看 `refusal_false_positive_rate` 是否异常升高。
5. 看 `gate_status` / `failed_rules` 是否通过。
6. 需要更细节时，回到评测报告页或检索日志页继续排查。

### 9.5 出现问题时回滚

1. 如果只想把当前 flag 退回 Phase 2 基线，点“回滚当前策略”。
2. 如果想把全部 Phase 3 策略整体退回基线，点“回滚到 Phase2 Baseline”。
3. 填写必填 `reason`。
4. 回滚后重新刷新策略中心，确认状态、Impact 和日志。

## 10. 开发维护说明

### 10.1 新增一个策略字段或新策略 flag

至少要改这些地方：

1. `backend/internal/rag/phase3/contract.go`
   - 新增 `FlagXXX`
   - 加入 `managedFeatureFlags`
   - 视需要加入 `rollbackOrder`
2. `backend/internal/config/config.go`
   - 在 `RAGFeatureFlags` 增加对应布尔字段
   - 补 `GetPhase3StrategyFlag` / `SetPhase3StrategyFlag`
   - 补环境变量覆盖
   - 如果有额外参数，也要补 `RAGPhase3Config`、默认值和校验
3. `backend/internal/rag/phase3admin/state.go`
   - 补 `flagLabels`
   - 补 `flagRiskLevels`
   - 补 `flagVersions`
   - 如果有特殊 rollout 副作用，补 `applyRolloutSideEffects`
4. `backend/api/handler/kb/handler_strategy_insights.go`
   - 补 `strategyProfileFlagEnabled`
   - 补 `strategyCandidateLog`
   - 否则 Impact/Gate 识别不到这个新策略
5. `backend/internal/milvus/evaluation/types.go`
   - 在 `StrategyProfile` 中补新策略开关或参数
6. `backend/internal/milvus/evaluation/profiles.go`
   - 如需默认评测 profile，补默认组合
7. `admin/src/types/kb.ts`
   - 如果接口结构变化，补 TS 类型
8. `admin/src/components/admin/strategy-center-page.tsx`
   - 如需新增页面字段、操作或说明，补前端展示
9. 测试
   - `backend/api/handler/kb/handler_strategy_test.go`
   - `backend/api/handler/kb/handler_strategy_insights_test.go`
   - `backend/internal/rag/phase3admin/state_test.go`
   - `admin/src/__tests__/strategy-center-page.test.tsx`

### 10.2 调整现有策略逻辑

策略中心本身只负责治理，不负责检索算法本体。真正改策略逻辑，通常还要进这些文件：

1. Parent/Child：
   - `backend/internal/milvus/retrieval/parent_child.go`
   - `backend/internal/milvus/splitter/parent_child.go`
2. Strategic TopK：
   - `backend/internal/milvus/retrieval/topk_policy.go`
3. Evidence Refusal：
   - `backend/internal/milvus/retrieval/evidence_gate.go`
4. Citation Consistency：
   - `backend/internal/milvus/retrieval/citation_consistency.go`
5. Domain Terms / Route Specific Rewrite / Model Assisted Rewrite：
   - `backend/internal/milvus/retrieval/rewrite.go`
   - `backend/internal/milvus/retrieval/rewrite_sources.go`

如果改了这些逻辑，还要同步检查：

1. 评测 profile 是否要调整。
2. Gate 阈值是否还合理。
3. Impact 指标解释是否要更新。

### 10.3 新增一个监控指标

通常需要联动修改：

1. 指标采集层
   - `backend/internal/model/kb_retrieve_log.go`
   - 如果日志里还没有该字段，需要先加模型和写入逻辑
2. 检索链路埋点
   - `backend/api/handler/kb/handler.go`
   - 或对应 retrieval 实现文件
3. 聚合与接口返回
   - `backend/api/handler/kb/handler_strategy_insights.go`
4. Prometheus 指标
   - `backend/internal/observability/metrics/rag_metrics.go`
5. 前端类型与展示
   - `admin/src/types/kb.ts`
   - `admin/src/components/admin/strategy-center-page.tsx`
6. 测试
   - 前后端相关单测都要补

### 10.4 如果要把版本/操作日志改成可持久化

当前实现只在内存里保存版本和操作记录。如果业务需要可靠审计或跨重启保留历史，后续要补：

1. 新的 model 与数据库表。
2. 迁移脚本。
3. `phase3admin` 从“包级内存”改为“DB + cache”。
4. 前端可能要加分页、筛选、按版本回滚等能力。

## 11. 注意事项与常见问题

### 11.1 页面数据为空

可能原因：

1. 当前没有管理员权限，请求被后端拦截。
2. `strategy/flags` 接口失败。
3. 当前运行态没有成功初始化 `config.Global`。

### 11.2 Impact 大量出现 `Contract gap`

可能原因：

1. 当前时间窗里没有足够检索日志。
2. 没有最近相关的成功评测报告。
3. 后端字段存在但当前没有计算逻辑。
4. 例如 `avg_context_tokens_delta`，当前代码就没有真正填值，页面基本只会看到 `Contract gap`。

### 11.3 策略改了但好像没有生效

重点先排查这几件事：

1. 你改的是 `enabled`，还是只改了 `status` / `rollout_percentage`。
2. 对大多数 flag 来说，`status` / `rollout_percentage` 目前只是治理元数据，不一定改变真实流量。
3. 后端 `reconfigureManager` 是否失败。
4. 环境变量是否覆盖了配置。

### 11.4 为什么“回滚当前策略”不是回到上一版

因为当前前端提交回滚请求时，固定把 `target_version` 写成了 `phase2_baseline`。  
也就是：

1. “回滚当前策略” = 只回滚当前 flag，但目标仍是 `phase2_baseline`
2. “回滚到 Phase2 Baseline” = 把全部 Phase 3 flag 都退回 `phase2_baseline`

如果要支持“从版本列表选择某个历史版本回滚”，需要前端补 UI。

### 11.5 为什么版本列表里的 Gate 经常是 `pending`

因为当前代码里创建 `VersionRecord` 时默认写 `gate_status="pending"`，但没有看到评测完成后把真实 Gate 结果回写到版本记录的逻辑。

所以：

1. 版本列表里的 `gate_status` 目前不一定能代表真实最新 Gate 结论。
2. 真正可参考的是右侧 `Gate 摘要`，它来自最近相关评测报告。

### 11.6 为什么评测运行页面的时间筛选可能没有效果

前端 `evaluation-runs-page.tsx` 有时间范围筛选，并会传 `start_time` / `end_time`。  
但当前后端 `ListEvalRuns` 只处理了 `dataset_id` 和 `status`，没有看到处理 `start_time` / `end_time` 的代码。

这属于当前仓库里已经存在的前后端不一致点。

### 11.7 为什么监控没有数据

Impact 和 Gate 依赖两类数据源：

1. `kb_retrieve_log`
2. `kb_eval_run` + 评测报告 JSON

其中任意一类缺失，策略中心都会变成“有页面、没证据”。

### 11.8 为什么服务重启后版本和操作记录可能变少甚至重置

因为 `VersionRecord` 和 `OperationRecord` 当前都保存在 `phase3admin/state.go` 的进程内内存中，不是数据库表。

### 11.9 `updated_at` 是否一定可信

不一定完全等于“真实历史最后发布时间”。

原因是：

1. `phase3admin.ensureStateLocked` 在初始化时会补一遍内存状态。
2. 如果原有内存状态不存在，`updated_at` 会取初始化时的当前时间。

所以它更像“当前内存态最近刷新/更新时刻”，不是强审计字段。

## 12. 结论

当前策略中心已经具备一个可用的治理闭环：

1. 配置开关
2. 观察影响
3. 看评测 Gate
4. 记录操作
5. 执行回滚

但它也有几个明确的工程边界：

1. 版本和操作日志还不是持久化台账。
2. 多数 flag 的 `status` / `rollout_percentage` 还不是严格的流量发布器。
3. Gate 与 Impact 当前是“按 flag 找最近相关评测”的模式，不是“按具体版本全链路追踪”的模式。

如果后续要把这个模块做成更完整的发布治理台，优先建议补三件事：

1. 版本/操作日志持久化。
2. 支持按历史版本回滚。
3. 把页面上展示的 rollout/status 语义和真实运行时控制语义彻底对齐。
