# RAG 企业级融合实施路线图（功能大纲 + 分阶段实现 + 测试方案）

## 1. 文档目的与使用方式

本文档融合以下两份方案，并升级为一份可直接执行的企业级路线图：

1. `backend/docs/rag-upload-chunking-collaboration-plan.md`
2. `backend/docs/enterprise-rag-implementation-playbook.md`

你可以把它当成项目主线任务单来使用：

1. 按阶段实现，不跨阶段硬上复杂能力。
2. 每个阶段完成后，按“阶段验收 + 回归测试”过门禁。
3. 门禁通过后，进入下一阶段。

说明：你提到的“Reg 优化”，本文按“RAG 检索与召回优化（含 rerank/检索策略优化）”处理。

---

## 2. 当前项目现状（基于代码扫描）

## 2.1 已有基础能力（可复用）

1. Milvus 管理、切割、索引、检索模块已存在。
2. Markdown 导入与文本切割能力已存在。
3. MQ 异步消费框架已存在（当前用于简历解析）。
4. 前端已有上传与轮询状态交互模式（简历场景）。

## 2.2 关键缺口（必须补齐）

1. 服务启动未真正启用 Milvus 在线链路。
2. 上传流程仍是“简历解析”，缺少“知识库文档入库”独立链路。
3. 缺少知识库域模型（知识库/文档/入库任务）与 API。
4. 缺少企业级评测闭环（召回率、引用准确率、回归门禁）。

---

## 3. 企业级建设目标（对标标准）

## 3.1 业务目标

1. 文档上传后可稳定入库并可检索。
2. 回答有引用、可溯源、可审计。
3. 检索质量可持续优化，并有数据证明提升。

## 3.2 技术目标

1. 稳定性：高成功率、可重试、可降级。
2. 质量：召回率、准确率、引用质量可量化。
3. 运维：全链路可观测、告警、可回滚。
4. 成本：可监控 token 与检索开销。

## 3.3 建议核心指标（第一版）

1. 入库成功率 >= 99%（非损坏文件）。
2. 检索 Recall@10 >= 0.85（核心评测集）。
3. 引用准确率 >= 0.95（回答引用与证据一致）。
4. 查询链路 P95 <= 3s（普通查询场景）。

---

## 4. 融合后的功能实现大纲（全景）

## 4.1 能力分层

1. 数据接入层：上传、文件校验、格式解析、去重。
2. 入库编排层：任务状态机、异步消费、重试补偿。
3. 检索层：向量检索、混合检索、动态 TopK、父子块。
4. 生成层：回答生成、引用约束、证据不足拒答。
5. 治理层：评测体系、监控告警、审计、安全、成本。

## 4.2 功能优先级

1. P0：先打通闭环（可用）。
2. P1：再做稳定与可观测（可上线）。
3. P2：重点做检索质量优化（可提升）。
4. P3：做高级检索能力（可领先）。
5. P4：做治理与规模化（可持续）。

## 4.3 可借鉴能力（来自候选实现，选择性吸收）

结合 `backend1` 的已有实现与踩坑记录，可以明确吸收以下能力，但不直接照搬整套架构：

1. 统一检索执行路径：检索层应统一走单一路径，保证返回结果始终带 `score`，避免不同分支字段不一致。
2. 检索结果标准化：统一输出 `content/score/citation/source/route`，为前端展示、调试和后续 rerank 留出稳定契约。
3. 混合检索标准流水线：采用“`dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate`”的可演进结构，而不是临时拼接。
4. 过滤后兜底策略：当 rerank 或 filter 后结果被全部裁空时，保留受控 fallback，避免用户看到大量无意义空结果。
5. 查询改写与术语扩展：对缩写词、实体词、别名、错拼问题，后续引入 rewrite/term expansion 会有明显收益。
6. Collection 一致性校验：启动期与诊断期都要校验导入 Collection、服务查询 Collection、当前 active Collection 是否一致。
7. 结构化检索日志：记录 query、rewrite、route 贡献、过滤表达式、召回数量、rerank 数量、最终返回数量、耗时分布。
8. 问题清单沉淀机制：把“score 丢失”“Collection 配错”“过滤后为空”等问题沉淀成 `KNOWN_ISSUES` 风格文档和回归用例。

同时明确不直接复用以下做法：

