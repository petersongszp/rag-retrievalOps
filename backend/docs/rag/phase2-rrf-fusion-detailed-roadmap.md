# Phase 2.1 详细功能实现路线（RRF 融合改造）

## 1. 文档定位

本文档是 `Phase 2` 检索质量优化阶段里的一个专项执行手册，目标是把“当前 min-max 加权融合升级为 RRF 融合”拆成可直接实施的细颗粒任务路线。

它有三个用途：

1. 作为团队推进 `RRF` 改造的统一执行文档。
2. 作为当前混合检索 `fusion` 层的设计收敛文档，避免大家一边讨论一边各自理解不同。
3. 作为后续 `Phase 3` 高级检索策略演进的稳定基线，尤其是 `route contribution`、`rewrite gain`、`TopK` 策略和离线评测门禁的基础。

本文档风格与现有路线文档保持一致：

目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `当前融合方案` 固定指：`分路 min-max 归一化 + dense/sparse 权重相加`。
2. `RRF` 固定指：`Reciprocal Rank Fusion`，即按排名而不是按原始分数做融合。
3. `混合检索标准流水线` 固定指：`dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate`。
4. `统一检索结果契约` 固定指：`content/score/citation/source`。
5. `贡献分析` 固定至少区分：
   - `route_hits`
   - `route_participation`
   - `primary_route`
   - `route_contrib`
6. `离线回归门禁` 固定至少输出：
   - `Recall@K`
   - `MRR`
   - `nDCG`
   - `Citation Accuracy`
   - `P95 latency`
   - `Empty-After-*`
   - `dense/sparse participation`

---

## 2. 本专项的背景与问题定义

当前项目的混合检索主链路已经跑通，具备：

1. `dense` 向量召回
2. `sparse` 关键词召回
3. `fusion`
4. `dedupe`
5. `rerank`
6. `debug trace`
7. `retrieve log`

但当前 `fusion` 层存在一个结构性问题：

1. 它不是 `RRF`
2. 它使用的是每路独立的 `min-max` 归一化
3. 再乘 `dense_weight/sparse_weight`

这套方案在工程上能跑，但有明显问题：

1. `dense_score` 和 `sparse_score` 本来就不是同一个分数体系，强行做分路 min-max 后再加权，解释性弱。
2. 当 `sparse` 候选很少时，路由内最低分很容易被直接归零。
3. 当前 `sparse` 路由本来就偏弱，如果融合再压制，会进一步放大 dense 的主导地位。
4. `SparseContribution = 0` 容易被误读成“sparse 没参与”，但实际上可能只是主路由不是 sparse。
5. 当前融合策略对 `TopK`、`rerank`、`rewrite gain` 的协同不够稳定。

因此，这次改造的目标不是“再调一调权重”，而是把融合底座从“分数驱动但分数体系不一致”升级成“排名驱动、跨路更稳的 RRF 融合”。

---

## 3. 本专项范围边界

## 3.1 本阶段必须完成

1. 在现有混合检索主链路中引入真正的 `RRF` 融合实现。
2. 保持现有 `dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate` 主流水线不被破坏。
3. 保证 `RRF` 可通过配置开关灰度切换，不影响当前线上默认路径的可回滚性。
4. 补齐 `RRF` 路由参与度、主路由、融合前后排名变化等可观测字段。
5. 建立 `min-max baseline` 与 `RRF candidate` 的离线回归对比门禁。
6. 产出一版可复用的 `RRF` 验收报告与回滚预案。

## 3.2 本阶段明确不做

1. 不在本阶段重写 `sparse` 候选召回底座，不把 `LIKE + Query + 本地BM25` 替换成独立全文检索引擎。
2. 不在本阶段引入学习排序模型或神经融合模型。
3. 不在本阶段重做 `reranker` 主体，只做与 `RRF` 的兼容对齐。
4. 不在本阶段引入全自动 AB 实验平台。
5. 不在本阶段改造父子块检索、证据拒答、citation consistency 主逻辑。

