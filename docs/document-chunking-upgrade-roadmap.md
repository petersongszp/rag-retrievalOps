# 文档切片能力升级功能实现路线大纲

## 1. 文档定位

本文档是 `document-chunking-comparison.md` 的落地执行版，用于指导当前 RAG RetrievalOps 平台从现有 `recursive splitter + Markdown 结构切分 + Parent-Child metadata` 升级到更稳定的结构化、语义化、上下文化切片体系。

本文档不把 Agentic Chunking 作为主链路默认方案。当前推荐路线是：

```text
Structure-aware Split -> Semantic Re-split -> Contextualized Embedding Text -> Parent-Child Retrieval
```

核心原则：

1. 先把现有工程基线用满，不推翻已有 `splitter`、`metadata`、`retrieval` 协议。
2. 先做可控、可评估、可灰度的增强，再做模型驱动的高成本实验。
3. `raw_content` 用于展示、引用与审计，`embedding_content` 用于向量化与可选 BM25 索引。
4. Agentic Chunking 只作为高价值、低吞吐文档的后置实验能力，不进入第一阶段主链路。

---

## 2. 当前状态与关键判断

## 2.1 已具备能力

1. 通用切片已基于 `eino-ext recursive splitter` 实现，默认配置在 `backend/config.yaml` 的 `DocumentSplitter` 下。
2. Markdown 专用切片已存在，支持标题、段落、代码块、列表、水平线等分隔优先级，核心文件为 `backend/internal/milvus/splitter/markdown.go`。
3. 父子块 metadata 已在切片阶段生成，核心文件为 `backend/internal/milvus/splitter/parent_child.go`。
4. 检索层已具备 Parent-Child 回填实现，核心文件为 `backend/internal/milvus/retrieval/parent_child.go`。
5. 检索链路已有混合检索、query rewrite、dynamic topk、rerank、retrieve audit 等质量与可观测基础。

## 2.2 当前主要差距

1. 后台知识库上传主链路对 `md/markdown` 文件仍走通用 `Split`，没有优先调用 `SplitMarkdownDocument`，导致 Markdown 结构切片收益没有完全进入主链路。
2. `enable_parent_child_retrieval` 当前默认关闭，Parent-Child 能力已实现但还不是默认质量基线。
3. Milvus 当前主要写入单一 `content` 字段，embedding、展示、引用共用同一份文本，没有拆分 `raw_content` 与 `embedding_content`。
4. 目前没有结构块内语义二次切分，超长段落或同一节内多主题内容仍可能被规则边界粗切。
5. 目前没有针对切片策略的独立评测集、指标与回归门禁。
6. Agentic Chunking、Late Chunking、Propositions 都还不适合作为主链路优先级，因为成本、工程复杂度与可复现性风险更高。

---

## 3. 范围边界

## 3.1 本阶段必须完成

1. 修正知识库上传主链路，让 `md/markdown` 文件优先走 Markdown 结构切片。
2. 抽象切片策略字段与版本字段，补齐 `split_strategy`、`split_version`、`embedding_build_strategy`、`context_version` 等 metadata。
3. 增加轻量 Contextual Retrieval 能力，生成 `embedding_content`，但展示与引用仍使用 `raw_content`。
4. 保留并灰度启用 Parent-Child Retrieval，验证子块召回与父块回填收益。
5. 增加结构块内语义二次切分能力，先只对超长结构块启用，且优先使用 embedding-based semantic split。
6. 建立切片评测集与回归门禁，覆盖 Markdown、TXT、PDF 抽取文本、标题依赖型 chunk、超长段落、多主题段落。
7. 在检索调试与日志中补齐切片策略、上下文化策略、Parent-Child 回填、语义二次切分等可观测字段。

## 3.2 本阶段明确不做

1. 不把 Agentic Chunking 作为默认入库主链路。
2. 不做 Late Chunking 主链路，因为当前 embedding 基础设施没有 token-level 表示或长上下文 pooling 能力。
3. 不做 Propositions 作为唯一索引层，仅保留为后续事实级补充索引。
4. 不把父块作为独立向量记录强制入库，当前仍以 child chunk 为主要召回单元。
5. 不让 LLM 自由决定所有 chunk 边界，任何模型辅助切片都必须走灰度、评测与回滚。
6. 不在第一阶段扩大 PDF/HTML/OCR 的复杂结构解析范围，先把现有 Markdown/TXT/PDF 文本抽取链路稳定下来。

