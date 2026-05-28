# Phase 4 详细功能实现路线（企业治理与规模化）

## 1. 文档定位

本文档是 Phase 4 的执行手册，目标是把“企业治理与规模化期”拆成可直接实施的细颗粒任务路线。
它有两个用途：

1. 作为团队推进 Phase 4 的统一执行文档。
2. 作为后续长期运营、策略自动化与规模化接入的治理基线。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `企业治理闭环` 固定指：策略变更、评测门禁、灰度发布、监控告警、回滚演练、审计留痕、复盘报告全部可追踪。
2. `AB 实验与灰度发布机制` 固定指：同一策略可按实验组、用户范围、知识库范围、流量比例逐步放量，并支持一键回滚。
3. `索引生命周期管理` 固定指：Collection/索引的版本化、重建、迁移、切换、回滚、健康巡检与操作留痕。
4. `成本治理` 固定指：embedding、检索、rerank、LLM、向量存储、上下文 token 的成本采集、归因、看板与预算告警。
5. `合规审计` 固定指：上传、解析、入库、检索、改写、生成、删除、索引切换、权限操作全链路留痕，并具备脱敏、保留期与查询能力。
6. `自动化周报` 固定指：按固定周期输出质量、稳定性、成本、索引健康、实验进展、风险清单与下周建议。
7. `Milvus/向量库运维工具化` 固定指：Collection 列表、active Collection 标记、健康检查、容量巡检、重建、切换、回滚与操作审计。
8. `Phase 4 回归基线` 固定指：以 Phase 3 验收报告为起点，持续对比 Recall@K、MRR、nDCG、Citation Precision、Citation Support Score、Evidence Refusal Rate、Rewrite Gain、P95 延迟、成本与稳定性指标。

---

## 2. Phase 4 范围边界

## 2.1 本阶段必须完成

1. AB 实验与灰度发布机制平台化，支持检索策略、rewrite、TopK、拒答、索引参数等策略受控实验。
2. 索引生命周期管理上线，支持 Collection/索引版本化、重建、切换、回滚与操作留痕。
3. 成本看板上线，覆盖 embedding、检索、rerank、LLM、向量存储与上下文 token。
4. 合规审计体系上线，覆盖操作链路与查询链路，并支持审计查询、脱敏展示与保留期策略。
5. 自动化周报上线，周期性输出质量、稳定性、成本、索引健康、实验结果与风险项。
6. Milvus/向量库运维工具化上线，支持 Collection 列表、active 标记、健康检查、容量巡检、重建、切换与回滚。
7. 建立 Phase 4 规模化门禁，任何策略、索引或成本治理变更都必须可灰度、可观测、可回滚。
8. 完成 Phase 4 验收报告与治理运行手册，支撑长期运营。

## 2.2 本阶段明确不做

1. 不做无门禁的全自动策略学习或在线强化学习。
2. 不做无人工审批的索引自动切换；自动化可以生成建议，但切换必须有审批或明确策略门禁。
3. 不做跨区域多活的完整工程落地；本阶段可输出容灾预案与演练清单，但不把多活作为必须交付。
4. 不把成本优化做成单纯降配；任何成本下降都必须同时观察质量、延迟与拒答误伤。
5. 不把审计日志直接暴露原始敏感内容；审计查询必须支持权限控制、脱敏与最小必要展示。
6. 不允许 Collection 重建、迁移、切换绕过灰度与回滚演练。

---

## 3. 目标与通过标准（Gate）

Phase 4 通过标准（全满足）：

1. 任意检索策略变更可按 `5% -> 20% -> 50% -> 100%` 或同等灰度节奏发布，并可在 10 分钟内回滚到上一稳定策略。
2. AB 实验能输出 baseline/candidate 的质量、延迟、成本与风险对比，且实验结果可按 `kb_id/query_type/route/strategy_version` 维度追踪。
3. Collection/索引具备版本化管理能力，重建、切换、回滚全流程有操作留痕，并能证明 active Collection 与查询 Collection 一致。
4. 成本看板能展示每千次问答成本、embedding 成本、LLM token 成本、检索/rerank 成本、向量存储成本与成本异常告警。
5. 审计日志覆盖上传、检索、生成、删除、策略变更、索引切换与权限操作，且支持脱敏查询、保留期策略与审计导出。
6. 自动化周报能稳定生成，并包含质量趋势、稳定性趋势、成本趋势、实验结果、索引健康、告警复盘与下周动作。
7. Milvus/向量库运维工具可完成健康巡检、容量巡检、Collection 列表、active 标记、重建计划、切换计划与回滚演练记录。
8. Phase 4 任何治理能力异常时不能拖垮主查询链路，必须降级为“记录告警 + 保持服务可用”。