---

## 4. 目标与通过标准（Gate）

Phase 2.1 通过标准（全满足）：

1. 代码层面存在真实 `RRF` 实现，而不是文档或配置名义上的 “RRF”。
2. `RRF` 与当前 `min-max` 融合可通过开关切换，且切换不破坏主检索契约。
3. 离线评测中，`RRF` 相比当前 `min-max`，在实体词、缩写词、短 query、sparse 候选少的场景下，`Recall@K`、`MRR` 或 route participation 至少有一项稳定改善。
4. `RRF` 不导致检索 P95 延迟出现不可接受退化。
5. 贡献分析可以明确回答：
   - sparse 是否参与了最终候选
   - sparse 是否进入最终 TopK
   - sparse 是主路由还是辅助路由
6. 出现退化时，可在 10 分钟内回滚到当前 `min-max` 融合方案。

---

## 5. 实现路线总览（L0 -> L8）

本专项按 9 条路线推进，按门禁顺序合流：

1. L0：基线冻结、问题口径统一与开关设计
2. L1：RRF 融合核心实现
3. L2：去重、主路由与贡献统计口径对齐
4. L3：调试信息、日志与指标扩展
5. L4：配置接入、灰度切换与回滚开关
6. L5：RRF 单元测试、回归测试与边界测试
7. L6：离线评测脚本与对比报告扩展
8. L7：线上观察指标、风险阈值与发布门禁
9. L8：灰度、验收、回滚演练与收口

建议顺序：

`L0 -> L1 + L2 -> L3 + L4 + L5 -> L6 -> L7 -> L8`

---

## 6. 详细路线拆解

## 6.1 L0 基线冻结、问题口径统一与开关设计

### 目标

在改 fusion 之前，先冻结当前 `min-max` 融合行为、日志口径和评测对比基线，避免改完以后无法解释收益或退化。

### 功能任务

1. 固定当前 `min-max` 融合基线快照：
   - `DenseWeight`
   - `SparseWeight`
   - `fusion_score`
   - `route_contrib`
   - `DenseContribution`
   - `SparseContribution`
2. 明确现有指标口径：
   - `route_hits`：该路实际召回数
   - `route_participation`：该路是否参与最终候选融合
   - `primary_route`：最终保留结果的主路由
   - `route_contrib`：最终结果的多路贡献
3. 新增专项 Feature Flag 设计：
   - `RAG_ENABLE_RRF_FUSION`
   - `RAG_RRF_K`
   - `RAG_RRF_DENSE_WEIGHT`
   - `RAG_RRF_SPARSE_WEIGHT`
4. 新增融合策略标识：
   - `fusion_strategy=minmax_v1`
   - `fusion_strategy=rrf_v1`
5. 在文档和团队口径里统一一句话：
   - `sparse_contribution=0` 不等于 sparse 没执行
   - 当前值更接近“最终主路由占比”

### 验收

1. 当前线上 `min-max` 行为可被完整复现。
2. 新旧融合策略的配置字段与日志字段命名冻结。
3. 团队在评审时使用同一套“参与/主路由/贡献”口径。

---

## 6.2 L1 RRF 融合核心实现

### 目标

把当前 `fusion.go` 从“按分数归一化融合”升级为“按排名融合”的真实 `RRF` 实现。

### 功能任务

1. 在 `backend/internal/milvus/retrieval/fusion.go` 扩展融合配置：
   - `FusionStrategy`
   - `RRFK`
   - `DenseWeight`
   - `SparseWeight`
2. 新增 `RRF` 计算逻辑：
   - 对每条 route 先按本路结果排序
   - 为每个候选计算 `1 / (k + rank)`
   - 可选接入 route 权重，形成 `weighted_rrf_score`
3. 为每个候选保留以下字段：
   - `rrf_score`
   - `route_rank`
   - `route_rrf_contrib`
   - `fusion_strategy`