---

## 4. 目标与通过标准（Gate）

本阶段通过标准如下：

1. `md/markdown` 知识库上传链路确认使用 Markdown 结构切片，标题层级、`section_title`、`hierarchy_path` 能稳定进入 metadata。
2. 新入库 chunk 同时具备 `raw_content` 与可追踪的 `embedding_content` 构建策略；引用和前端展示不被上下文化文本污染。
3. 开启 `enable_parent_child_retrieval` 的候选组在长文档、标题依赖型问题上优于当前基线。
4. 语义二次切分只作用于超长结构块，且不会显著增加普通短文档入库成本。
5. 离线评测能输出 Recall@K、MRR、nDCG、Citation Precision、Parent Fill Gain、Contextual Recall Gain、Ingest P95、平均 chunk 数、平均 embedding 文本长度。
6. 任一增强策略可独立关闭，并能回退到当前 `recursive splitter + content embedding` 路径。
7. Agentic Chunking 仅作为实验开关存在，不影响默认入库稳定性。

---

## 5. 实现路线总览（L0 -> L8）

推荐按 9 条路线推进：

1. L0：基线冻结、评测集与指标口径
2. L1：知识库主链路结构切片修正
3. L2：切片策略协议与 metadata 版本化
4. L3：`raw_content` / `embedding_content` 分离
5. L4：轻量 Contextual Retrieval 入库增强
6. L5：Parent-Child Retrieval 灰度启用与回填评估
7. L6：结构块内语义二次切分
8. L7：可观测性、调试视图与回归门禁
9. L8：Agentic Chunking 实验层与后续高级能力预留

建议顺序：

```text
L0 -> L1 -> L2 -> L3 + L4 -> L5 -> L6 -> L7 -> L8
```

---

## 6. 详细路线拆解

## 6.1 L0 基线冻结、评测集与指标口径

### 目标

先冻结当前切片与检索质量基线，避免后续每个优化点都变成“体感变好了”。这是路线里的安全带，系上它，我们就能比较大胆地开车。

### 功能任务

1. 固定当前配置快照：
   - `DocumentSplitter.ChunkSize`
   - `DocumentSplitter.OverlapSize`
   - `DocumentSplitter.Separators`
   - `rag.feature_flags.enable_parent_child_retrieval`
   - `rag.phase3.parent_child_fill_strategy`
   - `Milvus.TopK`
2. 建立切片评测集：
   - Markdown 标题层级文档
   - 标题依赖型短 chunk 文档
   - 超长段落文档
   - 同一章节多主题文档
   - PDF 抽取后的弱结构文本
   - TXT 纯文本
   - 表格或列表较多的 Markdown 文档
3. 建立对照组：
   - `baseline_recursive`
   - `markdown_structure`
   - `contextual_embedding`
   - `parent_child_enabled`
   - `semantic_resplit`
   - `agentic_shadow`
4. 扩展离线评测脚本或 profile：
   - `backend/scripts/evaluation/dataset.json`
   - `backend/scripts/evaluation/retrieval_strategy_profiles.example.json`
   - 新增切片专项 profile，例如 `chunking_strategy_profiles.example.json`
5. 统一指标口径：
   - Recall@K
   - MRR
   - nDCG
   - Citation Precision
   - Parent Fill Gain
   - Contextual Recall Gain
   - Chunk Purity
   - Chunk Self-contained Rate
   - Ingest P95
   - Embedding Text Length Avg/P95
   - Chunks Per Document Avg/P95

### 验收

1. 当前基线可复跑，评测输出能保存为 baseline snapshot。
2. 每个切片策略都能通过 profile 单独运行。
3. 指标能区分“召回变好”“引用变准”“成本变高”“chunk 数膨胀”。
4. 未通过 L0 前，不进入 L3 以后涉及索引内容变化的开发。

---

## 6.2 L1 知识库主链路结构切片修正

### 目标

让后台知识库上传主链路真正吃到 Markdown 结构切片能力，避免只有 CLI/importer 路径使用 `SplitMarkdownDocument`。

### 功能任务