1. 不照搬与面试 Agent 强耦合的 RAG 工具组织方式。
2. 不接受 Milvus 初始化失败但服务继续以半可用状态运行。
3. 不把多路检索、改写、融合、过滤、格式化全部堆在一个超大文件中实现。

统一口径说明：

1. `统一检索结果契约` 固定指：`content/score/citation/source`。
2. `source` 在 Phase 0 最小包含：`route/collection`；Phase 2 起可扩展 `rerank_score/highlight`。
3. `Collection 一致性校验` 固定指：导入 Collection、查询 Collection、当前 active Collection 三者一致。
4. `结构化检索日志` 固定至少包含：`query/user_id/kb_id/expr/topk/rewrite/routes/final_count/duration_ms`。

---

## 5. 清晰实现路线（阶段化执行）

以下路线可直接作为你“一个一个实现”的执行顺序。

## Phase 0（P0）闭环可用期

**目标**：让“上传 -> 切割 -> 向量入库 -> 检索 -> 返回引用”跑通。

### 5.0.1 要实现的功能

1. 启用 MilvusManager 在线初始化与健康检查。
2. 新增知识库模型：
   - `kb_knowledge_base`
   - `kb_document`
   - `kb_ingest_job`
3. 新增知识库 API：
   - 创建知识库
   - 上传文档
   - 任务状态查询
   - 文档列表/删除
   - 检索接口
4. 新增 MQ 消息类型 `knowledge_ingest`。
5. 消费逻辑：
   - 解析文本（pdf/md/txt）
   - 切块
   - metadata 注入（user_id/kb_id/document_id/chunk_index）
   - 向量入库
   - 状态回写
6. 前端新增知识库管理页最小能力：
   - 上传
   - 状态轮询
   - 删除
7. 增加低风险基线增强项：
   - 基于 `file_hash` 的重复文档识别与幂等保护
   - 启动时校验当前查询 Collection 是否存在且配置一致
   - 检索结果统一返回 `score/citation/source`
   - 检索路径统一，避免部分查询结果缺失 `score`

### 5.0.2 完成判定

1. 任意文档上传后可在任务页看到最终状态。
2. 检索可命中刚上传文档内容。
3. 删除文档后检索不再返回该文档 chunk。

### 5.0.3 完成后下一步

进入 Phase 1（先做生产可用性，不先做高级检索）。

---

## Phase 1（P1）生产可用期

**目标**：从“能跑”升级到“能稳定上线”。

### 5.1.1 要实现的功能

1. 任务状态机完善：`pending -> processing -> completed/failed`。
2. 重试与补偿：
   - 自动重试次数
   - 失败原因记录
   - 人工重试接口
3. 安全与隔离：
   - 检索强制带 `user_id + kb_id` 过滤
   - 上传权限与参数校验
4. 基础答案溯源（必须）：
   - 回答返回 doc/chunk 引用信息
5. 可观测性：
   - 入库成功率
   - 平均入库耗时
   - 检索耗时
   - 空结果率
6. 检索与入库结构化日志：
   - query / kb_id / user_id / expr
   - 召回数量、最终返回数量、空结果原因
   - 分阶段耗时（embedding / search / rerank / answer）
7. 失败分类：
   - 配置错误
   - 文件解析失败
   - embedding 失败
   - 向量库写入失败
   - 检索过滤后为空

### 5.1.2 完成判定

1. 失败任务可重试并成功恢复。
2. 检索无跨用户数据泄露。
3. 回答具备基础引用信息。
4. 关键指标可在日志/监控中看到。

### 5.1.3 完成后下一步

进入 Phase 2（开始做检索与召回质量优化）。

---

## Phase 2（P2）检索质量优化期（核心）

**目标**：显著提高召回质量与回答可靠性。

### 5.2.1 要实现的功能

1. 混合检索（优先项）：
   - 向量召回 + 关键词/BM25 召回
   - 融合后 rerank
   - 固化流水线：`dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate`
2. 动态 TopK（第一版，规则驱动）：
   - 按 query 类型与长度调整 K
   - 设置 `min_k/max_k` 与 token 预算守卫
3. 索引优化（第一轮）：
   - 针对当前数据规模调参
   - 形成“召回率-延迟”对比报告
4. 答案溯源增强：
   - 段落/片段级引用
   - 引用字段标准化
5. 建立离线评测集与回归脚本（必须）：
   - Recall@K
   - MRR
   - nDCG
   - 引用准确率
