# Phase 2 详细功能实现路线（检索质量优化）

## 1. 文档定位

本文档是 Phase 2 的执行手册，目标是把“检索质量优化”拆成可直接实施的细颗粒任务路线。
它有两个用途：

1. 作为团队推进 Phase 2 的统一执行文档。
2. 作为后续策略自动化阶段（Phase 3）的质量与评测基线。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `统一检索结果契约` 固定指：`content/score/citation/source`。
2. `source` 在 Phase 2 最小包含：`route/collection/retriever_version/rerank_score`。
3. `混合检索标准流水线` 固定指：`dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate`。
4. `结构化检索日志` 固定至少包含：`query/rewrite/final_query/user_id/kb_id/expr/topk/routes/route_hits/final_count/empty_reason/duration_ms/request_id`。
5. `检索质量回归` 固定指：同一评测集按 `baseline(phase1)` 与 `candidate(phase2策略)` 输出 Recall@K、MRR、nDCG、Citation Accuracy 对比。

---

## 2. Phase 2 范围边界

## 2.1 本阶段必须完成

1. 混合检索（Dense + BM25）上线，具备多路召回融合与去重能力。
2. 动态 TopK（规则版）上线，具备 token 预算守卫与上下限控制。
3. 查询改写与术语扩展受控接入，支持一键关闭与效果回退。
4. 重排能力从“简单重排”升级为“可配置重排”，并纳入离线评测门禁。
5. 完成第一轮索引参数优化，产出“召回率-延迟-成本”基准报告。
6. 建立检索质量离线回归机制，策略变更必须先过评测门禁。

## 2.2 本阶段明确不做

1. 父子块检索（Parent-Child Retrieval）。
2. 学习型动态 TopK（策略学习/在线学习）。
3. 全量 AB 自动实验平台。
4. 索引生命周期自动治理（自动重建/冷热迁移/多版本自动切换）。
5. 跨区域多活与多向量库异构容灾。

---

## 3. 目标与通过标准（Gate）

Phase 2 通过标准（全满足）：

1. 核心评测集 Recall@10 相比 Phase 1 基线有可观提升（建议门槛：相对提升 >= 8%）。
2. 实体词/缩写词场景下，混合检索优于纯向量检索（命中率与 MRR 同向提升）。
3. 检索 P95 延迟相较 Phase 1 不出现不可接受退化（建议门槛：退化不超过 20%，且有降级开关）。
4. 动态 TopK 生效后，平均上下文 token 成本在预算内，且回答完整性不下降。
5. 任意策略变更都可通过离线回归脚本复现结果，并可在 10 分钟内回滚到 Phase 1 策略。

---

## 4. 实现路线总览（L0 -> L8）

Phase 2 按 9 条路线推进，按门禁顺序合流：

1. L0：策略开关、配置冻结与基线快照
2. L1：混合检索召回链路（Dense + Sparse）
3. L2：融合、去重与统一打分
4. L3：查询改写与术语扩展（受控）
5. L4：动态 TopK（规则版）与 token 守卫
6. L5：重排能力升级与结果契约扩展
7. L6：索引参数优化与基准报告
8. L7：离线评测、回归门禁与贡献度分析
9. L8：灰度发布、回滚预案与验收收口

建议顺序：`L0 -> L1 + L2 -> L3 + L4 + L5 -> L6 + L7 -> L8`

---

## 5. 详细路线拆解

## 5.1 L0 策略开关、配置冻结与基线快照

### 目标
在不破坏 Phase 1 生产稳定性的前提下，为质量优化提供可灰度、可回滚、可对比的运行基础。
### 功能任务
1. 新增 Feature Flag：
   - `RAG_ENABLE_HYBRID_RETRIEVAL`
   - `RAG_ENABLE_QUERY_REWRITE`
   - `RAG_ENABLE_DYNAMIC_TOPK`
   - `RAG_ENABLE_ADVANCED_RERANK`