---

## 4. 实现路线总览（L0 -> L8）

Phase 4 按 9 条路线推进，按门禁顺序合流：

1. L0：Phase 3 基线冻结、治理开关与指标口径统一
2. L1：AB 实验与灰度发布机制平台化
3. L2：索引生命周期管理（重建/迁移/版本化/回滚）
4. L3：成本采集、归因与成本看板
5. L4：合规审计与数据保留策略
6. L5：自动化周报与运营复盘机制
7. L6：Milvus/向量库运维工具化
8. L7：规模化监控告警、容量巡检与治理门禁
9. L8：灰度发布、回滚演练与 Phase 4 验收收口

建议顺序：`L0 -> L1 + L2 -> L3 + L4 -> L5 + L6 + L7 -> L8`

---

## 5. 详细路线拆解

## 5.1 L0 Phase 3 基线冻结、治理开关与指标口径统一

### 目标

在进入企业治理前冻结 Phase 3 稳定基线，确保后续治理、成本、实验与索引变更都有可对比、可回滚的起点。

### 功能任务

1. 固定 Phase 3 基线快照：
   - 检索策略版本
   - route 权重
   - parent-child 配置
   - strategic TopK 配置
   - evidence gate 阈值
   - citation consistency 版本
   - rewrite 策略版本
   - Recall@K、MRR、nDCG、Citation Precision、Citation Support Score、P95 延迟、平均上下文 token
2. 新增治理类 Feature Flag：
   - `RAG_ENABLE_EXPERIMENT_PLATFORM`
   - `RAG_ENABLE_INDEX_LIFECYCLE`
   - `RAG_ENABLE_COST_DASHBOARD`
   - `RAG_ENABLE_COMPLIANCE_AUDIT`
   - `RAG_ENABLE_WEEKLY_REPORT`
   - `RAG_ENABLE_MILVUS_OPS_TOOLING`
   - `RAG_ENABLE_COLLECTION_SWITCH_GUARD`
3. 统一 Phase 4 指标口径：
   - `quality_score`
   - `cost_per_1k_queries`
   - `avg_context_tokens`
   - `strategy_regression_rate`
   - `rollback_success_rate`
   - `audit_coverage_rate`
   - `collection_health_score`
4. 建立治理数据最小结构：
   - `experiment_id`
   - `strategy_version`
   - `index_version`
   - `collection_version`
   - `cost_trace_id`
   - `audit_trace_id`
   - `release_id`
5. 明确降级规则：
   - 治理采集失败不阻断检索主链路
   - 审计写入失败必须告警并进入补偿队列
   - 成本统计失败只影响看板，不影响回答
   - Collection 切换守卫失败必须阻断切换

### 验收

1. Phase 3 基线可复跑、可比较、可回滚。
2. Phase 4 治理开关可独立控制，关闭后不影响 Phase 3 主链路。
3. 指标口径、实验维度、索引版本字段冻结，并在日志或数据表中可追踪。
4. 治理链路异常时有明确降级策略，不拖垮查询主链路。

---

## 5.2 L1 AB 实验与灰度发布机制平台化

### 目标

把 Phase 2/Phase 3 的临时灰度与对比实验升级为平台化能力，让策略变更可实验、可归因、可回滚。

### 功能任务

1. 新增实验配置模型：
   - `experiment_id`
   - `experiment_name`
   - `strategy_type`
   - `baseline_version`
   - `candidate_version`
   - `traffic_ratio`
   - `target_kb_ids`
   - `target_query_types`
   - `start_time/end_time`
   - `owner`
2. 支持实验分流规则：
   - 按用户哈希分流
   - 按 `kb_id` 分流
   - 按 `query_type` 分流
   - 按内部/外部环境分流
   - 支持 shadow 模式只记录不影响用户结果
3. 支持实验类型：
   - 检索策略实验
   - rewrite 策略实验
   - dynamic TopK 实验
   - evidence gate 阈值实验
   - rerank 版本实验
   - 索引参数实验