1. 在 `backend/internal/ragqueue/consumer.go` 的 `ingestKnowledgeDocument` 中按 `payload.FileType` 选择切片策略：
   - `md/markdown` 调用 `SplitMarkdownDocument`
   - `txt` 调用通用 `Split`
   - `pdf` 暂时调用通用 `Split`
2. 保持原有基础 metadata 前置：
   - `tenant_id`
   - `operator_admin_id`
   - `kb_id`
   - `document_id`
   - `file_name`
   - `collection`
3. 为 chunk metadata 增加策略标记：
   - `split_strategy=markdown_structure_v1` 或 `recursive_v1`
   - `split_version`
   - `source_file_type`
4. 增加单元测试或集成测试：
   - `md` 文件走 Markdown 切片
   - `txt` 文件走通用切片
   - `pdf` 抽取文本走通用切片
   - Markdown 标题能生成 `hierarchy_path`
5. 保持旧 importer 与 `milvusctl` 路径兼容，不破坏 `SplitMarkdown`、`SplitMarkdownDocument` 旧入口。

### 验收

1. 上传 Markdown 文档后，chunk metadata 中能看到正确 `section_title` 与 `hierarchy_path`。
2. TXT/PDF 上传行为不回退、不报错。
3. `split_strategy` 能在 Milvus metadata 与 retrieval source 中追踪到。
4. 对旧数据无破坏，历史 chunk 未带 `split_strategy` 时按 `legacy` 处理。

---

## 6.3 L2 切片策略协议与 metadata 版本化

### 目标

把切片结果从“只有内容和部分父子字段”升级为可审计、可回滚、可对比的策略协议。

### 功能任务

1. 定义切片协议字段：
   - `split_strategy`
   - `split_version`
   - `split_stage`
   - `semantic_split_enabled`
   - `semantic_split_score`
   - `semantic_parent_section_id`
   - `embedding_build_strategy`
   - `context_version`
2. 扩展 `DocumentMetadata` 或统一 metadata helper，避免字段散落在不同调用点。
3. 在 `annotateSplitChunks` 后统一补齐策略字段，避免每个上游重复拼接。
4. 在 retrieval 层 `source` 中透传关键字段：
   - `split_strategy`
   - `embedding_build_strategy`
   - `context_version`
   - `parent_child_available`
5. 兼容旧数据：
   - 没有 `split_strategy` 的 chunk 标记为 `legacy_recursive`
   - 没有 `embedding_build_strategy` 的 chunk 标记为 `raw_content_embedding`

### 验收

1. 新入库数据具备完整切片策略字段。
2. 检索返回 `source` 能解释该结果由哪种切片与 embedding 文本生成。
3. 老数据不会因字段缺失导致检索或前端展示异常。
4. 策略字段能用于离线评测分组统计。

---

## 6.4 L3 `raw_content` / `embedding_content` 分离

### 目标

解决当前 `content` 同时承担“展示、引用、embedding”的问题，为轻量 Contextual Retrieval 打基础。

### 功能任务

1. 明确字段语义：
   - `raw_content`：原始 chunk 文本，用于展示、引用、审计、citation。
   - `embedding_content`：上下文化后的检索文本，用于 embedding 与可选 BM25。
   - `content`：兼容字段，短期仍返回 `raw_content`，避免 API 破坏。
2. 设计最小兼容方案：
   - 如果暂时不改 Milvus schema，则写入前用 `embedding_content` 调 embedding，但 Milvus `content` 仍存 `raw_content`。
   - 如果改 Milvus schema，则新增 `embedding_content` 字段，并准备 collection version 与重建计划。
3. 优先推荐低风险方案：
   - 在 indexer 前构造 embedding 输入文本。
   - 存储仍保留 `doc.Content = raw_content`。
   - 在 metadata 中存 `embedding_content_hash`、`embedding_build_strategy`，不强制存完整 `embedding_content`。
4. 如果需要调试，可通过配置控制是否保存完整 `embedding_content`：
   - `SaveEmbeddingContentForDebug=false`
   - `EmbeddingContentMaxLength`
5. 调整 embedding 调用点：
   - 当前 `IndexerService.Store` 直接把 `docs` 交给 Eino indexer。
   - 需要在进入 indexer 前准备一份 embedding 专用 docs，或封装自定义 embedding converter。

### 验收

