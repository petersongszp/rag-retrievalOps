# Phase 3 详细功能实现路线（高级检索能力）

## 1. 文档定位

本文档是 Phase 3 的执行手册，目标是把“高级检索能力期”拆成可直接实施的细颗粒任务路线。
它有两个用途：

1. 作为团队推进 Phase 3 的统一执行文档。
2. 作为后续企业治理与规模化阶段（Phase 4）的高级检索质量、拒答可靠性与策略灰度基线。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `统一检索结果契约` 固定指：`content/score/citation/source`。
2. `source` 在 Phase 3 最小包含：`route/collection/retriever_version/rerank_score/parent_id/child_id/section_title/parent_fill_strategy`。
3. `父子块检索` 固定指：子块用于精确召回，父块/邻近块用于上下文回填，最终 citation 仍必须可定位到具体 child chunk。
4. `策略版动态 TopK` 固定指：基于分数分布、重排间距、query 类型、token 预算与证据密度共同决策最终 K。
5. `证据不足拒答` 固定指：当候选证据置信度不足或引用无法支撑回答时，返回标准拒答模板，不强行生成。
6. `高级查询改写` 固定指：在保留原 query 的前提下，对领域术语表、route-specific rewrite、模型辅助 rewrite 做受控增强。
7. `结构化检索日志` 在 Phase 3 固定扩展：`parent_child_enabled/parent_fill_strategy/parent_fill_count/evidence_gate_result/refusal_reason/citation_support_score/topk_decision_reason/rewrite_gain_bucket`。
8. `高级检索回归` 固定指：同一评测集按 `phase2_baseline` 与 `phase3_candidate` 输出 Recall@K、MRR、nDCG、Citation Precision、Parent Fill Gain、Evidence Refusal Rate、Rewrite Gain、P95 延迟对比。

---

## 2. Phase 3 范围边界

## 2.1 本阶段必须完成

1. 父子块检索上线，支持子块召回、父块回填、邻近窗口回填与 token 预算控制。
2. 策略版动态 TopK 上线，基于候选质量、证据密度与 token 预算动态截断。
3. 证据不足拒答策略上线，降低无证据或弱证据问题的强答与幻觉风险。
4. 引用一致性校验上线，检查回答关键句与引用片段是否匹配。
5. 检索调试视图上线，支持查看 rewrite、route contribution、parent-child 回填差异、TopK 决策与拒答原因。
6. 查询改写升级到高级版：领域动态术语表、route-specific rewrite、模型辅助 rewrite 灰度实验。
7. 建立 Phase 3 离线回归门禁，所有高级策略必须输出收益、延迟与风险对比。
8. 完成灰度、回滚演练与 Phase 3 验收报告，确保任一高级策略可独立关闭。

## 2.2 本阶段明确不做

1. 全自动策略学习与在线强化学习。
2. 无人工审核的模型 rewrite 全量上线。
3. 多向量库异构容灾与跨区域多活。
4. 索引生命周期自动治理（重建、冷热迁移、多版本自动切换），该能力放入 Phase 4。
5. 把拒答策略做成大模型自由判断；Phase 3 先以规则、分数与引用校验为主。
6. 全量 AB 自动实验平台；Phase 3 只做面向高级策略的灰度实验、影子评测与可回滚发布。

---

## 3. 目标与通过标准（Gate）

Phase 3 通过标准（全满足）：

1. 长文档/多段证据问题的 Recall@10、nDCG@10 或答案完整性相较 Phase 2 基线有可观提升（建议门槛：核心长文档集相对提升 >= 8%）。
2. 父子块检索在长文档评测集上优于普通 chunk 检索，且检索 P95 延迟退化在可接受范围内（建议门槛：不超过 Phase 2 基线 20%）。
3. 证据不足场景的强答率明显下降，且正常有证据问题不被过度拒答（拒答误伤率必须低于预设阈值）。
4. 引用一致性校验能发现并拦截一批 citation 不支撑回答的样本，Citation Precision 不低于 Phase 2 基线。
5. 高级 rewrite 策略必须输出 `Rewrite Gain`，且能区分规则、领域词表、route-specific、模型辅助各自贡献。
6. 任意 Phase 3 策略可独立关闭，并可在 10 分钟内回滚到 Phase 2 稳定路径。
7. 检索调试视图与结构化日志能还原一次请求的完整高级检索链路。