4. 实验结果采集：
   - Recall@K、MRR、nDCG
   - Citation Precision、Citation Support Score
   - Evidence Refusal Rate、Refusal False Positive Rate
   - Rewrite Gain、Route Contribution
   - P50/P95 延迟
   - 平均 token 与成本
   - 用户反馈或人工标注结果
5. 灰度发布规则：
   - `internal -> 5% -> 20% -> 50% -> 100%`
   - 每一档必须通过质量、延迟、成本、错误率门禁
   - 门禁失败自动建议暂停或回滚
6. 回滚能力：
   - 一键关闭 candidate
   - 回退 baseline version
   - 记录 `rollback_reason/rollback_operator/rollback_time`

### 验收

1. 至少支持一种检索策略和一种 rewrite 策略的 AB 实验。
2. 实验结果能按 baseline/candidate 输出质量、延迟、成本与风险对比。
3. 灰度比例可控，实验分组稳定，同一用户不会在短时间内频繁切组。
4. candidate 异常时可在 10 分钟内回滚到 baseline。

---

## 5.3 L2 索引生命周期管理（重建/迁移/版本化/回滚）

### 目标

让 Collection 与索引从“手工配置资产”升级为“可版本化、可巡检、可切换、可回滚”的企业级基础设施。

### 功能任务

1. 建立索引版本模型：
   - `index_version`
   - `collection_name`
   - `collection_role`
   - `embedding_model`
   - `embedding_dimension`
   - `metric_type`
   - `index_type`
   - `index_params`
   - `build_status`
   - `build_started_at/build_finished_at`
   - `created_by`
2. 定义 Collection 角色：
   - `active`
   - `candidate`
   - `standby`
   - `rollback`
   - `deprecated`
3. 支持索引重建流程：
   - 生成重建计划
   - 校验数据源与文档数量
   - 异步构建 candidate Collection
   - 运行离线评测与健康检查
   - 灰度切换 candidate
   - 保留 rollback Collection
4. 支持迁移与版本切换：
   - Collection 切换必须校验 `database/collection/dimension/metric_type/schema`
   - active 切换必须写入操作审计
   - 切换前必须确认最近一次回滚演练有效
5. 支持健康检查：
   - Collection 存在性
   - schema 一致性
   - 向量维度一致性
   - 索引加载状态
   - chunk 数与业务文档数对账
   - 查询 smoke test
6. 支持回滚：
   - active 指针回退
   - 查询配置回退
   - 实验配置回退
   - 回滚原因与影响范围留痕

### 验收

1. 能创建 candidate Collection 并完成一次完整重建计划。
2. active/candidate/rollback 状态清晰可查，且切换有审计记录。
3. Collection 切换前健康检查能发现 schema、dimension、collection 配置不一致问题。
4. 回滚演练可在 10 分钟内恢复上一稳定 Collection。

---

## 5.4 L3 成本采集、归因与成本看板

### 目标

让 RAG 成本从“模型账单后验统计”升级为“请求级归因、策略级对比、预算级告警”的治理能力。

### 功能任务

1. 建立成本采集字段：
   - `request_id`
   - `kb_id`
   - `user_id`
   - `experiment_id`
   - `strategy_version`
   - `embedding_tokens`
   - `context_tokens`
   - `completion_tokens`
   - `retrieval_candidate_count`
   - `rerank_candidate_count`
   - `llm_model`
   - `cost_estimate`
2. 成本拆分维度：
   - embedding 成本
   - dense 检索成本
   - sparse/BM25 检索成本
   - rerank 成本
   - LLM 输入/输出 token 成本
   - 向量存储成本
   - 索引重建成本
3. 成本看板指标：
   - 每千次问答成本
   - 每个 `kb_id` 成本
   - 每个策略版本成本
   - 每个实验组成本
   - 平均上下文 token
   - token 预算命中率
   - 高成本 query TopN
4. 成本告警规则：
   - `cost_per_1k_queries` 超预算
   - 平均上下文 token 异常升高
   - rerank 候选数异常升高
   - LLM 输出 token 异常升高
   - 索引重建成本超阈值
5. 成本优化建议：
   - 调整 TopK 与 token budget
   - 缩小 rewrite expansion
   - 缩小 parent fill window
   - 对热 query 增加缓存
   - 对低收益策略停止灰度

### 验收

1. 能按请求、知识库、策略版本、实验组归因成本。
2. 看板能展示质量、延迟、成本三者联动，不只展示单一账单。
3. 成本异常能触发告警并定位到策略、知识库或 query 类型。
4. 至少产出一版成本优化报告，且不以牺牲核心质量指标为代价。

