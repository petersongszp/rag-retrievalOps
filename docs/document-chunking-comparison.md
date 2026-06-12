# 文档切片方式对比分析

## 1. 背景与当前项目现状

当前项目已经具备一套较完整的基础切片体系，核心实现位于：

- `backend/internal/milvus/splitter/splitter.go`
- `backend/internal/milvus/splitter/markdown.go`
- `backend/internal/milvus/splitter/parent_child.go`
- `backend/config.yaml`

当前方案可概括为三层：

1. 通用切片：基于 `eino-ext` 的 `recursive splitter`
2. Markdown 增强：按 Markdown 标题、段落、代码块、列表等结构优先切分
3. Parent-Child 回填：检索命中 child，小范围补充 parent 上下文

当前配置与策略：

- 通用 `ChunkSize = 1000`
- 通用 `OverlapSize = 200`
- 通用分隔符：段落、换行、中英文句号/感叹号/问号
- Markdown 分隔优先级：多个空行 -> `H2` -> `H3` -> `H4` -> `H5` -> `H6` -> `H1` -> 段落 -> 代码块 -> 水平线 -> 列表 -> 换行 -> 句号
- Parent-Child 策略：`heading_section`、`heading_section_window`、`paragraph_window`
- 已有元数据：`chunk_id`、`parent_id`、`section_title`、`hierarchy_path`、`parent_start_offset`、`parent_end_offset`

结论先行：当前项目已经实现了“递归切片 + 文档结构感知 + Parent-Child 补全”的较优工程基线，但尚未进入“语义边界感知”“索引时上下文化”“命题级检索”“长上下文嵌入延迟切片”这一层。

---

## 2. 主流切片方式总览对比表

| 方式 | 原理简述 | 优点 | 缺点 | 适用场景 | 实现复杂度 | 当前项目是否已实现 |
| --- | --- | --- | --- | --- | --- | --- |
| 固定大小切片（Fixed-size Chunking） | 按固定字符数或 token 数切分，通常配合 overlap | 实现最简单、吞吐高、稳定 | 容易截断语义、标题与正文分离、噪声大 | 早期 PoC、日志流、格式混乱文本 | 低 | 否（未独立实现） |
| 递归字符切片（Recursive Character Splitting） | 按分隔符优先级递归切分，尽量在较自然边界断开 | 比固定切片更稳，工程成本低，效果通常明显更好 | 仍然是“边界启发式”，不真正理解语义 | 通用文档、知识库基线方案 | 低 | 是 |
| 语义切片（Semantic Chunking） | 基于句向量相似度、相邻句语义断点或 LLM 判断边界 | 更接近语义单元，减少“半段话”与跨主题混块 | 计算成本更高，阈值难调，离线构建更慢 | 长段落、无明确标题、FAQ、说明文、政策文档 | 中高 | 否 |
| 文档结构切片（Structure-aware） | 利用 Markdown/HTML/PDF 的标题、段落、表格、列表、代码块结构切分 | 保留章节语义与层级，可解释性强 | 对解析质量依赖高；PDF/扫描件困难 | 手册、制度文档、技术文档、网页文档 | 中 | 部分实现（Markdown 已实现） |
| Agentic Chunking | 用 LLM 阅读文档后决定边界、摘要、标签、父子关系 | 灵活，能处理复杂非标准结构 | 成本高、延迟高、稳定性与可重复性较差 | 高价值低吞吐知识库、法律/投研/医疗等复杂文档 | 高 | 否 |
| Late Chunking | 先用长上下文 embedding 模型编码整段文本，再对 token 表示做后置池化得到 chunk embedding | chunk embedding 带全局上下文，长文代词与省略指代更稳 | 依赖长上下文 embedding 和 token 级输出；工程门槛高 | 长文档、章节间依赖强的材料 | 高 | 否 |
| Contextual Retrieval | 在索引时给 chunk 补充“文档上下文说明”后再做 embedding/BM25 | 显著改善孤立 chunk 的可检索性 | 增加离线构建成本、存储成本与索引链路复杂度 | 监管、财报、制度、手册等上下文依赖强文档 | 中高 | 否（当前 Parent-Child 不等于它） |
| Propositions（命题切片） | 将文档抽取为原子事实/命题，按命题建立索引，再回溯原文 | 事实召回精度高，适合细粒度问答与多跳 | 抽取成本高，命题与原文映射维护复杂 | QA、事实核验、复杂检索、多跳问题 | 高 | 否 |