4. 保留现有 `min-max` 实现作为 baseline 和回滚路径，不直接删除。
5. 统一 `score` 主分写法：
   - `min-max` 路径写 `score=fusion_score`
   - `RRF` 路径写 `score=rrf_score`

### 设计约束

1. RRF 不直接依赖 `dense_score` 和 `sparse_score` 的绝对值可比性。
2. RRF 必须兼容某一路候选为空、某一路候选极少、某一路报错降级的情况。
3. RRF 结果要保留调试可解释性，不能只返回一个黑盒总分。

### 验收

1. 代码中存在真实的 `RRF` 融合实现。
2. 单路候选为空时，另一条路仍能正确产出 RRF 结果。
3. `RRF` 结果中可追踪每路 rank 和贡献。

---

## 6.3 L2 去重、主路由与贡献统计口径对齐

### 目标

保证 `RRF` 引入后，`dedupe` 和贡献统计仍然能正确表达“谁参与了、谁主导了、谁被保留了”。

### 功能任务

1. 在 `backend/internal/milvus/retrieval/dedupe.go` 中扩展去重后字段：
   - `primary_route`
   - `route_contrib`
   - `route_raw_scores`
   - `route_rank`
   - `route_rrf_contrib`
2. 明确主路由选取规则：
   - 当前默认仍按融合后最高分来源 route 作为 `primary_route`
3. 扩展 `countRouteContributions(...)` 的口径：
   - 继续保留主路由贡献数
   - 另外增加“参与贡献数”统计
4. 新增建议指标字段：
   - `DenseParticipation`
   - `SparseParticipation`
   - `DualRouteFinalCount`
   - `PrimaryDenseCount`
   - `PrimarySparseCount`

### 设计约束

1. 不能让 `RRF` 上线后贡献指标比现在更难解释。
2. 不能继续只依赖 `route` 一个字段回答所有问题。
3. 需要同时满足“上层简单可读”和“调试足够细”。

### 验收

1. 单条结果可以清楚看出：
   - 哪些 route 命中过
   - 哪些 route 参与过融合
   - 最终主路由是谁
2. 汇总指标可以区分“参与占比”和“主路由占比”。

---

## 6.4 L3 调试信息、日志与指标扩展

### 目标

让 `RRF` 的行为可回放、可解释、可诊断，不变成“算法换了但没人知道为什么结果变了”。

### 功能任务

1. 扩展 `debug_trace`：
   - `fusion_strategy`
   - `rrf_k`
   - `route_rank`
   - `route_rrf_contrib`
   - `pre_fusion_rank`
   - `post_fusion_rank`
2. 扩展 `kb_retrieve_log` 落库字段：
   - `fusion_strategy`
   - `dense_participation`
   - `sparse_participation`
   - `dual_route_final_count`
3. 扩展日志打印：
   - 记录 `fusion_strategy`
   - 记录 `route_participation`
   - 记录 `primary_route_distribution`
4. 扩展 metrics：
   - `rag_retrieve_fusion_strategy_total`
   - `rag_retrieve_route_participation_total`
   - `rag_retrieve_primary_route_total`

### 验收

1. 任意一条请求可以从 debug trace 看清 RRF 细节。
2. 可以回答“某条 sparse 候选为什么输了”。
3. 可以从汇总指标看出 RRF 是否让 sparse 参与变多。

---

## 6.5 L4 配置接入、灰度切换与回滚开关

### 目标

让 `RRF` 成为一个可灰度、可关闭、可快速回滚的策略，而不是一次性硬切。

### 功能任务

1. 在 `backend/internal/config/config.go` 和 `backend/config.yaml` 中增加 RRF 配置：
   - `fusion_strategy`
   - `rrf_k`
   - `rrf_dense_weight`
   - `rrf_sparse_weight`