---

## 5.5 L4 合规审计与数据保留策略

### 目标

让 RAG 全链路操作与查询行为可审计、可追溯、可脱敏查询，并满足企业内部合规与安全排查需要。

### 功能任务

1. 审计事件分类：
   - `DocumentUploaded`
   - `DocumentParsed`
   - `DocumentIngested`
   - `DocumentDeleted`
   - `KnowledgeRetrieved`
   - `AnswerGenerated`
   - `StrategyChanged`
   - `ExperimentChanged`
   - `CollectionRebuilt`
   - `CollectionSwitched`
   - `PermissionChanged`
2. 审计字段标准化：
   - `audit_trace_id`
   - `request_id`
   - `operator_id`
   - `user_id`
   - `kb_id`
   - `document_id`
   - `action`
   - `resource_type`
   - `resource_id`
   - `before/after`
   - `result`
   - `reason`
   - `created_at`
3. 查询链路留痕：
   - original query
   - rewritten query
   - final query
   - route contribution
   - selected citations
   - refusal reason
   - answer metadata
   - strategy/index/experiment version
4. 安全与脱敏：
   - query 与 answer 支持敏感字段脱敏
   - 审计查询按角色授权
   - 原文片段按最小必要原则展示
   - 导出任务必须记录审批人与用途
5. 数据保留策略：
   - 审计日志保留期
   - 原始文件保留期
   - 检索日志保留期
   - 实验数据保留期
   - 过期清理任务与清理审计
6. 补偿机制：
   - 审计写入失败进入补偿队列
   - 补偿失败触发告警
   - 审计缺口按 `audit_coverage_rate` 统计

### 验收

1. 上传、检索、生成、删除、策略变更、Collection 切换均有审计事件。
2. 审计查询可按 `audit_trace_id/request_id/kb_id/document_id/operator_id` 定位。
3. 审计展示具备脱敏与权限控制，不泄露非必要原文。
4. 审计写入失败不会影响主链路，但必须可补偿、可告警、可统计缺口。

---

## 5.6 L5 自动化周报与运营复盘机制

### 目标

把质量、稳定性、成本、实验与索引健康沉淀为固定节奏的运营报告，减少“出了问题才翻日志”的被动排查。

### 功能任务

1. 周报数据源：
   - 离线评测结果
   - 线上质量指标
   - 检索与生成延迟
   - 成本看板
   - 实验平台
   - 审计缺口
   - Collection 健康巡检
   - 告警与事故记录
2. 周报核心章节：
   - 本周质量趋势
   - 本周稳定性趋势
   - 本周成本趋势
   - 本周实验结果
   - 本周索引与 Collection 健康
   - 本周告警复盘
   - 风险清单与 owner
   - 下周建议动作
3. 自动生成规则：
   - 固定周期生成
   - 支持手动补跑
   - 支持按环境生成
   - 支持按 `kb_id` 生成局部报告
4. 报告输出格式：
   - Markdown 文档
   - 管理后台页面
   - 可选通知消息
5. 周报质量门禁：
   - 缺少关键指标时标红
   - 指标异常必须附带候选原因
   - 未关闭风险必须保留到下期

### 验收

1. 能自动生成至少一版完整周报。
2. 周报能同时覆盖质量、稳定性、成本、实验、索引健康与风险清单。
3. 周报中的异常项可追溯到实验、策略版本、Collection 版本或具体告警。
4. 风险项必须有 owner、截止时间与处理状态。

---

## 5.7 L6 Milvus/向量库运维工具化

### 目标

把 Milvus/向量库运维从临时命令升级为可审计、可复用、可演练的工具链。

### 功能任务

1. 工具入口能力：
   - Collection 列表
   - active Collection 标记
   - Collection schema 查看
   - Collection row count/chunk count 查看
   - index 信息查看
   - load 状态查看
2. 健康巡检：
   - 连接状态
   - database/collection 存在性
   - schema 一致性
   - dimension 一致性
   - active/candidate/rollback 状态一致性
   - 查询 smoke test
3. 容量巡检：
   - chunk 数趋势
   - 向量存储增长
   - 索引大小趋势
   - 热知识库 TopN
   - 冷知识库 TopN
4. 运维操作：
   - 创建 candidate Collection
   - 触发重建任务
   - 执行健康检查
   - 执行切换预检
   - 执行 active 切换
   - 执行回滚