---

## 3. 各类切片方式详细分析

## 3.1 固定大小切片（Fixed-size Chunking）

**原理简述**

按固定字符数或 token 数直接切分，例如每 500/1000 token 一个块，并保留一定 overlap。

**优点**

- 最容易实现，几乎不依赖文档结构
- 构建速度快，适合大规模批量导入
- 行为稳定，调试简单

**缺点**

- 容易把一句话、一个表格或一段逻辑切断
- 标题和正文容易分离，导致检索块缺乏自解释性
- overlap 只能缓解，不能从根本上解决语义截断

**适用场景**

- 冷启动阶段
- OCR 后的脏文本
- 日志、聊天记录、流水文本

**实现复杂度**

- 低

**当前项目是否已实现**

- 未独立实现
- 当前项目的 recursive splitter 在极端情况下会退化为近似固定大小切片，但这不是纯 fixed-size 策略

## 3.2 递归字符切片（Recursive Character Splitting）

**原理简述**

按一组有优先级的分隔符递归切分，优先尝试较“自然”的边界，如段落、换行、句号；如果仍超长，再继续向更细粒度分隔符递归。

**优点**

- 明显优于纯 fixed-size
- 不需要复杂模型，构建成本低
- 可根据语言与文档类型自定义分隔符
- 很适合作为工程默认基线

**缺点**

- 本质仍是启发式分隔，不理解主题是否真的切换
- 面对超长段落、无标点文本、表格/代码混排时效果有限
- 不能解决 chunk 自身缺乏上下文标签的问题

**适用场景**

- 大多数通用知识库文档
- 中英文混合文本
- 需要低成本稳定上线的 RAG 系统

**实现复杂度**

- 低

**当前项目是否已实现**

- 已实现
- 当前项目使用 `eino-ext` 的 recursive splitter，默认分隔符与 `ChunkSize/OverlapSize` 已配置完成

## 3.3 语义切片（Semantic Chunking）

**原理简述**

先把文本切到句子或小句粒度，再计算相邻句之间的 embedding 相似度，或让 LLM 判断“主题是否发生变化”，在相似度低谷处作为断点；最终再合并成目标 chunk。

**优点**

- 更贴近真实语义边界
- 能减少“一个 chunk 同时包含多个主题”的情况
- 对无明显标题但内部主题切换频繁的文档很有效

**缺点**

- 离线构建成本高于 recursive
- 阈值选择敏感，不同语料差异大
- 如果句向量质量一般，断点可能不稳定
- LLM 判断边界时成本和可重复性问题更明显

**适用场景**

- FAQ
- 产品说明
- 政策制度
- 多段解释性文本
- 没有稳定标题结构的知识文档

**实现复杂度**

- 中高

**当前项目是否已实现**

- 未实现

## 3.4 文档结构切片（Document Structure-aware）

**原理简述**

不先把文档当成纯字符串，而是优先识别结构单元，例如 Markdown 标题、HTML DOM、PDF 段落/表格/页块，然后按结构切分；必要时再对过长结构块做二次切片。

**优点**

- 最符合“文档本来怎么组织”的语义
- 非常利于可解释性、引用展示和 parent-child 组织
- 对手册、制度、技术文档效果通常很好

**缺点**

- 结构解析质量直接决定上限
- PDF、扫描件、复杂表格常常比较难
- 仅靠结构不一定能处理同一节内部的语义跳跃

**适用场景**

- Markdown 手册
- HTML 帮助中心
- Wiki
- 规范文档
- 有清晰标题层级的长文

**实现复杂度**

- 中

**当前项目是否已实现**

- 部分实现
- Markdown 场景已实现较强的结构感知切片
- HTML/PDF 尚未形成统一的结构切片框架

## 3.5 Agentic Chunking（LLM 驱动切片）

**原理简述**

让 LLM 在导入时读取文档，主动决定：

- 应该在哪些地方切块
- 每块的主题是什么
- 哪些块应该建立父子关系
- 是否要生成摘要、标签、关键词、命题

这类方案没有完全统一标准，核心思想是“用模型替代固定规则决定切片策略”。