2. 在 `backend/internal/config/config.go` 增加 Phase 2 策略配置：
   - `hybrid_dense_weight/hybrid_sparse_weight`
   - `candidate_topk/min_topk/max_topk`
   - `rewrite_timeout_ms/rewrite_max_expansions`
   - `rerank_timeout_ms/rerank_model`
3. 启动时打印“检索策略快照摘要”（脱敏），确保排障时可还原运行参数。
4. 固定 Phase 1 基线快照（配置 + 指标 + 评测报告），作为后续对比基准。

### 验收

1. 所有 Phase 2 策略可独立开关，不影响 Phase 1 路径。
2. 配置缺失或权重非法时启动 fail-fast 并给出可读错误。
3. 能从启动日志与配置快照恢复“当时启用了哪些策略”。

---

## 5.2 L1 混合检索召回链路（Dense + Sparse）

### 目标
让检索从单路向量召回升级为多路召回，提高实体词、缩写词、错拼场景命中率。

### 功能任务

1. 在 `backend/internal/milvus/retrieval` 增加或完善模块：
   - `sparse_search.go`（关键词候选召回 + 显式倒排/BM25 排序）
   - `hybrid_search.go`（多路编排入口）
2. 统一召回输入参数：`query/expr/topk/kb_scope/kb_id/request_id`。
3. Dense 路由复用现有 `RetrieverService`，Sparse 路由采用倒排/BM25 索引实现。
   - 当前实现口径：Sparse 路由先按 query term 从 Milvus 拉取候选，再在应用侧构建显式倒排索引，并基于 BM25 完成 sparse 排序与 TopK 截断。
4. 每路召回独立打点：命中数量、耗时、错误码。
5. 召回失败容错：单路失败不拖垮整体请求（降级为可用路由）。

### 验收

1. 混合检索开关开启后，检索可同时产生 dense/sparse 两路候选。
2. 任一路由失败时请求可降级成功，且日志可见降级原因。
3. 路由命中与耗时在指标面板可观测。

---

## 5.3 L2 融合、去重与统一打分

### 目标
把多路召回结果稳定合并为一条可解释、可比较、可扩展的标准结果流。

### 功能任务

1. 增加 `fusion.go`：
   - 多路分数归一化（避免单路分数体系压制其他路由）
   - 可配置加权融合（dense_weight/sparse_weight）
2. 增加 `dedupe.go`：
   - 按 `document_id + chunk_id` 去重
   - 对重复命中保留最高融合分并记录来源路由
3. 融合后统一注入字段：
   - `score`（统一主分）
   - `source.route`（主贡献路由）
   - `source.route_contrib`（各路贡献可选）
4. 固化空结果分类：
   - `Empty-After-Retrieve`
   - `Empty-After-Fusion`
   - `Empty-After-Filter`

### 验收

1. 融合结果按统一分数排序稳定可复现。
2. 去重后结果无重复 chunk，且不丢失来源信息。
3. 空结果原因可在日志中明确区分。

---

## 5.4 L3 查询改写与术语扩展（受控）

### 目标
在“可回滚、可评估”的前提下提升 query 表达质量，减少缩写词/别名带来的召回损失。

### 功能任务

1. 增加 `rewrite.go`（或独立 `query_rewrite` 模块）：
   - 缩写扩展（如技术缩写）
   - 别名扩展（同义术语）
   - 轻量错拼纠正（规则优先）
2. 改写链路默认“受控执行”：
   - 超时即跳过
   - 扩展条数上限
   - 不可用时自动回退原 query
3. 日志补齐：
   - `original_query`
   - `rewrite_query`
   - `rewrite_strategy`
   - `rewrite_applied(bool)`
4. 增加黑名单机制：高风险 query 类型可禁用改写。

### 验收

1. 改写开关关闭时行为与 Phase 1 完全一致。
2. 改写失败不影响主流程可用性。
3. 可在离线评测中对比“改写前/后”指标差异。

---

## 5.5 L4 动态 TopK（规则版）与 token 守卫