5. 操作保护：
   - 高风险操作二次确认
   - 只允许通过受控入口切换 active
   - 所有操作写入审计
   - 失败时输出明确恢复建议
6. 与现有工具衔接：
   - 复用或扩展 `backend/internal/milvus/cmd/milvusctl`
   - 复用 `MilvusManager.HealthCheck`
   - 复用检索评测与 benchmark 能力输出巡检报告

### 验收

1. 能通过工具查看 Collection 列表、active 标记、schema、index 与 load 状态。
2. 健康巡检能发现配置不一致、schema 不一致、dimension 不一致与查询不可用问题。
3. 重建、切换、回滚操作均有审计留痕。
4. 运维工具异常不会影响线上查询链路。

---

## 5.8 L7 规模化监控告警、容量巡检与治理门禁

### 目标

让企业规模化接入后的质量退化、成本异常、容量风险和索引风险能够提前发现、快速定位、受控处理。

### 功能任务

1. 质量告警：
   - Recall@10 低于基线
   - Citation Precision 下降
   - Citation Support Score 下降
   - Evidence Refusal Rate 异常升高
   - Refusal False Positive Rate 超阈值
   - Empty-After-Filter Rate 异常升高
2. 稳定性告警：
   - 检索 P95 延迟异常
   - rerank 耗时异常
   - LLM 调用错误率升高
   - 入库失败率升高
   - 审计补偿队列堆积
3. 成本告警：
   - 每千次问答成本超阈值
   - 平均上下文 token 异常
   - 高成本 query 激增
   - 索引重建成本超预算
4. 容量告警：
   - Collection 存储增长过快
   - chunk 数超过阈值
   - 热知识库查询量激增
   - candidate Collection 构建滞后
5. 治理门禁：
   - 策略发布门禁
   - 索引切换门禁
   - 成本预算门禁
   - 审计覆盖率门禁
   - 周报风险关闭门禁
6. 问题清单沉淀：
   - `KNOWN_ISSUES`
   - 固定回归用例
   - 事故复盘记录
   - 运行手册更新

### 验收

1. 质量、稳定性、成本、容量、审计五类告警均有明确阈值与处理建议。
2. 策略发布和 Collection 切换必须通过治理门禁。
3. 告警可定位到 `experiment_id/strategy_version/index_version/collection_version/kb_id`。
4. 已知问题能沉淀为回归用例或运行手册条目。

---

## 5.9 L8 灰度发布、回滚演练与 Phase 4 验收收口

### 目标

确保 Phase 4 治理能力安全上线，并能证明系统进入长期运营状态后仍可持续优化、可审计、可控成本、可回滚。

### 功能任务

1. 灰度顺序：
   - 内部环境全量启用治理采集
   - 单知识库启用实验平台
   - 单知识库启用成本看板与周报
   - 单 candidate Collection 执行重建与切换演练
   - 小流量真实用户启用 AB 实验
   - 全量启用审计与自动化周报
2. 回滚演练：
   - 回滚策略实验 candidate
   - 回滚 Collection active 指针
   - 关闭成本看板采集
   - 关闭自动化周报
   - 审计补偿队列恢复
   - 回退到 Phase 3 稳定链路
3. 验收报告：
   - Phase 3 baseline vs Phase 4 governance
   - 实验平台结果
   - 索引生命周期演练结果
   - 成本看板结果
   - 审计覆盖率结果
   - 周报样例
   - 告警与门禁演练结果
   - 风险与遗留问题
4. 运行手册：
   - 策略发布 SOP
   - Collection 切换 SOP
   - 成本异常处理 SOP
   - 审计查询 SOP
   - 周报复盘 SOP
   - 回滚演练 SOP

### 验收

1. Phase 4 治理能力灰度期间不影响 Phase 3 查询质量与稳定性。
2. 策略、索引、成本、审计、周报任一治理模块异常时均可独立关闭或降级。
3. 至少完成一次策略回滚与一次 Collection 回滚演练。
4. Phase 4 验收报告与运行手册可支撑长期运营。

---

## 6. 推荐实施节奏（无固定时长）

## 6.1 阶段推进建议