1. 检索结果展示仍是原始 chunk，不出现 `[Document]`、`[Section]` 等上下文化前缀污染。
2. embedding 实际使用的文本可追踪到构建策略。
3. 不改 schema 的方案可以平滑上线；改 schema 的方案必须通过 index lifecycle 或新 collection 灰度。
4. 出现问题时可关闭上下文化 embedding，回退到 `raw_content_embedding`。

---

## 6.5 L4 轻量 Contextual Retrieval 入库增强

### 目标

让 child chunk 在向量召回阶段更自解释，特别提升“离开标题就看不懂”的短 chunk 召回能力。

### 功能任务

1. 新增上下文化文本构建器，例如 `contextualizer`：
   - 输入 `schema.Document` chunk 与 metadata
   - 输出 `embedding_content`
   - 输出 `context_summary` 或 `context_prefix`
2. 建议默认模板：

```text
[Document]: {title 或 file_name}
[Section]: {hierarchy_path 或 section_title}
[Chunk]:
{raw_content}
```

3. 控制模板长度：
   - 没有标题时不生成空前缀
   - `hierarchy_path` 过长时截断
   - `embedding_content` 超过上限时保留 `raw_content` 与最近章节路径
4. 新增配置：

```yaml
DocumentSplitter:
  ContextualEmbeddingEnabled: true
  ContextualEmbeddingStrategy: "title_section_v1"
  ContextualEmbeddingMaxPrefixChars: 400
  ContextualEmbeddingMaxContentChars: 3000
  SaveEmbeddingContentForDebug: false
```

5. metadata 补齐：
   - `embedding_build_strategy=title_section_v1`
   - `context_version=chunk_context_v1`
   - `context_prefix_hash`
   - `embedding_content_hash`
6. BM25 / sparse 路线兼容：
   - 第一阶段 dense route 使用 `embedding_content`
   - sparse route 是否使用上下文化文本单独评测，不默认绑定

### 验收

1. 标题依赖型问题的 Recall@K 有提升。
2. 普通短文档没有明显误召回增加。
3. 平均 embedding 文本长度增长在阈值内。
4. 关闭 `ContextualEmbeddingEnabled` 后可回到原始 content embedding。

---

## 6.6 L5 Parent-Child Retrieval 灰度启用与回填评估

### 目标

利用现有 Parent-Child 实现，让 child 精确召回后能补齐同章节或邻近窗口上下文，提高答案完整性与引用解释性。

### 功能任务

1. 灰度打开配置：
   - `rag.feature_flags.enable_parent_child_retrieval=true`
   - `rag.phase3.parent_child_fill_strategy=section_window`
   - `rag.phase3.parent_child_window_size=1`
   - `rag.phase3.parent_child_max_tokens=1200`
2. 建立策略对照：
   - `child_only`
   - `sibling_window`
   - `section_window`
   - `child_first_with_parent_summary`
3. 扩展日志字段：
   - `parent_child_enabled`
   - `parent_fill_strategy`
   - `parent_fill_count`
   - `parent_fill_tokens`
   - `parent_fill_reason`
4. 扩展调试视图：
   - 回填前 child content
   - 回填后 filled content
   - parent / sibling 来源
   - token budget 消耗
5. 降级策略：
   - parent 查询失败回退 child-only
   - parent metadata 缺失回退 child-only
   - 回填超预算保留原 child

### 验收

1. 长文档问题答案完整性提升。
2. P95 检索延迟退化在可接受范围内。
3. parent 回填没有显著引入不相关上下文。
4. 任意时刻关闭 `enable_parent_child_retrieval` 可恢复旧路径。

---

## 6.7 L6 结构块内语义二次切分

### 目标

只对超长结构块做语义边界优化，减少一个 chunk 混入多个主题的问题。第一阶段使用 embedding-based semantic split，不默认使用 LLM 判断边界。

### 功能任务

1. 新增配置：

```yaml
DocumentSplitter:
  SemanticSecondarySplitEnabled: false
  SemanticMinBlockSize: 1200
  SemanticTargetChunkSize: 1000
  SemanticMaxChunkSize: 1400
  SemanticBreakpointPercentile: 20
  SemanticMinSentencesPerChunk: 2
```

2. 实现触发条件：
   - 仅对结构切片后的超长 block 启用
   - 普通短 chunk 不走语义二次切分
   - PDF/TXT 弱结构文本可按段落窗口进入候选