### 目标
在保证回答完整性的同时，降低无效上下文注入与 token 成本浪费。

### 功能任务

1. 增加 `topk_policy.go`：
   - 基于 query 长度、query 类型、候选分布规则计算 K
   - 约束 `min_topk <= final_topk <= max_topk`
2. 增加 token 预算守卫：
   - 超预算时按重排分截断
   - 保留最小可回答上下文块数
3. 将 `candidate_topk` 与 `final_topk` 解耦：
   - 候选召回充足
   - 最终返回可控
4. 日志补齐字段：
   - `candidate_topk`
   - `final_topk`
   - `token_budget`
   - `truncate_reason`

### 验收

1. 动态 TopK 在规则命中时可观察到 K 值变化。
2. token 预算超限时可按策略截断且不超预算。
3. 平均上下文 token 成本可量化下降且回答质量不明显回退。

---

## 5.6 L5 重排能力升级与结果契约扩展

### 目标
将重排从“可用”提升为“可配置、可追踪、可回退”的核心排序能力。

### 功能任务

1. 在 `backend/internal/milvus/retrieval/reranker.go` 扩展重排器接口：
   - 保留当前轻量重排实现作为 fallback
   - 接入可配置重排模型（规则版或模型版）
2. 重排阶段输出增强：
   - `rerank_score`
   - `rerank_version`
   - `rerank_latency_ms`
3. 检索统一结果契约扩展：
   - 保持 `content/score/citation/source` 不变
   - 在 `source` 增加 `rerank_score`（Phase 2 最小）
4. 增加重排超时/失败降级：回退融合排序结果，保证可用性。

### 验收

1. 重排器可按配置切换，失败时可自动回退。
2. 返回结果可追踪重排分与版本。
3. 重排阶段耗时可观测，且不会导致请求普遍超时。

---

## 5.7 L6 索引参数优化与基准报告

### 目标
在现有数据规模下，找到更优的召回率-延迟平衡点并形成可复用参数基线。

### 功能任务

1. 在 `backend/internal/milvus` 建立参数扫描清单：
   - HNSW：`M/efConstruction/efSearch`
   - IVF：`nlist/nprobe`
2. 对固定评测集执行离线参数扫描：
   - 记录 Recall@K、MRR、nDCG
   - 记录 P50/P95 延迟
   - 记录资源消耗（CPU/内存）
3. 生成参数对比报告（建议落地 `backend/docs/`）：
   - baseline（Phase 1 默认参数）
   - candidate（Phase 2 调优参数）
   - 最终推荐参数与风险说明
4. 增加索引参数回滚清单：参数切换失败时恢复路径明确。

### 验收

1. 至少产出一版完整参数扫描报告。
2. 推荐参数在评测指标与延迟之间有明确收益解释。
3. 参数回滚流程可演练且可执行。

---

## 5.8 L7 离线评测、回归门禁与贡献度分析

### 目标
让每次检索策略变更都可量化、可复现、可比较，避免“凭感觉调参”。

### 功能任务

1. 扩展 `backend/scripts/evaluation/dataset.json`：覆盖缩写词、实体词、同义词、长 query、歧义 query。
2. 扩展 `backend/scripts/evaluation/evaluate.py`：
   - 支持 baseline/candidate 双跑对比
   - 输出 Recall@K、MRR、nDCG、Citation Accuracy
3. 增加策略贡献分析：
   - `dense_only`
   - `hybrid`
   - `hybrid+rewrite`
   - `hybrid+rewrite+dynamic_topk`
4. 在 CI 或发布前流程加入“检索质量门禁”：低于阈值拒绝合并/发布。
5. 固化回归报告模板：变更说明、指标变化、风险结论、回滚策略。

### 验收

1. 策略改动可自动产出对比报告。
2. 关键指标低于阈值时门禁生效。
3. 路由贡献度可回答“哪条策略真正带来收益”。

---

## 5.9 L8 灰度发布、回滚预案与验收收口

### 目标
确保 Phase 2 策略在真实流量下安全落地，并且出现退化时可快速止损。