---

## 4. 实现路线总览（L0 -> L8）

Phase 3 按 9 条路线推进，按门禁顺序合流：

1. L0：Phase 2 基线冻结、策略开关与评测集扩展
2. L1：父子块元数据与切块协议升级
3. L2：父子块检索链路（子块召回 + 父块回填）
4. L3：策略版动态 TopK 与 token 预算联动
5. L4：证据不足拒答策略与标准拒答模板
6. L5：引用一致性校验与 citation 质量增强
7. L6：高级查询改写（领域词表、route-specific、模型辅助灰度）
8. L7：检索调试视图、可观测性与离线回归门禁
9. L8：灰度发布、回滚预案与验收收口

建议顺序：`L0 -> L1 + L2 -> L3 + L4 + L5 -> L6 + L7 -> L8`

---

## 5. 详细路线拆解

## 5.1 L0 Phase 2 基线冻结、策略开关与评测集扩展

### 目标
在进入高级检索前冻结 Phase 2 稳定基线，确保后续每个高级策略的收益、延迟与风险都可归因。

### 功能任务

1. 固定 Phase 2 基线快照：
   - 检索配置
   - route 权重
   - rewrite 规则版本
   - rerank 版本
   - Recall@K、MRR、nDCG、Citation Accuracy、P95 延迟、平均上下文 token