6. 查询改写与术语扩展（受控上线）：
   - 缩写词、实体词、别名扩展
   - rewrite 前后效果对比
   - 失败可一键关闭
7. 过滤后结果兜底：
   - 记录 `Empty-After-Filter`
   - 在可控条件下返回最小候选，避免误判为系统异常

### 5.2.2 完成判定

1. 相比 Phase 1，核心评测集 Recall@10 有可观提升。
2. 混合检索在实体词/缩写词场景优于纯向量。
3. 动态 TopK 不增加失控成本（有预算控制）。
4. 每次检索策略改动都可跑离线回归。

### 5.2.3 完成后下一步

进入 Phase 3（高级检索，父子块与策略化 TopK）。

---

## Phase 3（P3）高级检索能力期

**目标**：在 Phase 2 检索质量基线稳定后，解决长文档、复杂问题、证据不足与长尾表达带来的高级检索质量问题。

本阶段不是“堆更多策略”，而是把 Phase 2 已经跑通的 `dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate` 主链路升级为可解释、可灰度、可评测的高级检索系统。

统一口径说明：

1. `父子块检索` 固定指：子块用于精确召回，父块/邻近块用于上下文回填，最终仍返回可定位 citation。
2. `策略版动态 TopK` 固定指：基于分数分布、重排间距、query 类型、token 预算与证据密度共同决策最终 K。
3. `证据不足拒答` 固定指：当候选证据置信度不足或引用无法支撑回答时，返回标准拒答模板，不强行生成。
4. `高级查询改写` 固定指：在保留原 query 的前提下，对领域术语表、route-specific rewrite、模型辅助 rewrite 做受控增强。
5. `模型辅助 rewrite` 不直接替换原 query，只允许生成受限 expansion，并必须绑定 A/B、离线评测与 `Rewrite Gain`。

### 5.3.1 Phase 3 范围边界

#### 本阶段必须完成

1. 父子块检索上线，支持子块召回、父块回填、邻近窗口回填与 token 预算控制。
2. 策略版动态 TopK 上线，基于候选质量与 token 预算动态截断。
3. 证据不足拒答策略上线，降低无证据或弱证据问题的幻觉风险。
4. 引用一致性校验上线，检查回答关键句与引用片段是否匹配。
5. 检索调试视图上线，支持查看 rewrite、route contribution、parent-child 回填差异与拒答原因。
6. 查询改写升级到高级版：领域动态术语表、route-specific rewrite、模型辅助 rewrite 灰度实验。
7. 建立 Phase 3 离线回归门禁，所有高级策略必须输出收益、延迟与风险对比。

#### 本阶段明确不做

1. 全自动策略学习与在线强化学习。
2. 无人工审核的模型 rewrite 全量上线。
3. 多向量库异构容灾与跨区域多活。
4. 索引生命周期自动治理（重建、冷热迁移、多版本自动切换），该能力放入 Phase 4。
5. 把拒答策略做成大模型自由判断；Phase 3 先以规则、分数与引用校验为主。

### 5.3.2 目标与通过标准（Gate）

Phase 3 通过标准（全满足）：

1. 长文档/多段证据问题的 Recall@10、nDCG@10 或答案完整性相较 Phase 2 基线有可观提升。
2. 父子块检索在长文档评测集上优于普通 chunk 检索，且 P95 延迟退化在可接受范围内。
3. 证据不足场景的强答率明显下降，且正常有证据问题不被过度拒答。
4. 引用一致性校验能发现并拦截一批 citation 不支撑回答的样本。
5. 高级 rewrite 策略必须输出 `Rewrite Gain`，且能区分规则、领域词表、route-specific、模型辅助各自贡献。
6. 任意 Phase 3 策略可独立关闭，并可在 10 分钟内回滚到 Phase 2 稳定路径。

### 5.3.3 实现路线总览（L0 -> L8）

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

### 5.3.4 L0 Phase 2 基线冻结、策略开关与评测集扩展

#### 目标

在进入高级检索前冻结 Phase 2 稳定基线，避免后续策略收益无法归因。

#### 功能任务

1. 固定 Phase 2 基线快照：
   - 检索配置
   - route 权重
   - rewrite 规则版本
   - rerank 版本
   - Recall@K、MRR、nDCG、Citation Accuracy、P95 延迟