3. 实现基础流程：
   - 句子切分
   - 相邻句 embedding 相似度计算
   - 找低相似度候选断点
   - 在 `target/max chunk size` 约束下合并句子
4. metadata 记录：
   - `semantic_split_enabled`
   - `semantic_split_score`
   - `semantic_breakpoint_method=embedding_similarity_v1`
   - `semantic_parent_section_id`
5. 成本控制：
   - 对同一文档句向量批量 embedding
   - 超过最大句数时回退 recursive
   - embedding 服务失败时回退结构切片
6. 评测重点：
   - Chunk Purity
   - Recall@K
   - Chunks Per Document
   - Ingest P95
   - Embedding Cost

### 验收

1. 多主题长段落的 chunk purity 提升。
2. 入库 P95 和 embedding 成本增长可控。
3. 语义二次切分失败时不阻断入库。
4. 默认可先关闭，仅在指定知识库或实验 profile 中开启。

---

## 6.8 L7 可观测性、调试视图与回归门禁

### 目标

让切片升级可解释、可比较、可回滚。所有新增策略必须能在日志、评测与调试页面里被看见。

### 功能任务

1. 检索日志扩展：
   - `split_strategy`
   - `embedding_build_strategy`
   - `context_version`
   - `semantic_split_enabled`
   - `parent_fill_strategy`
   - `parent_fill_tokens`
2. 入库日志扩展：
   - `chunk_count`
   - `avg_chunk_chars`
   - `p95_chunk_chars`
   - `avg_embedding_chars`
   - `semantic_resplit_count`
   - `markdown_structure_chunk_count`
3. 调试视图扩展：
   - 原始 chunk
   - embedding context 前缀
   - section path
   - parent-child 回填前后
   - strategy metadata
4. 评测脚本扩展：
   - profile 按切片策略分组
   - 输出 markdown/json 报告
   - 对比 baseline 与 candidate
5. 门禁规则：
   - Recall@K 不得低于 baseline
   - Citation Precision 不得低于 baseline
   - Ingest P95 不得超过阈值
   - 平均 chunk 数不得异常膨胀
   - 平均 embedding 文本长度不得超过预算

### 验收

1. 任意检索结果能追溯切片策略与上下文化策略。
2. 任意策略上线前有 baseline 对比报告。
3. 门禁失败时能明确建议关闭哪个策略。
4. 调试视图能帮助定位“切片问题”而不是只看到最终召回结果。

---

## 6.9 L8 Agentic Chunking 实验层与后续高级能力预留

### 目标

在主链路稳定后，为 Agentic Chunking、Propositions、Late Chunking 预留实验入口，但不让它们影响默认入库稳定性。

### 功能任务

1. 定义 Agentic Chunking 适用条件：
   - 高价值知识库
   - 文档量小
   - 文档结构复杂
   - 允许较高入库延迟和成本
   - 有人工评测或抽检流程
2. 新增实验开关：

```yaml
DocumentSplitter:
  AgenticChunkingEnabled: false
  AgenticChunkingMode: "shadow"
  AgenticChunkingMaxDocumentChars: 30000
  AgenticChunkingAllowedKBIDs: []
```

3. Agentic 输出必须结构化：
   - chunk boundaries
   - chunk title
   - chunk summary
   - parent-child relation
   - tags
   - confidence
4. 实验模式优先级：
   - `shadow`：只生成候选切片，不写入主索引
   - `ab`：指定知识库小流量双索引对比
   - `manual_review`：人工审核后写入
5. 后续 Propositions 预留：
   - 作为补充索引层
   - 命中 proposition 后回源到 raw chunk / parent context
6. 后续 Late Chunking 预留：
   - 等 embedding 模型支持长上下文 token-level 表示后再评估
   - 不在当前 Milvus indexer 链路强行实现

### 验收

1. Agentic Chunking 默认关闭。
2. Agentic 实验不会写坏主索引。
3. Agentic 结果能与结构切片结果做离线对比。
4. 没有评测收益时不得进入主链路。

---

## 7. 推荐实施节奏

## 7.1 第一批任务

1. 修复 `md/markdown` 上传主链路走 `SplitMarkdownDocument`。
2. 增加 `split_strategy` 与 `split_version` metadata。
3. 扩展切片专项评测集与 baseline profile。
4. 灰度打开 Parent-Child Retrieval，并记录回填收益。