**优点**

- 对复杂非标准文档适应性强
- 可以顺带产出摘要、标签、知识图谱边
- 对高价值文档库有很强上限

**缺点**

- 成本高
- 导入慢
- 输出边界不够稳定，复现性不如规则法
- 需要评估与回滚机制，否则索引质量波动较大

**适用场景**

- 法律合同
- 投研报告
- 医疗文档
- 高价值但文档量可控的企业知识库

**实现复杂度**

- 高

**当前项目是否已实现**

- 未实现

## 3.6 Late Chunking（延迟切片）

**原理简述**

传统做法是先切片，再分别对每个 chunk 做 embedding；Late Chunking 则相反，先把尽可能长的整段文本送入长上下文 embedding 模型，拿到 token 级表示后，再按边界对 token 表示做池化，得到 chunk embedding。这样每个 chunk 的向量都带有更完整的全局上下文。

**优点**

- 对长文中的代词、省略、前文指代有明显改善
- 比传统 overlap 更“语义化”地保留邻近上下文
- 对长文档 retrieval 很有潜力

**缺点**

- 强依赖长上下文 embedding 模型
- 需要拿到 token 级隐藏表示或等价能力
- 实现链路与常规 embedding API 差异较大
- 成本、工程复杂度、部署复杂度都更高

**适用场景**

- 长章节文档
- 上下文依赖强的说明类材料
- 需要提升长文检索质量，且能自建或接入高级 embedding 能力的系统

**实现复杂度**

- 高

**当前项目是否已实现**

- 未实现

## 3.7 Contextual Retrieval（上下文检索）

**原理简述**

Anthropic 提出的核心做法不是改变 chunk 边界本身，而是在索引阶段为每个 chunk 补充“它在整篇文档中的上下文说明”，再基于这个“上下文化 chunk”做 embedding 和 BM25。典型效果是让本来孤立的一段话变得可自解释。

例如，原始 chunk 只写“同比增长 3%”，而上下文化后会变成“这是某公司某季度营收描述，所属财报章节为……，该段在讨论……，原文为：同比增长 3%”。

**优点**

- 对“孤立 chunk 不自解释”问题非常有效
- 与现有向量检索体系兼容性较好
- 尤其适合章节内部引用、代词、省略主语、财报/制度类文本

**缺点**

- 索引阶段需要额外摘要或上下文化生成
- 增加 embedding 文本长度、存储成本与构建时延
- 如果上下文化文本质量不好，会引入噪声

**适用场景**

- 财报
- 制度
- 政策
- 企业手册
- 任意“标题依赖强、段落脱离上下文后不自解释”的文档

**实现复杂度**

- 中高

**当前项目是否已实现**

- 未实现
- 需要特别说明：当前项目的 Parent-Child 检索补全是“命中后补父上下文”，而 Contextual Retrieval 是“索引前给 chunk 加上下文说明再建索引”，两者不是同一个方案

## 3.8 Propositions（命题切片）

**原理简述**

把文档转换为更细粒度的“原子事实”或“命题”，例如：

- “Redis 是内存数据库”
- “Redis 支持持久化”
- “A 产品在 2025 年支持多租户”

检索时先召回命题，再回溯命题所属原文 chunk 或父段落用于生成。

**优点**

- 事实级召回能力强
- 对细粒度问答、属性查询、复杂多跳问题更友好
- 更容易做证据核验与引用一致性检查

**缺点**

- 命题抽取成本高
- 命题与原文之间的映射要维护好
- 过度原子化后，可能损失叙述语境
- 更适合作为“补充索引层”，不适合作为唯一索引层

**适用场景**

- 事实问答
- 多跳问答
- 参数/属性型检索
- 高精度证据检索

**实现复杂度**

- 高

**当前项目是否已实现**

- 未实现

---

## 4. 当前项目切片方式的优势

### 4.1 已有方案的优势

**1. 基线合理，工程上成熟**

`recursive splitter` 是非常常见且稳妥的默认方案。相较 fixed-size，它已经显著减少了粗暴截断问题。

**2. Markdown 结构感知做得已经不错**

当前 Markdown 专用分隔符优先级明显强于通用递归切片，能够较好保留标题层级、段落与代码块边界。