1. 先完成 `L0`，冻结 Phase 3 基线、治理开关与指标口径。
2. 再完成 `L1 + L2`，打通“策略实验”和“索引生命周期”两条最高风险治理主线。
3. 然后完成 `L3 + L4`，补齐成本治理与合规审计。
4. 再完成 `L5 + L6 + L7`，形成周报、运维工具、监控告警与门禁闭环。
5. 最后执行 `L8`，完成灰度、回滚演练、验收报告与运行手册。

## 6.2 并行与合流规则

1. 可并行：`L1` 的实验配置与 `L3` 的成本采集字段设计，`L2` 的索引版本模型与 `L6` 的 Milvus 工具命令设计，`L4` 的审计事件规范与 `L5` 的周报模板设计。
2. 必须串行：`L2` 的 active 切换依赖 `L0` 指标口径与 `L7` 门禁规则，`L8` 依赖 `L1~L7` 全部通过。
3. 统一合流：全部治理能力通过 `L8` 验收后，再进入长期运营与策略自动化阶段。

---

## 7. 角色分工（建议）

1. 架构负责人：L0 + L8（基线冻结、治理门禁、验收报告、运行手册）。
2. 后端A：L1（AB 实验、灰度发布、策略版本与回滚）。
3. 后端B：L2 + L6（索引生命周期、Collection 治理、Milvus 运维工具）。
4. 后端C：L3 + L4（成本采集、成本看板、合规审计、数据保留）。
5. QA/SRE：L5 + L7（自动化周报、监控告警、容量巡检、门禁演练）。
6. 算法/检索：参与 L1/L2/L7（实验指标、索引评测、质量门禁阈值）。

补充协作约束：

1. 架构、后端、算法先冻结“实验维度、策略版本、指标口径”，再开发实验平台。
2. 后端与 SRE 先冻结“Collection 切换门禁、回滚 SOP、健康巡检项”，再允许重建与切换。
3. 后端与安全/合规先冻结“审计事件、脱敏规则、保留期策略”，再开放审计查询。
4. 灰度前必须完成一次“策略回滚 + Collection 回滚 + 审计补偿”的联合演练。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0~L8）。
2. Phase 3 baseline vs Phase 4 governance 指标对比。
3. AB 实验平台验收：
   - experiment count
   - baseline/candidate 对比
   - 灰度比例
   - 回滚演练结果
4. 索引生命周期验收：
   - active collection
   - candidate collection
   - rollback collection
   - index version
   - health check result
   - switch/rollback drill result
5. 成本治理验收：
   - cost_per_1k_queries
   - embedding cost
   - retrieval/rerank cost
   - LLM token cost
   - storage/index rebuild cost
   - 高成本 query TopN
6. 合规审计验收：
   - audit_coverage_rate
   - audit event count
   - compensation queue status
   - retention policy status
   - desensitization check result
7. 自动化周报验收：
   - 周报样例
   - 缺失指标清单
   - 风险项与 owner
   - 下周动作
8. 监控告警与门禁验收：
   - quality alerts
   - stability alerts
   - cost alerts
   - capacity alerts
   - gate pass/fail cases
9. 遗留问题与负责人。
10. 是否进入长期运营阶段（是/否）。

---

## 9. Phase 4 完成后下一步（明确路线衔接）

下一阶段固定进入长期运营与策略自动化阶段，按以下顺序：

1. 将 Phase 4 周报转为固定运营节奏。
2. 基于 AB 实验结果沉淀策略推荐规则。
3. 基于成本与质量趋势调整 TopK、rewrite、parent fill 与 rerank 策略。
4. 基于索引健康趋势规划周期性重建与容量扩容。
5. 基于审计与风险清单完善权限、安全与数据保留策略。
6. 在治理门禁稳定后，再评估是否引入更自动化的策略学习或跨区域容灾。

完成 Phase 4 门禁后，系统不再按一次性项目推进，而进入“指标驱动 + 门禁发布 + 周期复盘”的长期运营模式。

---

## 10. 文档维护规则（作为后续实现参考基线）

1. 任何 Phase 4 范围变更，先改本文档再改代码。
2. 新增实验类型、策略版本字段、索引版本字段、审计事件、成本指标必须同步更新对应 L0/L1/L2/L3/L4/L7 章节。
3. 每次策略发布、Collection 切换、成本治理变更或审计策略变更后，必须补充“阶段验收模板”记录。
4. 周报模板、运行手册、回滚 SOP 的变更必须在本文件或关联文档中留痕。
5. 后续按本文档逐项实现时，以本版本为 Phase 4 唯一参考。