2. 新增 Feature Flag：
   - `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
   - `RAG_ENABLE_STRATEGIC_TOPK`
   - `RAG_ENABLE_EVIDENCE_REFUSAL`
   - `RAG_ENABLE_CITATION_CONSISTENCY`
   - `RAG_ENABLE_DOMAIN_TERMS`
   - `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE`
   - `RAG_ENABLE_MODEL_ASSISTED_REWRITE`
3. 在 `backend/internal/config/config.go` 增加 Phase 3 策略配置：
   - `parent_child_fill_strategy/parent_child_window_size/parent_child_max_tokens`
   - `strategic_topk_min_k/strategic_topk_max_k/strategic_topk_budget_ratio`
   - `evidence_min_rerank_score/evidence_min_density/evidence_min_citation_coverage`
   - `citation_check_threshold/citation_check_version`
   - `domain_term_timeout_ms/model_rewrite_timeout_ms/model_rewrite_shadow_ratio`
4. 扩展 `backend/scripts/evaluation/dataset.json` 或同类评测集：
   - 长文档深问
   - 多段证据综合
   - 证据不足问题
   - 缩写歧义问题
   - 领域术语问题
   - 口语化/轻微错拼问题
5. 固化实验组：
   - `phase2_baseline`
   - `parent_child`
   - `parent_child+strategic_topk`
   - `parent_child+refusal`
   - `parent_child+advanced_rewrite`

### 验收

1. Phase 2 基线可复跑、可比较、可回滚。
2. 所有 Phase 3 策略可独立开关，不影响 Phase 2 主路径。
3. 配置缺失或阈值非法时启动 fail-fast，并给出可读错误。
4. 评测集能覆盖长文档、证据不足与高级 rewrite 场景。

---

## 5.2 L1 父子块元数据与切块协议升级

### 目标
让入库数据具备“子块精确召回、父块完整回填”的结构基础，同时保持引用可定位与旧数据兼容。

### 功能任务

1. 扩展 chunk metadata：
   - `parent_id`
   - `child_id`
   - `document_id`
   - `chunk_index`
   - `parent_start_offset`
   - `parent_end_offset`
   - `section_title`
   - `hierarchy_path`
2. 设计父块构造规则：
   - 按标题层级生成父块
   - 按自然段窗口生成父块
   - 对超长父块执行 token 上限截断
   - 记录父块 token 数与构造策略版本
3. 保持 citation 可定位：
   - 最终引用仍指向具体 child chunk
   - `source` 补充 `parent_id/child_id/section_title/hierarchy_path`
4. 对历史数据提供兼容策略：
   - 无 parent metadata 时回退普通 chunk 检索
   - 支持后续重建索引补齐 parent-child metadata
   - 检索日志记录 `parent_child_available=false`
5. 增加 metadata 单元测试：
   - 标题层级解析
   - 父子块 offset 一致性
   - 超长父块截断
   - citation child chunk 定位

### 验收

1. 新入库文档具备完整 parent-child metadata。
2. 无 parent metadata 的旧数据不影响检索可用性。
3. citation 能定位到具体 child chunk，而不是只给大段父块。
4. metadata 构造规则有测试覆盖，异常文档不会破坏入库链路。

---

## 5.3 L2 父子块检索链路（子块召回 + 父块回填）

### 目标
用子块保证命中精度，用父块/邻近块补齐上下文完整性，提升长文档与跨段落问题的回答质量。

### 功能任务

1. 新增或完善 `parent_child_retriever`：
   - child chunk 走 Phase 2 混合召回
   - 命中 child 后按 `parent_id` 聚合
   - 根据父块、邻近块、同章节块回填上下文
2. 回填策略分层：
   - `parent_only`
   - `sibling_window`
   - `section_window`
   - `child_first_with_parent_summary`
3. 回填后去重与预算控制：
   - 同 parent 下 child 去重
   - 回填内容按 rerank/fusion 分排序
   - 超预算时优先保留命中 child 与高置信父块片段
4. 结果契约扩展：
   - `source.parent_id`
   - `source.child_id`
   - `source.parent_fill_strategy`
   - `source.parent_fill_tokens`
   - `source.original_child_score`
5. 空结果分类扩展：
   - `Empty-After-Child-Retrieve`
   - `Empty-After-Parent-Fill`
   - `Empty-After-Parent-Budget`
6. 增加降级策略：
   - parent 查询失败时回退 child-only 结果
   - parent metadata 缺失时回退 Phase 2 普通 chunk 路径
   - 回填超时时保留已命中的 child 证据

### 验收

1. 父子块检索在长文档问题上优于 Phase 2 普通 chunk 检索。
2. 回填不会导致上下文 token 普遍超预算。
3. 调试日志能还原“child 命中 -> parent 聚合 -> 回填 -> 截断”的完整过程。
4. parent 回填失败时请求仍可返回 child-only 结果，并记录降级原因。

---

## 5.4 L3 策略版动态 TopK 与 token 预算联动

### 目标
把 Phase 2 的规则版 TopK 升级为基于候选质量、证据密度与上下文预算的策略版决策。

### 功能任务

1. 新增 `strategic_topk_policy`：
   - 读取 fusion score 分布
   - 读取 rerank score 间距
   - 读取 route contribution
   - 读取 parent-child 聚合数量
   - 读取 token 预算剩余量
2. 策略规则：
   - 高置信陡降分布：提前截断
   - 平缓分布：保留更多候选
   - 多 parent 分散命中：提高上下文覆盖
   - 单 parent 高集中命中：减少冗余上下文
   - 证据不足风险高：交给拒答策略，不盲目扩大 K
3. 日志补齐：
   - `topk_policy_version`
   - `score_distribution`
   - `rerank_gap`
   - `evidence_density`
   - `topk_decision_reason`
4. 成本保护：
   - 强制 `min_k <= final_k <= max_k`
   - 强制上下文 token 不超过预算
   - 策略异常时回退 Phase 2 规则版 TopK

### 验收

1. 不同 query 类型下 final K 分布符合预期。
2. 平均 token 成本可控，且长文档问题完整性不下降。
3. TopK 决策原因可在日志与调试视图中解释。
4. 策略异常或配置缺失时可回退 Phase 2 规则版 TopK。

---

## 5.5 L4 证据不足拒答策略与标准拒答模板

### 目标
当检索证据不足时，系统明确拒答或提示补充材料，而不是编造答案。

### 功能任务

1. 新增 `evidence_gate`：
   - 最低 rerank score 阈值
   - 最低 citation 覆盖阈值
   - 最低 evidence density 阈值
   - query 类型敏感阈值
2. 拒答原因分类：
   - `No-Retrieval-Hit`
   - `Low-Rerank-Confidence`
   - `Insufficient-Citation-Coverage`
   - `Contradictory-Evidence`
   - `Out-Of-KB-Scope`
3. 标准拒答模板：
   - 说明当前知识库没有足够证据
   - 给出可追溯原因
   - 建议用户补充文档或缩小问题范围
4. 降级策略：
   - 拒答策略异常时不阻断主链路
   - 但必须记录 `evidence_gate_error`
   - 可以通过 `RAG_ENABLE_EVIDENCE_REFUSAL` 一键关闭
5. 评测集补齐：
   - 无证据问题
   - 弱证据问题
   - 有明确证据但 query 表达模糊的问题
   - 知识库范围外的问题

### 验收

1. 证据不足评测集上的强答率下降。
2. 有明确证据的问题不过度拒答。
3. 拒答原因可统计、可回放、可用于优化评测集。
4. 关闭拒答开关后，可恢复 Phase 2 生成链路。

---

## 5.6 L5 引用一致性校验与 citation 质量增强

### 目标
确保回答中的关键结论能被引用片段真实支撑，降低“答案像对但引用不支撑”的风险。

### 功能任务

1. 新增 `citation_consistency_checker`：
   - 提取回答关键句
   - 匹配引用片段
   - 计算词面重合、实体重合与语义相似度
2. 生成校验结果：
   - `citation_supported`
   - `citation_support_score`
   - `unsupported_claims`
   - `citation_check_version`
3. 对低一致性结果执行处理：
   - 降低回答置信提示
   - 触发重新截断/重新选择证据
   - 严重不一致时交给拒答模板
4. 前端引用展示增强：
   - 展示 child chunk citation
   - 展示 parent context 来源
   - 标识低支撑引用
5. 结构化日志补齐：
   - `citation_supported`
   - `citation_support_score`
   - `unsupported_claim_count`
   - `citation_check_latency_ms`

### 验收

1. Citation Precision 不低于 Phase 2 基线。
2. 能识别一批“答案看似合理但引用不支撑”的样本。
3. 引用展示仍能定位到具体文档、章节与 chunk。
4. citation 校验超时或失败时可降级，不阻断主链路。

---

## 5.7 L6 高级查询改写（领域词表、route-specific、模型辅助灰度）

### 目标
在不破坏 Phase 2 规则受控 rewrite 稳定性的前提下，逐步提升领域术语、不同 route 与长尾表达的召回能力。

### 分阶段原则

1. 先做按领域动态加载术语表。
2. 再做不同 route 的 rewrite 策略分化。
3. 最后做小模型辅助 rewrite 灰度实验。

### 功能任务

1. 按领域动态加载术语表：
   - 支持按 `kb_id` 加载术语表
   - 支持按 `language/category/document_tag` 加载术语表
   - 支持领域优先、全局兜底
   - 支持术语表版本号与热更新
   - 记录 `term_dict_scope/term_dict_version/term_hits`
2. route-specific rewrite：
   - dense route 默认保守改写，保留 query 主体语义
   - sparse/BM25 route 允许更积极的术语扩展、别名扩展、缩写展开
   - 支持 dense 使用 `original_query`，sparse 使用 `rewritten_query`
   - 记录 `route_rewrite_strategy` 与每条 route 的 `final_query`
3. 模型辅助 rewrite 灰度实验：
   - 仅对规则未覆盖但高价值的 query 触发
   - 不允许模型直接替换整句 query
   - 模型只输出结构化字段：`normalized_terms/aliases/abbreviations/must_keep_terms/risk_level`
   - 原 query 与模型 expansion 并行召回
   - 必须保留 A/B 对照和关闭开关
4. 风险控制：
   - 高风险 query 禁用模型辅助
   - `risk_level=high` 的 expansion 不进入召回
   - 模型超时、异常或输出非法时回退规则版 rewrite
   - 所有 rewrite 策略绑定离线评测并输出 `Rewrite Gain`
5. 评测拆分：
   - `rules_only`
   - `domain_terms`
   - `route_specific`
   - `model_assisted_shadow`
   - `model_assisted_ab`

### 验收

1. 领域术语表在缩写、别名、专业词场景有可量化收益。
2. route-specific rewrite 能说明 dense/sparse 各自收益与风险。
3. 模型辅助 rewrite 只以灰度实验方式存在，不替换原 query，不无监控全量上线。
4. 所有 rewrite 改动都有 `Rewrite Gain`、Route Contribution、P95 延迟与失败率报告。

---

## 5.8 L7 检索调试视图、可观测性与离线回归门禁

### 目标
让高级检索策略的每一次命中、回填、截断、拒答、改写都可解释、可复现。

### 功能任务

1. 检索调试视图展示：
   - original query
   - rewritten query
   - route-specific final query
   - route hits 与 route contribution
   - parent-child 回填前后上下文差异
   - TopK 决策原因
   - evidence gate 判定
   - citation consistency 结果
2. 结构化日志扩展：
   - `parent_child_enabled`
   - `parent_fill_strategy`
   - `parent_fill_count`
   - `evidence_gate_result`
   - `refusal_reason`
   - `citation_support_score`
   - `rewrite_gain_bucket`
3. 离线回归脚本扩展：
   - 长文档完整性指标
   - 证据不足拒答准确率
   - Citation Precision/Recall
   - Parent Fill Gain
   - Rewrite Gain
   - Route Contribution
4. 门禁规则：
   - 核心质量指标不得低于 Phase 2 基线
   - P95 延迟退化不得超过预设阈值
   - 拒答误伤率不得超过预设阈值
   - 模型辅助 rewrite 无收益时不得进入扩大灰度
5. 监控看板补齐：
   - Parent Fill Gain
   - Evidence Refusal Rate
   - Refusal False Positive Rate
   - Citation Support Score
   - Route-specific Rewrite Gain
   - Model-assisted Rewrite Error Rate

### 验收

1. 调试视图能还原一次请求的完整高级检索链路。
2. 离线评测能按策略拆分输出收益与风险。
3. 门禁失败时能阻止发布或自动建议关闭对应策略。
4. 监控看板能同时观察质量、延迟、成本与拒答误伤。

---

## 5.9 L8 灰度发布、回滚预案与验收收口

### 目标
确保 Phase 3 高级检索能力在真实流量中安全上线，并能快速回退到 Phase 2 稳定路径。

### 功能任务

1. 灰度顺序：
   - 内部环境全量
   - 长文档知识库灰度
   - 证据不足评测流量灰度
   - 高价值领域术语库灰度
   - 小流量真实用户灰度
2. 回滚顺序：
   - 关闭 `RAG_ENABLE_MODEL_ASSISTED_REWRITE`
   - 关闭 `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE`
   - 关闭 `RAG_ENABLE_DOMAIN_TERMS`
   - 关闭 `RAG_ENABLE_EVIDENCE_REFUSAL`
   - 关闭 `RAG_ENABLE_CITATION_CONSISTENCY`
   - 关闭 `RAG_ENABLE_STRATEGIC_TOPK`
   - 关闭 `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
   - 回退到 Phase 2 混合检索路径