**3. Parent-Child 设计方向正确**

当前系统不是只做“切片”，而是进一步做了“命中子块后补父上下文”，这对最终生成质量、引用可解释性、前端展示都很重要。

**4. 元数据设计较完整**

已有：

- `chunk_id`
- `parent_id`
- `section_title`
- `hierarchy_path`
- `parent_start_offset`
- `parent_end_offset`

这使得后续引入更高级切片策略时，不需要推翻现有检索协议。

**5. 对结构化 Markdown 文档已经具备较高性价比**

对于企业内部文档、技术手册、接入文档，这套方案已经能覆盖相当大一部分主流需求。

---

## 5. 当前项目切片方式的不足

### 5.1 主要不足

**1. 仍以分隔符启发式为主，缺少真正的语义边界判断**

当前 chunk 是否切开，主要由换行、标题、句号等边界决定；如果一段内部主题已切换但表面上没有明显分隔符，系统无法识别。

**2. 结构感知目前主要集中在 Markdown**

HTML、富文本、PDF、扫描件、表格型文档尚未形成统一结构切片策略，跨格式一致性不足。

**3. Child chunk 的“自解释性”仍然不够**

虽然检索后会补 parent，但向量召回阶段仍然是用 child 自身文本去匹配。对于“本段必须依赖上文标题才看得懂”的内容，初始召回仍可能偏弱。

**4. Overlap 是通用补丁，不是根本解决方案**

`OverlapSize=200` 能缓解边界断裂，但会增加索引冗余，也不能解决全局语义归属问题。

**5. 尚未针对细粒度事实问答建立更细索引层**

如果后续问答逐渐偏向“属性值、条件、时间点、产品能力差异、多跳证据”，单纯 chunk 级索引会逐渐碰到上限。

---

## 6. 优先级最高的升级建议

## 建议：在现有方案上优先升级为“结构优先 + 语义二次切分 + 轻量 Contextual Retrieval”

这是当前项目性价比最高的方向，原因如下：

- 不需要推翻现有 recursive / Markdown / Parent-Child 架构
- 能同时解决“边界不够语义化”和“child 不够自解释”两个核心问题
- 对现有 metadata、检索补全、前端展示兼容性最好
- 风险远小于直接上 Agentic Chunking 或 Late Chunking

### 为什么不是优先做 Late Chunking

因为 Late Chunking 依赖长上下文 embedding 与 token 级表示能力，当前项目并没有体现出这类 embedding 基础设施，短期落地成本偏高。

### 为什么不是优先做 Propositions

因为命题索引更适合作为第二索引层。若当前基础 chunk 召回还没做到足够稳，直接上 proposition 会把系统复杂度明显抬高。

### 为什么不是优先做 Agentic Chunking

因为它适合高价值低吞吐库，但对当前项目这种通用知识库平台来说，先做“可控、可评估、可批量”的升级更合适。

---

## 7. 具体实现方案

## 7.1 目标架构

将当前切片升级为四阶段：

1. 文档结构解析
2. 结构块内语义二次切分
3. 为 child 生成轻量上下文化文本
4. 保留现有 Parent-Child 检索补全

即：

`Structure-aware Split -> Semantic Re-split -> Contextualized Embedding Text -> Parent-Child Retrieval`

## 7.2 详细落地方案

### 阶段一：保留现有结构切片作为第一层边界

沿用现有：

- Markdown 标题层级切片
- 段落边界
- 代码块/列表/水平线等特殊结构

这一步不需要推翻已有实现，只需要把它显式定义为“一级切片”。

### 阶段二：只对“过长结构块”做语义二次切分

对超过阈值的结构块，不再只依赖句号和换行继续切，而是：

1. 先按句子切分
2. 计算相邻句 embedding 相似度
3. 识别相似度低谷作为候选断点
4. 在满足 `ChunkSize` 上下限约束的前提下合并成最终 child chunk

建议策略：

- 仅对超过 `800~1200` 字符或超过指定 token 的结构块启用
- 先做 embedding-based semantic split，不急着上 LLM-based split
- LLM 仅作为后续实验开关，不作为默认主链路

建议新增配置：