2. 在 `backend/internal/milvus/init.go` 中把 RRF 参数接进 `HybridRetrieverConfig`
3. 增加开关优先级规则：
   - `EnableHybridRetrieval=false` 时不进入 fusion 改造
   - `EnableHybridRetrieval=true && EnableRRFFusion=false` 时继续走 `min-max`
   - `EnableRRFFusion=true` 时走 `RRF`
4. 保留 `min-max` 原路径，作为即时回滚方案。

### 验收

1. 配置切换可在不改代码的情况下完成。
2. 切换 `RRF` 后主流程正常。
3. 关闭 `RRF` 后可无损回退到当前基线。

---

## 6.6 L5 RRF 单元测试、回归测试与边界测试

### 目标

用测试把 RRF 设计固化下来，避免后面改着改着又退回“看起来像 RRF，实际不是”。

### 功能任务

1. 新增 RRF 单元测试：
   - dense/sparse 都有候选
   - 只有 dense
   - 只有 sparse
   - 一路候选数远大于另一路
   - 重复 chunk 双路同时命中
2. 新增去重协同测试：
   - RRF 融合后去重是否保留 route 贡献
3. 新增边界测试：
   - `rrf_k <= 0`
   - route 权重非法
   - 某 route 返回空数组
   - 某 route 报错降级
4. 新增回归测试：
   - 保证 `min-max` baseline 路径不被 RRF 改坏

### 验收

1. 测试能覆盖 RRF 主路径和典型边界。
2. 改动 fusion 逻辑时能第一时间发现行为漂移。
3. baseline 路径测试持续通过。

---

## 6.7 L6 离线评测脚本与对比报告扩展

### 目标

让 `RRF` 改造不是“感觉上更合理”，而是能产出可量化收益报告。

### 功能任务

1. 在 `backend/scripts/evaluation/evaluate.py` 增加融合策略对比能力：
   - `minmax_v1`
   - `rrf_v1`
2. 扩展策略 profile：
   - `phase2_minmax_baseline`
   - `phase2_rrf_candidate`
3. 输出对比指标：
   - `Recall@K`
   - `MRR`
   - `nDCG`
   - `Citation Accuracy`
   - `dense participation`
   - `sparse participation`
   - `primary sparse ratio`
   - `P95 latency`
4. 强化场景评测集：
   - 缩写词 query
   - 短 query
   - sparse 命中少的 query
   - dense 和 sparse 候选重叠度低的 query
5. 生成专项对比报告：
   - 哪类 query 明显受益
   - 哪类 query 无明显变化
   - 哪类 query 退化

### 验收

1. RRF 改造后可自动产出 baseline/candidate 对比报告。
2. 报告可以支持“是否上线”的决策。
3. 能回答“RRF 到底提升了什么，而不是只换了个名字”。

---

## 6.8 L7 线上观察指标、风险阈值与发布门禁

### 目标

在真实流量下监控 RRF 风险，避免离线收益不错但线上体验退化。

### 功能任务

1. 建立 RRF 专项观察指标：
   - `sparse participation rate`
   - `primary sparse ratio`
   - `dual-route final ratio`
   - `empty-after-fusion rate`
   - `retrieve p95`
2. 设置风险阈值：
   - `Recall` 退化阈值
   - `P95 latency` 退化阈值
   - `Empty-After-Fusion` 激增阈值
3. 设定发布门禁：
   - 离线收益不过门禁不上线
   - 线上关键指标异常自动建议回滚
4. 在策略中心或发布摘要中展示：
   - 当前 `fusion_strategy`
   - 当前 `rrf_k`
   - 当前 route 权重

### 验收

1. 可以在线监控 RRF 是否真的提升 sparse 参与。
2. 出现风险时有明确信号，而不是靠人工猜。
3. 发布和回滚依据可以落在指标上。

---

## 6.9 L8 灰度、验收、回滚演练与收口

### 目标

确保 RRF 能安全进入真实流量，并且出问题时可迅速恢复到 `min-max`。