### 功能任务

1. 灰度顺序：
   - 内部环境全量
   - 小流量用户灰度
   - 分批扩量
2. 监控看板补齐（复用 `backend/internal/observability/metrics/rag_metrics.go`）：
   - route 命中贡献
   - 空结果原因占比
   - rewrite 命中率
   - rerank 耗时
3. 告警规则补齐：
   - 召回率回退（离线门禁 + 在线代理指标）
   - P95 延迟异常
   - Empty-After-Filter 激增
4. 回滚预案：
   - 关闭 `RAG_ENABLE_HYBRID_RETRIEVAL`
   - 关闭 `RAG_ENABLE_QUERY_REWRITE`
   - 关闭 `RAG_ENABLE_DYNAMIC_TOPK`
   - 回退到 Phase 1 检索路径
5. 输出 Phase 2 验收报告与上线复盘记录。

### 验收

1. 灰度过程中关键指标稳定，无大面积质量回退。
2. 出现异常时可在 10 分钟内回滚到 Phase 1。
3. 验收报告可支撑进入 Phase 3 决策。

---

## 6. 推荐实施节奏（无固定时长）

## 6.1 阶段推进建议

1. 先完成 `L0`，冻结开关与参数口径。
2. 再完成 `L1 + L2`，形成“多路召回-融合-去重”主链路。
3. 然后完成 `L3 + L4 + L5`，打通改写、动态 TopK、重排闭环。
4. 再完成 `L6 + L7`，通过索引调优与离线评测建立门禁。
5. 最后执行 `L8`，完成灰度、回滚演练与交付收口。

## 6.2 并行与合流规则

1. 可并行：`L1` 与 `L6`（前者开发、后者准备扫描方案），`L3` 与 `L4`。
2. 必须串行：`L5` 依赖 `L2` 融合结果稳定，`L8` 依赖 `L1~L7` 全部通过。
3. 统一合流：全部功能通过 `L8` 验收后再进入 Phase 3。

---

## 7. 角色分工（建议）

1. 后端A：L0 + L3（配置、开关、query rewrite）。
2. 后端B：L1 + L2（hybrid 召回、融合去重、统一打分）。
3. 后端C：L4 + L5（动态 TopK、重排升级、结果契约）。
4. 算法/检索：L6 + L7（参数扫描、评测门禁、贡献分析）。
5. SRE/QA：L8（灰度、监控告警、回滚演练、验收收口）。

补充协作约束：

1. 后端与算法先冻结“评测集与指标口径”，再并行开发策略。
2. 后端与 QA 先冻结“空结果原因分类”和“回滚判定阈值”。
3. 灰度前必须完成一次“开关回滚演练”并记录结果。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0~L8）
2. 离线评测结果（baseline vs candidate）
3. 指标快照：
   - Recall@10
   - MRR
   - nDCG
   - Citation Accuracy
   - 检索 P95 延迟
   - 平均上下文 token
4. 路由贡献分析（dense/sparse/rewrite/dynamic_topk）
5. 灰度与回滚演练结果（成功/失败 + 原因）
6. 遗留问题与负责人
7. 是否进入 Phase 3（是/否）

---

## 9. Phase 2 完成后下一步（明确路线衔接）

下一阶段固定进入 Phase 3（策略自动化与复杂场景增强），按以下顺序：

1. 父子块检索（Parent-Child）
2. 动态 TopK（策略版）
3. 证据不足拒答策略
4. 策略灰度 AB 实验平台

完成 Phase 3 门禁后，再进入 Phase 4 的索引治理自动化与规模化运营。

---

## 10. 文档维护规则（作为后续实现参考基线）

1. 任何 Phase 2 范围变更，先改本文档再改代码。
2. 新增策略开关、结果字段、评测指标必须同步更新对应 L0/L5/L7 章节。
3. 每次策略发布或回滚后补充“阶段验收模板”记录。
4. 后续我按本文档逐项实现时，以本版本为唯一参考。