```yaml
DocumentSplitter:
  SemanticSecondarySplitEnabled: true
  SemanticMinBlockSize: 800
  SemanticTargetChunkSize: 1000
  SemanticMaxChunkSize: 1400
  SemanticBreakpointPercentile: 20
  SemanticMinSentencesPerChunk: 2
```

### 阶段三：为 embedding 单独生成“上下文化文本”

新增一个与 `chunk.Content` 分离的索引字段，例如：

- `raw_content`：原始 chunk 文本，用于展示、引用、审计
- `embedding_content`：用于向量化的上下文化文本

`embedding_content` 建议由以下信息拼接组成：

1. `document title`
2. `hierarchy_path`
3. `section_title`
4. 可选的父块摘要或节摘要
5. 原始 chunk 正文

示意格式：

```text
[Document]: RAG 平台接入指南
[Section]: 接入流程 > SDK 初始化 > 鉴权配置
[Context]: 本节介绍 SDK 初始化时的鉴权参数与环境变量要求。
[Chunk]:
......
```

注意：

- 展示给用户的仍然是 `raw_content`
- 只在 embedding / BM25 索引层使用 `embedding_content`
- 这本质上是轻量版 Contextual Retrieval

建议新增元数据：

- `embedding_build_strategy`
- `context_summary`
- `context_version`
- `semantic_split_score`
- `semantic_parent_section_id`

### 阶段四：继续沿用 Parent-Child 检索补全

现有 Parent-Child 机制不但要保留，反而会在升级后更有价值：

- child 负责精确命中
- contextualized embedding 提高 child 被召回概率
- parent 负责生成阶段补足上下文

这三者组合起来，比单独升级任意一个点都更稳。

---

## 8. 推荐实施顺序

### P0：低风险增强

1. 抽象“一级结构切片”和“二级切片”接口
2. 把当前 Markdown 逻辑纳入统一结构切片框架
3. 增加 `split_strategy`、`embedding_build_strategy` 等元数据

### P1：最高优先级功能

1. 为超长结构块增加 embedding-based 语义二次切分
2. 为 chunk 新增 `embedding_content`
3. 向量索引使用 `embedding_content`，展示与引用仍使用 `raw_content`

### P2：扩展格式支持

1. HTML 标题/段落/列表结构切片
2. PDF 段落/页块/表格结构切片
3. OCR 文本清洗后再进入统一切片链路

### P3：高级实验能力

1. 命题索引层（Propositions）
2. LLM 辅助 Agentic Chunking
3. 长上下文 embedding 场景下的 Late Chunking 评估

---

## 9. 预期收益

如果按上述路线升级，预期收益主要在四个方面：

**1. 提高初始召回率**

尤其是“离开标题就难以理解”的 chunk，会更容易被向量检索命中。

**2. 提高召回块纯度**

语义二次切分后，一个 chunk 混入多个主题的概率会下降。

**3. 提高答案引用质量**

child 更精确，parent 继续补全，最终引用既细粒度又有上下文。

**4. 为后续 Proposition / Agentic / 图谱化检索打基础**

因为元数据和分层切片框架一旦抽象好，后续升级不需要重做底座。

---

## 10. 最终判断

当前项目的切片体系，在工程实践里已经超过“只有 fixed-size 或基础 recursive”的水平，尤其是 Markdown 结构感知和 Parent-Child 设计是明显优势。

但如果目标是进一步提升：

- 长文召回稳定性
- 标题依赖型 chunk 的可检索性
- 复杂知识库问答的证据质量

那么下一步最值得投入的，不是直接追求最复杂的 Agentic Chunking，也不是立刻切到 Late Chunking，而是先完成：

**结构优先切片 + 语义二次切分 + 轻量 Contextual Retrieval**

这条路线与当前架构最兼容、投入产出比最高，也最容易做灰度实验与效果评估。

---

## 11. 参考资料

- LangChain `RecursiveCharacterTextSplitter` 源码：<https://raw.githubusercontent.com/langchain-ai/langchain/master/libs/text-splitters/langchain_text_splitters/character.py>
- Anthropic Contextual Retrieval：<https://www.anthropic.com/engineering/contextual-retrieval>
- Jina Late Chunking：<https://jina.ai/news/late-chunking-in-long-context-embedding-models/>
- Dense X Retrieval: What Retrieval Granularity Should We Use?：<https://arxiv.org/abs/2312.06648>