2. 新增 Feature Flag：
   - `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
   - `RAG_ENABLE_STRATEGIC_TOPK`
   - `RAG_ENABLE_EVIDENCE_REFUSAL`
   - `RAG_ENABLE_CITATION_CONSISTENCY`
   - `RAG_ENABLE_DOMAIN_TERMS`
   - `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE`
   - `RAG_ENABLE_MODEL_ASSISTED_REWRITE`
3. 扩展 Phase 3 评测集：
   - 长文档深问
   - 多段证据综合
   - 证据不足问题
   - 缩写歧义问题
   - 领域术语问题
   - 口语化/轻微错拼问题
4. 固化实验组：
   - `phase2_baseline`
   - `parent_child`
   - `parent_child+strategic_topk`
   - `parent_child+refusal`
   - `parent_child+advanced_rewrite`

#### 验收

1. Phase 2 基线可复跑、可比较、可回滚。
2. 所有 Phase 3 策略可独立开关，不影响 Phase 2 主路径。
3. 评测集能覆盖长文档、证据不足与高级 rewrite 场景。

### 5.3.5 L1 父子块元数据与切块协议升级

#### 目标

让入库数据具备“子块精确召回、父块完整回填”的结构基础。

#### 功能任务

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
3. 保持 citation 可定位：
   - 最终引用仍指向具体 child chunk
   - `source` 可补充 `parent_id/child_id/section_title`
4. 对历史数据提供兼容策略：
   - 无 parent metadata 时回退普通 chunk 检索
   - 支持后续重建索引补齐 parent-child metadata

#### 验收

1. 新入库文档具备完整 parent-child metadata。
2. 无 parent metadata 的旧数据不影响检索可用性。
3. citation 能定位到具体 child chunk，而不是只给大段父块。

### 5.3.6 L2 父子块检索链路（子块召回 + 父块回填）

#### 目标

用子块保证命中精度，用父块/邻近块补齐上下文完整性。

#### 功能任务

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

#### 验收

1. 父子块检索在长文档问题上优于 Phase 2 普通 chunk 检索。
2. 回填不会导致上下文 token 普遍超预算。
3. 调试日志能还原“child 命中 -> parent 聚合 -> 回填 -> 截断”的完整过程。

### 5.3.7 L3 策略版动态 TopK 与 token 预算联动

#### 目标

把 Phase 2 的规则版 TopK 升级为基于候选质量和证据密度的策略版决策。

#### 功能任务

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

#### 验收

1. 不同 query 类型下 final K 分布符合预期。
2. 平均 token 成本可控，且长文档问题完整性不下降。
3. TopK 决策原因可在日志与调试视图中解释。

### 5.3.8 L4 证据不足拒答策略与标准拒答模板

#### 目标

当检索证据不足时，系统明确拒答或提示补充材料，而不是编造答案。

#### 功能任务

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

#### 验收

1. 证据不足评测集上的强答率下降。
2. 有明确证据的问题不过度拒答。
3. 拒答原因可统计、可回放、可用于优化评测集。

### 5.3.9 L5 引用一致性校验与 citation 质量增强

#### 目标

确保回答中的关键结论能被引用片段真实支撑。

#### 功能任务

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

#### 验收

1. Citation Precision 不低于 Phase 2 基线。
2. 能识别一批“答案看似合理但引用不支撑”的样本。
3. 引用展示仍能定位到具体文档、章节与 chunk。

### 5.3.10 L6 高级查询改写（领域词表、route-specific、模型辅助灰度）

#### 目标

在不破坏 Phase 2 规则受控 rewrite 稳定性的前提下，逐步提升领域术语、不同 route 与长尾表达的召回能力。

#### 分阶段原则

1. 先做按领域动态加载术语表。
2. 再做不同 route 的 rewrite 策略分化。
3. 最后做小模型辅助 rewrite 灰度实验。

#### 功能任务

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

#### 验收

1. 领域术语表在缩写、别名、专业词场景有可量化收益。
2. route-specific rewrite 能说明 dense/sparse 各自收益与风险。
3. 模型辅助 rewrite 只以灰度实验方式存在，不替换原 query，不无监控全量上线。
4. 所有 rewrite 改动都有 `Rewrite Gain`、Route Contribution、P95 延迟与失败率报告。

### 5.3.11 L7 检索调试视图、可观测性与离线回归门禁

#### 目标

让高级检索策略的每一次命中、回填、截断、拒答、改写都可解释、可复现。

#### 功能任务

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

#### 验收

1. 调试视图能还原一次请求的完整高级检索链路。
2. 离线评测能按策略拆分输出收益与风险。
3. 门禁失败时能阻止发布或自动建议关闭对应策略。

### 5.3.12 L8 灰度发布、回滚预案与验收收口

#### 目标

确保 Phase 3 高级检索能力在真实流量中安全上线，并能快速回退到 Phase 2 稳定路径。

#### 功能任务

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
   - 关闭 `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
   - 回退到 Phase 2 混合检索路径