3. 告警规则补齐：
   - P95 延迟异常
   - Parent Fill Gain 低于预期
   - Evidence Refusal Rate 异常升高
   - Refusal False Positive Rate 超阈值
   - Citation Support Score 下降
   - Model-assisted Rewrite Error Rate 升高
4. 输出 Phase 3 验收报告：
   - 指标对比
   - 策略贡献
   - 延迟与成本变化
   - 风险与回滚演练记录

### 验收

1. 灰度过程中关键质量指标稳定。
2. 任一高级策略异常时可独立关闭。
3. 回滚演练可在 10 分钟内恢复 Phase 2 稳定路径。
4. Phase 3 验收报告可支撑进入 Phase 4 决策。

---

## 6. 推荐实施节奏（无固定时长）

## 6.1 阶段推进建议

1. 先完成 `L0`，冻结 Phase 2 基线与 Phase 3 评测口径。
2. 再完成 `L1 + L2`，打通父子块数据结构与检索主链路。
3. 然后完成 `L3 + L4 + L5`，形成“动态截断、证据拒答、引用校验”的可靠性闭环。
4. 再完成 `L6`，按“领域术语表 -> route-specific rewrite -> 模型辅助灰度”的顺序推进高级 rewrite。
5. 最后完成 `L7 + L8`，补齐调试视图、离线门禁、灰度与回滚。