## 7.2 第二批任务

1. 实现 `raw_content` 与 `embedding_content` 构建分离。
2. 增加 `ContextualEmbeddingEnabled` 配置。
3. 让 dense embedding 使用上下文化文本，展示仍使用原文。
4. 补齐日志、调试视图与回归门禁。

## 7.3 第三批任务

1. 实现 embedding-based semantic secondary split。
2. 对超长结构块灰度启用。
3. 评估 chunk purity、召回、成本与入库延迟。
4. 根据评测决定是否扩大启用范围。

## 7.4 第四批任务

1. 建立 Agentic Chunking shadow 实验。
2. 只对指定高价值知识库运行。
3. 与默认结构切片结果离线对比。
4. 决定是否进入后续 AB 或人工审核写入。

---

## 8. 角色分工建议

1. 后端 A：L1 + L2，负责主链路切片策略选择、metadata 协议与兼容。
2. 后端 B：L3 + L4，负责 `embedding_content` 构建、indexer 适配与上下文化 embedding。
3. 后端 C：L5 + L7，负责 Parent-Child 灰度、日志、调试视图与回归门禁。
4. 算法/检索：L6 + L8，负责 semantic split、Agentic shadow、评测指标设计。
5. QA/SRE：L0 + 灰度回滚，负责 baseline、评测报告、延迟成本监控与回滚演练。

---

## 9. 回滚与降级策略

按风险从高到低关闭：

1. 关闭 `AgenticChunkingEnabled`。
2. 关闭 `SemanticSecondarySplitEnabled`。
3. 关闭 `ContextualEmbeddingEnabled`。
4. 关闭 `enable_parent_child_retrieval`。
5. 将 Markdown 主链路临时回退到通用 `Split`。
6. 回退到当前 `recursive splitter + raw content embedding` 基线。

回滚要求：

1. 每个策略必须有独立开关。
2. 每次打开策略前必须记录 baseline。
3. 任何策略导致检索 P95、入库失败率、空召回率、引用准确率异常时，优先关闭该策略而不是回滚整套系统。
4. 涉及 Milvus schema 或 collection 变更时，必须使用新 collection 或 index lifecycle 灰度，不直接原地破坏旧 collection。

---

## 10. 阶段验收模板

执行完成后按以下模板填写：

1. 功能完成情况：
   - L0
   - L1
   - L2
   - L3
   - L4
   - L5
   - L6
   - L7
   - L8
2. 配置快照：
   - `DocumentSplitter`
   - `rag.feature_flags`
   - `rag.phase3`
   - `Milvus`
3. 切片指标：
   - chunk count
   - avg chunk chars
   - p95 chunk chars
   - chunks per document
   - semantic resplit count
   - contextual embedding text avg/p95
4. 检索指标：
   - Recall@K
   - MRR
   - nDCG
   - Citation Precision
   - Parent Fill Gain
   - Contextual Recall Gain
5. 成本与稳定性：
   - Ingest P95
   - Retrieve P95
   - embedding token/字符增长
   - 入库失败率
   - 回填失败率
6. 风险案例：
   - 误召回案例
   - 上下文化噪声案例
   - 语义切分过碎案例
   - parent 回填污染案例
7. 灰度结论：
   - 是否扩大开启 Parent-Child
   - 是否默认开启 Contextual Embedding
   - 是否继续 semantic split 灰度
   - 是否进入 Agentic shadow
8. 回滚演练结果：
   - 成功/失败
   - 用时
   - 残留问题
9. 下一阶段入口：
   - Propositions
   - Agentic AB
   - Late Chunking 评估
   - HTML/PDF/OCR 结构解析

---

## 11. 文档维护规则

1. 任何切片策略新增字段，必须同步更新本文档 L2、L7 与验收模板。
2. 任何影响 embedding 输入文本的变更，必须说明 `raw_content` 与 `embedding_content` 的边界。
3. 任何 Agentic Chunking 相关实现，必须默认关闭并绑定评测报告。
4. 任何 Milvus schema 或 collection 变更，必须补充回滚路径。
5. 后续实现教程可以基于本文档按 L1、L3、L6 分别拆成独立实现文档。