3. 监控看板补齐：
   - Parent Fill Gain
   - Evidence Refusal Rate
   - Refusal False Positive Rate
   - Citation Support Score
   - Route-specific Rewrite Gain
   - Model-assisted Rewrite Error Rate
4. 输出 Phase 3 验收报告：
   - 指标对比
   - 策略贡献
   - 延迟与成本变化
   - 风险与回滚演练记录

#### 验收

1. 灰度过程中关键质量指标稳定。
2. 任一高级策略异常时可独立关闭。
3. 回滚演练可在 10 分钟内恢复 Phase 2 稳定路径。

### 5.3.13 推荐实施节奏

1. 先完成 `L0`，冻结 Phase 2 基线与 Phase 3 评测口径。
2. 再完成 `L1 + L2`，打通父子块数据结构与检索主链路。
3. 然后完成 `L3 + L4 + L5`，形成“动态截断、证据拒答、引用校验”的可靠性闭环。
4. 再完成 `L6`，按“领域术语表 -> route-specific rewrite -> 模型辅助灰度”的顺序推进高级 rewrite。
5. 最后完成 `L7 + L8`，补齐调试视图、离线门禁、灰度与回滚。

### 5.3.14 角色分工（建议）

1. 后端A：L1 + L2（父子块 metadata、回填与检索链路）。
2. 后端B：L3 + L4（策略版 TopK、证据不足拒答）。
3. 后端C：L5 + L7（引用一致性、调试视图、结构化日志）。
4. 算法/检索：L6（领域术语表、route-specific rewrite、模型辅助 rewrite 实验）。
5. QA/SRE：L0 + L8（基线冻结、评测门禁、灰度监控、回滚演练）。

### 5.3.15 阶段验收模板（执行后填写）

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
5. 灰度与回滚演练结果（成功/失败 + 原因）。
6. 遗留问题与负责人。
7. 是否进入 Phase 4（是/否）。

### 5.3.16 完成后下一步

进入 Phase 4（治理、规模化与持续优化）。

---

## Phase 4（P4）企业治理与规模化期

**目标**：形成可持续优化机制。

### 5.4.1 要实现的功能

1. AB 实验与灰度发布机制。
2. 索引生命周期管理（重建/迁移/版本化）。
3. 成本看板（embedding/检索/LLM）。
4. 合规审计（操作与查询链路留痕）。
5. 自动化周报（质量/稳定性/成本）。
6. Milvus/向量库运维工具化：
   - Collection 列表、当前 active Collection 标记
   - Collection 健康检查与容量巡检
   - 重建、切换、回滚操作留痕

### 5.4.2 完成判定

1. 检索策略变更可灰度与一键回滚。
2. 指标退化可快速发现与定位。
3. 成本与质量可联动决策。

---

## 6. 阶段门禁（你每一阶段结束后做什么）

每阶段收尾必须做 4 件事：

1. 阶段验收（功能演示 + 指标对照）。
2. 回归测试（离线评测 + 冒烟链路）。
3. 风险清单更新（遗留问题与应对）。
4. 下一阶段启动包（任务分解 + owner 指派）。

---

## 7. 全面测试方案（重点：RAG/Reg 召回率优化）

## 7.1 测试分层

1. 单元测试：解析、切块、过滤表达式、metadata 构造。
2. 集成测试：上传到检索的完整链路。
3. 离线评测：召回率与引用质量。
4. 线上观测：真实流量指标与告警。

## 7.2 召回率测试（Reg/RAG 优化核心）

### 7.2.1 评测集构建

1. 建立 `qa_goldens` 数据集：
   - `query`
   - `gold_doc_ids`（标准证据文档）
   - `gold_chunk_ids`（可选）
   - `query_type`（事实/解释/多条件）