### 功能任务

1. 灰度顺序：
   - 内部环境全量
   - 小流量灰度
   - 分批扩量
   - 全量
2. 回滚顺序：
   - 关闭 `RAG_ENABLE_RRF_FUSION`
   - 回退到 `fusion_strategy=minmax_v1`
3. 输出专项验收报告：
   - 代码改动摘要
   - 离线收益
   - 线上指标变化
   - 问题清单
   - 回滚演练结果
4. 记录上线结论：
   - 保持 RRF
   - 继续灰度
   - 或回滚 baseline

### 验收

1. 灰度期间关键指标稳定。
2. 回滚流程清晰、可执行、演练通过。
3. 验收报告能支撑后续继续优化 sparse 或进入下一阶段。

---

## 7. 推荐实施节奏

## 7.1 阶段推进建议

1. 先完成 `L0`，统一口径、冻结 baseline、设计开关。
2. 再完成 `L1 + L2`，把 RRF 核心逻辑和贡献口径做对。
3. 再完成 `L3 + L4 + L5`，把调试、配置、测试补齐。
4. 再完成 `L6`，通过离线报告验证收益。
5. 最后执行 `L7 + L8`，完成灰度、门禁、回滚和验收。

## 7.2 并行与合流规则

1. 可并行：
   - `L3` 与 `L5`
   - `L4` 与 `L6`
2. 必须串行：
   - `L2` 依赖 `L1`
   - `L7` 依赖 `L6`
   - `L8` 依赖 `L1~L7`
3. 统一合流：
   - 通过 `L8` 验收后，RRF 才视为正式替换候选融合底座

---

## 8. 角色分工（建议）

1. 后端A：`L1 + L2`
   - 负责 `fusion.go`、`dedupe.go`、贡献统计口径改造
2. 后端B：`L3 + L4`
   - 负责日志、调试、配置、开关和初始化接入
3. 后端C：`L5`
   - 负责单元测试、边界测试、回归测试
4. 算法/检索：`L6 + L7`
   - 负责离线评测、收益分析、门禁阈值
5. QA/SRE：`L8`
   - 负责灰度、回滚演练、上线验收

补充协作约束：

1. 先冻结指标口径，再做代码和评测并行。
2. 先验证日志字段，再推进灰度。
3. 任何“RRF 收益”结论都必须带离线报告和线上观察口径。

---

## 9. 阶段验收模板（执行后填写）

1. 功能完成情况（按 `L0~L8`）
2. 代码改动范围
3. 配置与开关变更
4. 离线评测结果（`minmax_v1` vs `rrf_v1`）
5. 指标快照：
   - `Recall@10`
   - `MRR`
   - `nDCG`
   - `Citation Accuracy`
   - `dense participation`
   - `sparse participation`
   - `primary sparse ratio`
   - `P95 latency`
6. 线上灰度结果
7. 回滚演练结果
8. 遗留问题与负责人
9. 是否正式切换到 `RRF`（是/否）

---

## 10. 本专项完成后的下一步

RRF 改造完成后，下一步建议固定进入两个方向之一：

1. 方向A：继续做 `sparse` 召回增强
   - term extraction
   - 中文能力
   - 缩写词与专业词典
   - 更强全文检索底座
2. 方向B：进入 `Phase 3` 高级检索策略
   - parent-child
   - strategic topK
   - evidence refusal
   - citation consistency

建议优先顺序：

1. 先完成 RRF
2. 再看 sparse participation 是否明显改善
3. 如果 sparse 仍弱，再重投 sparse 召回底座

---

## 11. 文档维护规则

1. 任何 RRF 范围变更，先改本文档再改代码。
2. 新增字段、指标、配置开关必须同步更新 `L0/L3/L4/L6/L7` 章节。
3. 每次灰度或回滚后补充“阶段验收模板”记录。
4. 后续我按本文档逐项实现时，以本版本为唯一参考。