## 6.2 并行与合流规则

1. 可并行：`L1` 与 `L7` 的日志字段设计，`L3` 与 `L4` 的策略阈值设计，`L6` 的术语表准备与 `L2` 的父子块链路开发。
2. 必须串行：`L2` 依赖 `L1` metadata 稳定，`L3` 依赖 `L2` 候选与回填结果，`L8` 依赖 `L1~L7` 全部通过。
3. 统一合流：全部功能通过 `L8` 验收后再进入 Phase 4。

---

## 7. 角色分工（建议）

1. 后端A：L1 + L2（父子块 metadata、回填与检索链路）。
2. 后端B：L3 + L4（策略版 TopK、证据不足拒答）。
3. 后端C：L5 + L7（引用一致性、调试视图、结构化日志）。
4. 算法/检索：L6（领域术语表、route-specific rewrite、模型辅助 rewrite 实验）。
5. QA/SRE：L0 + L8（基线冻结、评测门禁、灰度监控、回滚演练）。

补充协作约束：

1. 后端与算法先冻结“parent-child metadata、评测集、指标口径”，再并行开发策略。
2. 后端与 QA 先冻结“拒答原因分类、引用一致性字段、回滚判定阈值”。
3. 灰度前必须完成一次“逐个关闭高级策略”的回滚演练并记录结果。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0~L8）。
2. 离线评测结果（Phase 2 baseline vs Phase 3 candidate）。
3. 指标快照：
   - Recall@10
   - MRR
   - nDCG
   - Citation Precision
   - Citation Support Score
   - Parent Fill Gain
   - Evidence Refusal Rate
   - Refusal False Positive Rate
   - Rewrite Gain
   - Route Contribution
   - 检索 P95 延迟
   - 平均上下文 token