2. 按场景分层：
   - 简单事实
   - 缩写/实体词
   - 长文档深问
   - 跨段落推理

### 7.2.2 核心指标

1. HitRate@K：前 K 是否命中标准文档。
2. Recall@K：前 K 覆盖标准证据比例。
3. MRR：首个正确命中的排序质量。
4. nDCG@K：综合排序相关性。
5. Citation Precision：引用是否真实支撑回答。
6. Empty-After-Filter Rate：进入 rerank/过滤后被裁空的比例。
7. Route Contribution：`dense`、`sparse`、`rewrite` 路径各自对命中的贡献占比。
8. Rewrite Gain：启用 rewrite 后 Recall@K / HitRate@K 的增益。
9. Score Completeness：最终返回结果中 `score` 字段完整率。

### 7.2.3 对比实验设计

每次优化必须做 A/B 对比：

1. Baseline：纯向量固定 TopK。
2. Exp-1：纯向量固定 TopK + 统一 score/citation 契约。
3. Exp-2：混合检索。
4. Exp-3：混合检索 + 查询改写。
5. Exp-4：混合检索 + 动态 TopK。
6. Exp-5：混合检索 + 动态 TopK + 父子块（Phase 3 后）。

输出统一对比表：

1. 指标变化（Recall@10、MRR、nDCG、引用准确率）。
2. 延迟变化（P50/P95）。
3. 成本变化（平均 token、平均候选数）。
4. 空裁剪率变化（`Empty-After-Filter Rate`）。
5. 路由贡献变化（各 route 命中占比）。

### 7.2.4 上线门禁建议

1. Recall@10 不得低于 baseline - 2%。
2. Citation Precision 不得下降。
3. P95 延迟涨幅不得超过预设阈值（例如 15%）。
4. `Score Completeness` 必须保持 100%。
5. `Empty-After-Filter Rate` 不得异常升高。

## 7.3 动态 TopK 专项测试

1. 按 query_type 检查 K 分布是否符合预期。
2. 检查上下限是否生效（`min_k/max_k`）。
3. 检查 token 预算守卫是否拦截超长上下文。

## 7.4 父子块检索专项测试

1. 长文档问题的完整性对比（有无父块回填）。
2. 召回命中与回答完整性双指标对比。
3. 观察是否引入无关上下文噪音。

## 7.5 溯源专项测试

1. 回答中每条引用是否可定位到真实 chunk。
2. 回答关键结论是否有证据支撑。
3. 无证据问题是否触发拒答策略（Phase 3）。

## 7.6 工程稳定性专项测试

1. Collection 配置不一致时，启动期是否直接告警并阻断错误配置上线。
2. 同一文件重复上传时，`file_hash` 去重是否按预期生效。
3. 检索所有出口是否稳定返回 `score/citation/source`。
4. filter 后为空时，是否正确记录日志并触发受控兜底。
5. 检索链路日志是否能完整还原“query -> route -> rerank -> final”全过程。

---

## 8. 多人协作执行图（建议 6 角色）

1. 架构负责人：阶段门禁、技术裁决、质量基线。
2. 后端-模型层：`kb_*` 数据模型与 DAO。
3. 后端-编排层：上传 API、任务状态、MQ 发布。
4. 后端-检索层：召回、融合、动态 TopK、父子块。
5. 前端：知识库管理页、引用展示、评测对比页。
6. QA/SRE：评测框架、压测、监控告警、发布门禁。

协作原则：

1. API 与 metadata 契约先冻结。
2. 阶段内 PR 不跨职责大面积混改。
3. 每个 PR 必须带“测试证明”。

---

## 9. 立即执行的首批任务（从今天开始）

1. 完成 Phase 0 的接口与数据模型开发。
2. 同步建立最小评测集（先 30~50 条 query）。
3. Phase 0 验收通过后，立即进入 Phase 1 稳定性改造。
4. Phase 1 通过后，再进入 Phase 2 的混合检索与召回优化。

---

## 10. 你后续“一个个实现”的操作规则

后续每次推进按以下模板：

1. 选定当前阶段唯一目标（不要并行跨阶段）。
2. 先实现功能，再补对应测试。
3. 产出阶段结果：
   - 功能清单完成情况
   - 指标变化
   - 遗留风险
4. 只有阶段门禁通过，才切换下一阶段。

这份路线图就是你后续开发的主执行文档。