4. 高级 rewrite 分析：
   - domain terms gain
   - route-specific gain
   - model-assisted shadow/AB gain
   - rewrite risk cases
5. 父子块检索分析：
   - child hit count
   - parent fill count
   - parent fill token cost
   - parent fill noise cases
6. 证据拒答与引用校验分析：
   - refusal true positive cases
   - refusal false positive cases
   - unsupported claims
   - low citation support cases
7. 灰度与回滚演练结果（成功/失败 + 原因）。
8. 遗留问题与负责人。
9. 是否进入 Phase 4（是/否）。

---

## 9. Phase 3 完成后下一步（明确路线衔接）

下一阶段固定进入 Phase 4（治理、规模化与持续优化），按以下顺序：

1. AB 实验与灰度发布机制平台化。
2. 索引生命周期管理（重建/迁移/版本化）。
3. 成本看板（embedding/检索/LLM）。
4. 合规审计（操作与查询链路留痕）。
5. 自动化周报（质量/稳定性/成本）。
6. Milvus/向量库运维工具化。

完成 Phase 4 门禁后，再进入长期运营与策略自动化阶段。

---

## 10. 文档维护规则（作为后续实现参考基线）

1. 任何 Phase 3 范围变更，先改本文档再改代码。
2. 新增策略开关、结果字段、评测指标必须同步更新对应 L0/L2/L3/L5/L7 章节。
3. 每次高级策略发布或回滚后补充“阶段验收模板”记录。
4. 模型辅助 rewrite 相关变更必须同步记录评测结果、灰度比例、失败率和关闭条件。
5. 后续按本文档逐项实现时，以本版本为唯一参考。
