# 后台管理平台检索实验室使用文档

## 背景

这篇文档用于帮助用户和开发学习者快速上手后台管理平台里的"检索实验室"功能。

检索实验室是知识库模块的延伸页面。知识库页面负责的是"文档怎么进来、入库任务怎么跑"，而检索实验室负责的是"文档进来之后，检索效果到底怎么样"。

它提供了一个无需写代码就能运行检索测试的界面，并且能把一次检索结果沉淀为评测样本，直接对接后续的自动化评测流程。

## 这篇文档会做什么

看完这篇文档，你应该能理解下面几件事：

1. 检索实验室在整个后台里承担什么角色。
2. 页面上的每个区域、按钮、字段分别是什么意思。
3. 一次检索请求从前端到后端经历了什么。
4. 检索结果中的 `request_id` 可以用来做什么。
5. 检索结果如何保存为评测样本，以及和评测模块的衔接关系。
6. Contract gap 机制是怎么工作的。
7. 当前实现里有哪些使用限制。

## 需要先理解的术语

### 知识库

知识库是一组被系统统一管理的文档集合。检索实验室的所有操作都基于知识库展开。每个知识库在创建时会关联一个向量集合（collection），检索就是在这些向量上做的。

### 检索（Retrieve）

检索是指把用户输入的问题（query）交给后端，后端在指定知识库的向量索引中搜索最相关的文档片段（chunk），并返回结果列表。返回的每条结果包含内容、相似度分数、出处信息等。

### Top K

Top K 是检索时的一个参数，表示"最多返回多少条结果"。当前页面提供 3、5、10 三个选项，默认是 5。后端硬上限是 20。

### request_id

每次检索请求都会由后端生成一个全局唯一的 `request_id`（UUID 格式）。它是连接检索实验室、审计日志、调试视图三个页面的核心纽带。

### Contract gap

Contract gap 是这个项目的一个内部概念。当前端期望某个字段有值、但后端接口实际没有返回时，页面不会静默跳过，而是会显示 `Contract gap` 来提醒你："这里前端预期有数据，但当前接口合同没有给到"。

它不是业务值，而是一种接口契约检查机制。

### 评测样本（Eval Case）

评测样本是评测模块的基本单位。每条样本包含一个 query、一组标准答案（relevant_ids）和其他元数据。检索实验室可以把一次检索的结果快速保存为评测样本草稿，减少手工录入工作。

### 调试视图（Debug View）

调试视图是基于 `request_id` 展开的详细检索链路追踪页面，可以看到 query 改写、路由命中、融合排序、rerank、过滤等每个阶段的中间结果。

## 整体流程

从用户进入检索实验室到完成一次检索测试，主流程是这样的：

1. 打开检索实验室页面。
2. 选择一个知识库。
3. 输入查询问题，选择 Top K。
4. 点击"运行检索测试"。
5. 后端生成 `request_id`，在知识库的向量索引中执行检索。
6. 页面展示检索结果列表，包括每条结果的内容、分数、出处信息。
7. 页面自动检查每条结果的契约字段完整性，如果有缺失会显示 Contract gap。
8. 用户可以复制 `request_id`、查看 Trace、查看调试视图。
9. 用户可以把当前检索结果保存为评测样本草稿。

这条链路体现了这个项目后台检索模块的亮点：它不是只做"搜一下看看结果"的简单测试工具，而是把检索测试、契约检查、链路追踪和评测沉淀衔接成了完整的运维闭环。

## 页面定位与项目亮点

检索实验室在整个后台里主要承担三个角色：

1. **检索测试入口**：不需要写代码或调用 curl，直接在页面上对任意知识库发起检索请求，查看返回结果。
2. **契约检查入口**：每次检索后自动检查后端返回的字段是否完整，帮助开发和运维发现接口退化。
3. **评测沉淀入口**：把一次检索结果快速保存为评测样本草稿，打通"手动测试 → 自动化评测"的链路。

对用户来说，检索实验室回答的是"我对这个知识库问一个问题，系统能找到什么、找不到什么"。

对开发和运维来说，检索实验室回答的是"检索接口当前返回了哪些字段、哪些字段缺失了、检索链路每个阶段的中间结果是什么"。

## 页面使用说明

### 页面入口

检索实验室的路由是：

1. `admin/src/app/(admin)/retrieval-lab/page.tsx`
2. `admin/src/components/admin/retrieval-lab-page.tsx`

在后台左侧导航里，对应菜单是"检索实验室"。

另外，知识库详情页顶部有一个"打开检索实验室"按钮，点击后会跳转到检索实验室，并自动带上当前知识库上下文。

### 调试视图入口

调试视图的路由是：

1. `admin/src/app/(admin)/retrieval-lab/debug/page.tsx`
2. `admin/src/components/admin/retrieval-debug-page.tsx`

调试视图不是独立使用的页面，它依赖检索实验室产生的 `request_id` 才能加载数据。入口在检索结果区域的"查看调试视图"按钮。

### 页面整体结构

检索实验室页面从上到下分为四个主要区域：

1. **标题区域**：页面标题和功能说明。
2. **检索表单区域**：选择知识库、输入查询、设置参数。
3. **检索结果区域**：显示 request_id、操作按钮、结果列表、契约缺口记录。
4. **保存评测样本弹窗**：把当前检索结果保存为评测样本草稿。

### 标题区域

页面顶部显示：

- 标题：`检索实验室`
- 说明：`运行检索测试、检查 request_id 与契约字段，并把一次检索沉淀为评测样本草稿。`

### 检索表单区域

检索表单是一张卡片，包含三个字段：

| 字段 | 控件类型 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| 知识库 | 下拉选择 | 是 | 当前选中的知识库 | 从知识库列表中选择要检索的目标。选项来自 `KnowledgeBaseProvider` 维护的知识库列表。 |
| 查询 | 多行文本框 | 是 | 无 | 输入要检索的问题。支持多行文本。 |
| Top K | 下拉选择 | 否 | 5 | 指定最多返回多少条结果。可选值：3、5、10。 |

表单底部有一个"运行检索测试"按钮，点击后发起检索请求。

如果当前已经通过知识库详情页进入检索实验室，"知识库"字段会自动填充为之前选中的知识库。

### 检索结果区域

检索请求成功后，页面会展示以下内容：

#### Request ID 卡片

页面顶部会显示一个独立卡片，包含：

- **Request ID 值**：以 `code` 样式展示后端返回的 UUID。如果后端没有返回 `request_id`，会显示 `Contract gap: request_id`。
- **复制 request_id 按钮**：把 `request_id` 复制到剪贴板。
- **查看 Trace 按钮**：跳转到 Trace 日志页面（`/trace-logs/retrieval?request_id=...`），查看该次检索的审计日志。
- **查看调试视图 按钮**：跳转到调试视图页面（`/retrieval-lab/debug?request_id=...`），查看该次检索的完整链路追踪。
- **保存为评测样本 按钮**：打开保存评测样本弹窗。

#### 结果列表

每条检索结果会显示为一张独立卡片，包含以下信息：

| 显示位置 | 字段 | 说明 |
| --- | --- | --- |
| 卡片头部左侧 | 结果序号 | 蓝色标签，如"结果 1"、"结果 2" |
| 卡片头部右侧 | Score | 相似度分数。如果没有值，显示 `Contract gap` |
| 内容区域 | Content | 检索命中的文档片段内容 |
| 标签区域 | 文件 | 来源文件名。来自 `citation.file_name` |
| 标签区域 | Chunk Index | 文档切片序号。来自 `citation.chunk_index` |
| 标签区域 | Chunk ID | 文档切片唯一标识。来自 `citation.chunk_id` |
| 标签区域 | Route | 检索路由。如 `dense`、`sparse`、`hybrid` |
| 标签区域 | Collection | 向量集合名称。来自 `source.collection` |
| 标签区域 | Retriever | 检索器版本。来自 `source.retriever_version` |

如果某条结果有任何 Contract gap，卡片底部会显示一条警告，列出具体缺失了哪些字段。

#### 空结果

如果检索没有返回任何结果，页面会显示一个空状态提示："未返回检索结果。"。

#### 契约缺口记录

如果检索结果中存在任何 Contract gap，页面底部会显示一个可折叠的"契约缺口记录"面板。展开后是一个表格，列出所有缺失字段的详情：

| 列名 | 说明 |
| --- | --- |
| 缺失字段 | 字段路径，如 `score`、`citation.file_name` |
| 所属接口 | 该字段所属的 API 接口路径 |
| 影响页面 | 受影响的前端页面名称 |
| 阻塞验收 | 是否阻塞验收。`是` 表示该字段缺失会影响核心功能，`否` 表示仅影响展示 |

这个机制的价值在于：不需要人工逐字段检查后端返回，页面会自动告诉你哪些预期字段缺失了。

### 保存评测样本弹窗

点击"保存为评测样本"按钮后，会打开一个弹窗，用于把当前检索结果沉淀为评测样本草稿。

弹窗包含以下字段：

| 字段 | 控件类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| 评测集 | 下拉选择（支持搜索） | 是 | 选择要保存到哪个评测集。选项来自 `/admin/kb/eval/datasets` 接口。如果还没有评测集，会提示用户先去创建。 |
| Case Key | 文本输入 | 是 | 样本的唯一标识。自动生成，格式为 `retrieval-lab-YYYYMMDDHHmmss`。 |
| Top K | 数字输入 | 是 | 默认取当前检索的 Top K 值。 |
| Query | 多行文本框 | 是 | 默认取当前检索的查询内容。 |
| Query Type | 文本输入 | 否 | 查询类型标签，如 `factual`、`multi-hop`。 |
| Collection | 文本输入 | 否 | 向量集合名称。默认取当前检索结果中第一条的 `source.collection`。 |
| Tags | 多行文本框 | 否 | 标签列表，逗号或换行分隔。 |
| Relevant IDs 草稿 | 多行文本框 | 否 | 标准答案的 chunk_id 列表。可以点击"复制当前结果 chunk_id 到 relevant_ids 草稿"按钮自动填充。 |
| Notes | 多行文本框 | 否 | 备注说明。 |

弹窗底部有一个辅助按钮："复制当前结果 chunk_id 到 relevant_ids 草稿"。点击后会把当前检索结果中所有不重复的 `chunk_id` 写入 Relevant IDs 草稿区。

保存成功后，页面会提示："样本已保存，请前往评测集页面补齐标准答案并执行校验。"

这里有一个很重要的设计意图：保存的是**草稿**，不是最终样本。用户还需要去评测集页面确认 relevant_ids 是否准确、补齐标准答案，然后执行校验才能进入评测流程。

## 字段说明

### 检索请求字段映射

| 页面字段 | API 请求字段 | 后端结构体字段 | 含义 | 取值 / 说明 |
| --- | --- | --- | --- | --- |
| 知识库 | `kb_id` | `retrieveRequest.KBID` | 目标知识库 ID | 知识库主键，`uint64` |
| 查询 | `query` | `retrieveRequest.Query` | 检索查询文本 | 非空字符串，前后端均校验 |
| Top K | `top_k` | `retrieveRequest.TopK` | 最大返回结果数 | 前端可选 3/5/10，后端硬上限 20 |

### 检索响应字段映射

| 响应字段 | 后端结构体 | 类型 | 含义 |
| --- | --- | --- | --- |
| `request_id` | `retrieveResponse.RequestID` | `string` | 本次请求的唯一标识（UUID），用于链路追踪和调试 |
| `items` | `retrieveResponse.Items` | `[]retrieveItem` | 检索结果列表 |
| `items[].content` | `retrieveItem.Content` | `string` | 检索命中的文档片段内容 |
| `items[].score` | `retrieveItem.Score` | `float64` | 相似度分数 |
| `items[].citation` | `retrieveItem.Citation` | `citation` | 引用出处信息 |
| `items[].citation.kb_id` | `citation.KBID` | `uint64` | 来源知识库 ID |
| `items[].citation.document_id` | `citation.DocumentID` | `uint64` | 来源文档 ID |
| `items[].citation.chunk_id` | `citation.ChunkID` | `string` | 文档切片唯一标识 |
| `items[].citation.file_name` | `citation.FileName` | `string` | 来源文件名 |
| `items[].citation.chunk_index` | `citation.ChunkIndex` | `int` | 文档切片在原文中的序号 |
| `items[].source` | `retrieveItem.Source` | `source` | 检索源信息 |
| `items[].source.route` | `source.Route` | `string` | 检索路由，如 `dense`、`sparse`、`hybrid` |
| `items[].source.collection` | `source.Collection` | `string` | 向量集合名称 |
| `items[].source.retriever_version` | `source.RetrieverVersion` | `string` | 检索器版本标识 |

### 保存评测样本字段映射

| 页面字段 | API 请求字段 | 后端含义 | 说明 |
| --- | --- | --- | --- |
| 评测集 | `dataset_id`（URL 路径参数） | 目标评测集 ID | 接口为 `POST /admin/kb/eval/datasets/{dataset_id}/items` |
| Case Key | `case_key` | 样本唯一标识 | 非空，评测集内唯一 |
| Query | `query` | 检索查询文本 | 非空 |
| Top K | `top_k` | 最大返回结果数 | 正整数 |
| Query Type | `query_type` | 查询类型 | 可选，如 `factual`、`multi-hop` |
| Collection | `collection` | 向量集合名称 | 可选 |
| Tags | `tags`（逗号/换行分隔后解析为数组） | 标签列表 | 字符串数组 |
| Relevant IDs | `relevant_ids`（逗号/换行分隔后解析为数组） | 标准答案 chunk_id 列表 | 字符串数组，草稿阶段可留空 |
| Notes | `notes` | 备注 | 可选 |

### Contract gap 检查字段清单

前端会对每条检索结果检查以下字段是否完整：

| 检查字段 | 所属层级 | 阻塞验收 | 说明 |
| --- | --- | --- | --- |
| `score` | 根级 | 是 | 相似度分数 |
| `citation` | 根级 | 是 | 引用出处对象 |
| `citation.file_name` | citation | 否 | 来源文件名 |
| `citation.chunk_index` | citation | 否 | 切片序号 |
| `citation.chunk_id` | citation | 否 | 切片唯一标识 |
| `source` | 根级 | 否 | 检索源对象 |
| `source.route` | source | 否 | 检索路由 |
| `source.collection` | source | 否 | 向量集合名称 |
| `source.retriever_version` | source | 否 | 检索器版本 |

其中 `score` 和 `citation` 标记为"阻塞验收"，意味着如果这两个字段缺失，说明检索接口的核心功能存在问题。

## 检索实验室与评测模块的衔接关系

检索实验室和评测模块通过"保存为评测样本"功能直接衔接。整体链路如下：

1. 在检索实验室运行一次检索，确认结果符合预期。
2. 点击"保存为评测样本"，把当前的 query、top_k、chunk_id 列表等信息保存到指定评测集。
3. 前往评测集页面，补齐标准答案（relevant_ids），执行校验。
4. 评测集校验通过后，可以创建评测运行（Eval Run），对 baseline 和 candidate 两个策略配置进行对比评测。
5. 评测报告会给出 recall@k、MRR、NDCG、citation accuracy、P95 延迟等指标，以及自动化的 gate check。

这个衔接的价值在于：检索实验室解决的是"我想看看检索效果"的探索性需求，评测模块解决的是"我要系统性地衡量和对比检索效果"的工程化需求。两者之间的桥梁就是评测样本。

### 关于 Relevant IDs 草稿

保存评测样本时，页面提供了一个"复制当前结果 chunk_id 到 relevant_ids 草稿"的辅助功能。它会把当前检索结果中所有不重复的 `chunk_id` 自动填充到 relevant_ids 字段。

但这里需要注意：自动填充的 chunk_id 是"当前检索返回的结果"，不等于"标准答案"。用户需要自己判断哪些 chunk_id 是真正相关的，删除不相关的，再执行校验。

这个设计故意把"自动填充"和"确认为标准答案"分成两步，避免用户误把所有检索结果都当作正确答案。

## request_id 的作用

`request_id` 是检索实验室的核心枢纽，它连接了三个页面：

### 1. 复制 request_id

点击"复制 request_id"按钮，可以把 UUID 复制到剪贴板。用途包括：

- 粘贴到后端日志中搜索，定位具体的检索执行过程。
- 分享给团队成员，方便协作排查问题。
- 作为工单或 issue 中的复现标识。

### 2. 查看 Trace

点击"查看 Trace"按钮，会跳转到 `/trace-logs/retrieval?request_id=...` 页面。这个页面展示的是该次检索的审计日志记录，包含完整的请求参数、响应结果、耗时等信息。

对应的后端接口是 `GET /api/admin/kb/retrieve/audit/{request_id}`。

### 3. 查看调试视图

点击"查看调试视图"按钮，会跳转到 `/retrieval-lab/debug?request_id=...` 页面。调试视图展示的是该次检索的完整链路追踪，包括：

- 原始 query 和改写后的 query
- 改写策略
- 各路由（dense/sparse）的命中结果
- 融合排序前后的结果对比
- 去重前后的结果对比
- Rerank 前后的结果对比
- 过滤和截断的结果
- Parent-child 填充信息
- Top-K 决策信息
- 证据门控结果
- 引用检查结果
- 各阶段耗时
- 降级信息

对应的后端接口是 `GET /api/admin/kb/retrieve/audit/{request_id}/debug`（也兼容旧路由 `GET /api/admin/kb/retrieve/debug/{request_id}`）。

调试视图的价值在于：当检索结果不符合预期时，不需要翻后端日志，直接在页面上就能看到每个阶段的中间结果，快速定位问题出在哪个环节。

## 后端 API 说明

### 检索接口

**接口地址**：`POST /api/admin/kb/retrieve`

**请求参数**（JSON Body）：

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `kb_id` | `uint64` | 否（与 `kb_ids` 二选一） | 单个知识库 ID |
| `kb_ids` | `[]uint64` | 否（与 `kb_id` 二选一） | 多个知识库 ID |
| `query` | `string` | 是 | 检索查询文本，不能为空 |
| `top_k` | `int` | 否 | 最大返回结果数，默认 5，硬上限 20 |

**响应结构**（成功时）：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "request_id": "uuid-string",
    "items": [
      {
        "content": "文档片段内容",
        "score": 0.85,
        "citation": {
          "kb_id": 1,
          "document_id": 10,
          "chunk_id": "chunk-xxx",
          "file_name": "example.pdf",
          "chunk_index": 3
        },
        "source": {
          "route": "dense",
          "collection": "kb_1",
          "retriever_version": "v1"
        }
      }
    ],
    "evidence_gate_result": "pass",
    "citation_check": {
      "supported": true,
      "support_score": 0.95,
      "unsupported_claims": [],
      "unsupported_claim_count": 0,
      "version": "v1",
      "latency_ms": 120
    }
  }
}
```

**响应字段说明**：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `request_id` | `string` | 本次请求的唯一标识（UUID） |
| `items` | `[]retrieveItem` | 检索结果列表 |
| `evidence_gate_result` | `string` | 证据门控结果，如 `pass`、`refuse` |
| `citation_check` | `object` | 引用检查结果（可选） |
| `citation_check.supported` | `bool` | 引用是否被支持 |
| `citation_check.support_score` | `float64` | 引用支持分数 |
| `citation_check.unsupported_claims` | `[]string` | 不被支持的声明列表 |
| `refusal` | `object` | 拒绝信息（仅在证据门控拒绝时返回） |
| `refusal.reason` | `string` | 拒绝原因 |
| `refusal.message` | `string` | 拒绝提示信息 |

**后端处理流程**：

1. 验证用户身份和权限。
2. 检查速率限制（每用户独立限流）。
3. 绑定和校验请求参数。
4. 检查 RAG 功能是否启用。
5. 获取 Milvus 管理器和检索服务。
6. 根据 feature flag 和 release 决策选择使用 dense 还是 hybrid 检索。
7. 确定目标知识库和向量集合。
8. 执行检索（支持单知识库和多知识库，多知识库时会合并结果）。
9. 对检索结果执行证据门控和引用检查。
10. 构建响应并记录审计日志。
11. 返回结果。

### 获取审计日志接口

**接口地址**：`GET /api/admin/kb/retrieve/audit/{request_id}`

**说明**：根据 `request_id` 获取单条检索审计日志的详情。

### 获取调试视图接口

**接口地址**：`GET /api/admin/kb/retrieve/audit/{request_id}/debug`

**兼容旧路由**：`GET /api/admin/kb/retrieve/debug/{request_id}`

**说明**：根据 `request_id` 获取该次检索的完整调试追踪数据，包括各阶段的中间结果、耗时、降级信息等。

### 创建评测样本接口

**接口地址**：`POST /api/admin/kb/eval/datasets/{dataset_id}/items`

**请求参数**（JSON Body）：

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `case_key` | `string` | 是 | 样本唯一标识 |
| `query` | `string` | 是 | 检索查询文本 |
| `top_k` | `int` | 是 | 最大返回结果数 |
| `query_type` | `string` | 否 | 查询类型 |
| `tags` | `[]string` | 否 | 标签列表 |
| `relevant_ids` | `[]string` | 否 | 标准答案 chunk_id 列表 |
| `collection` | `string` | 否 | 向量集合名称 |
| `notes` | `string` | 否 | 备注 |

### 列出评测集接口

**接口地址**：`GET /api/admin/kb/eval/datasets`

**说明**：获取评测集列表，用于"保存为评测样本"弹窗中的评测集下拉选项。

## 当前实现的限制和注意点

### 限制一：页面只展示检索结果，不展示 evidence_gate 和 citation_check

后端 `retrieveResponse` 结构体包含了 `evidence_gate_result`、`citation_check`、`refusal` 等字段，但当前前端页面没有展示这些信息。如果检索被证据门控拒绝（`refusal` 不为空），用户看到的只是"未返回检索结果"，不会知道是被门控拒绝了。

### 限制二：保存评测样本时 relevant_ids 是手动确认的草稿

自动填充的 chunk_id 只是"当前检索返回的结果"，不等于"标准答案"。页面提示语也明确说了："可留空。只有你确认后才应当作为 golden answer 使用。"如果用户不加筛选直接保存，后续评测的 recall 指标可能会偏高。

### 限制三：Top K 硬上限 20

后端 `maxRetrieveTopK` 常量设为 20。即使前端传更大的值，后端也会 clamp 到 20。当前前端下拉选项最大是 10，所以正常情况下不会触发这个限制，但通过 API 直接调用时需要注意。

### 限制四：检索超时 3 秒

后端默认检索超时是 3 秒（`defaultRetrieveTimeout`）。如果知识库数据量大或检索链路复杂（如包含 rerank），可能会超时。超时后前端会显示错误信息，但不会展示部分结果。

### 限制五：每用户独立限流

后端对每个用户维护了一个独立的速率限制器（`rate.Limiter`）。如果短时间内频繁调用检索接口，可能会触发 429 限流。当前前端没有针对 429 做特殊提示，用户看到的只是通用错误信息。

### 限制六：调试视图依赖后端存储的 debug trace

调试视图的数据来源是后端在检索时生成并持久化的 debug trace。如果后端存储出现问题或 debug trace 被清理，调试视图将无法加载数据。页面会显示错误提示。

### 限制七：当前前端没有展示 source 中的扩展字段

后端 `source` 结构体包含 `parent_id`、`child_id`、`section_title`、`hierarchy_path`、`parent_fill_strategy`、`parent_fill_tokens`、`citation_supported`、`citation_support_score`、`citation_check_version`、`low_support_citation` 等扩展字段，但当前前端结果卡片只展示了 `route`、`collection`、`retriever_version` 三个字段。其他字段需要通过调试视图或审计日志才能看到。

### 限制八：不支持直接在页面上编辑检索参数策略

检索实验室的检索调用使用的是后端默认的检索策略（dense 或 hybrid，由 release 决策和 feature flag 控制）。用户无法在页面上指定使用哪种检索策略、是否启用 rerank、是否启用 query rewrite 等。这些策略调整需要通过策略管理或实验管理模块来完成。

## 开发实现说明

### 前端相关文件

如果你要修改检索实验室页面，建议优先看下面几个文件：

1. `admin/src/components/admin/retrieval-lab-page.tsx`
   作用：检索实验室主组件，包含检索表单、结果展示、Contract gap 检查、保存评测样本弹窗。
2. `admin/src/app/(admin)/retrieval-lab/page.tsx`
   作用：检索实验室路由页面，直接渲染 `RetrievalLabPage` 组件。
3. `admin/src/app/(admin)/retrieval-lab/debug/page.tsx`
   作用：调试视图路由页面，直接渲染 `RetrievalDebugPage` 组件。
4. `admin/src/components/admin/retrieval-debug-page.tsx`
   作用：调试视图主组件，展示检索链路各阶段的中间结果。
5. `admin/src/components/admin/knowledge-base-provider.tsx`
   作用：知识库列表加载、当前知识库状态维护。检索实验室通过 `useKnowledgeBaseContext` 获取知识库列表和当前选中状态。
6. `admin/src/config/api.ts`
   作用：接口地址配置。检索实验室用到的常量包括 `RETRIEVE`、`LIST_EVAL_DATASETS`、`CREATE_EVAL_CASE`。
7. `admin/src/types/kb.ts`
   作用：类型定义。核心类型包括 `RetrieveItem`、`RetrieveResponse`、`RetrievalDebugTrace`、`EvalDataset`。

### 后端相关文件

如果你要查后端行为和数据口径，重点看：

1. `backend/api/router/custom_kb.go`
   作用：注册检索相关路由。检索接口注册在 `/api/admin/kb/retrieve`，调试视图注册在 `/retrieve/audit/:request_id/debug` 和 `/retrieve/debug/:request_id`。
2. `backend/api/handler/kb/handler.go`
   作用：`Retrieve` 函数（约第 847 行开始）是检索接口的核心实现。`retrieveRequest`、`retrieveItem`、`retrieveResponse` 等结构体定义在文件开头（约第 64-164 行）。
3. `backend/api/handler/kb/retrieval_debug_trace_v2.go`
   作用：调试视图的数据构建逻辑。
4. `backend/internal/rag/phase3/contract.go`
   作用：定义调试视图的路由常量 `RetrievalDebugRoute` 和 `LegacyRetrievalDebugRoute`。

### 主要接口清单

| 接口 | 方法 | 作用 |
| --- | --- | --- |
| `/api/admin/kb/retrieve` | `POST` | 执行检索 |
| `/api/admin/kb/retrieve/audit` | `GET` | 列出检索审计日志 |
| `/api/admin/kb/retrieve/audit/{request_id}` | `GET` | 获取单条检索审计日志 |
| `/api/admin/kb/retrieve/audit/{request_id}/debug` | `GET` | 获取检索调试视图 |
| `/api/admin/kb/eval/datasets` | `GET` | 列出评测集（保存样本时用） |
| `/api/admin/kb/eval/datasets/{dataset_id}/items` | `POST` | 创建评测样本 |

## 常见使用场景

### 场景一：验证知识库检索效果

建议顺序：

1. 进入知识库详情页，确认文档已经成功入库（状态为 `completed`）。
2. 点击"打开检索实验室"。
3. 确认知识库已自动选中。
4. 输入一个你期望知识库能回答的问题。
5. 点击"运行检索测试"。
6. 检查返回结果的内容、分数、来源是否符合预期。
7. 如果结果不符合预期，点击"查看调试视图"，检查各阶段的中间结果。

### 场景二：排查检索结果异常

建议顺序：

1. 在检索实验室运行检索，获取 `request_id`。
2. 点击"查看调试视图"。
3. 检查 query 改写是否正确。
4. 检查各路由的命中结果。
5. 检查融合排序、rerank、过滤各阶段是否有异常。
6. 检查是否有证据门控拒绝或降级。
7. 根据调试信息定位问题环节。

### 场景三：沉淀评测样本

建议顺序：

1. 在检索实验室运行多次检索，确认检索效果稳定。
2. 对每次有价值的检索，点击"保存为评测样本"。
3. 选择目标评测集（如果没有，先去创建）。
4. 确认 query、top_k 等字段正确。
5. 点击"复制当前结果 chunk_id 到 relevant_ids 草稿"。
6. 人工审核 relevant_ids，删除不相关的 chunk_id。
7. 保存样本。
8. 前往评测集页面，执行校验，补齐标准答案。
9. 等评测集状态变为 `ready` 后，创建评测运行。

### 场景四：检查接口契约完整性

建议顺序：

1. 在检索实验室运行检索。
2. 观察每条结果卡片上是否有 `Contract gap` 标记。
3. 展开底部的"契约缺口记录"面板，查看缺失字段详情。
4. 根据"阻塞验收"列判断问题的严重程度。
5. 反馈给后端开发，补齐缺失字段。

## 如何验证

如果你要验证这篇文档对应的实现，可以按下面方式检查：

1. 打开 `/retrieval-lab`，确认页面能正常展示检索表单。
2. 选择一个已有知识库，输入查询，点击"运行检索测试"，确认返回结果。
3. 确认结果卡片上正确显示了 score、content、file_name、chunk_index、chunk_id、route、collection、retriever_version。
4. 复制 `request_id`，确认剪贴板内容正确。
5. 点击"查看 Trace"，确认跳转到正确的审计日志页面。
6. 点击"查看调试视图"，确认跳转到正确的调试页面并加载了数据。
7. 点击"保存为评测样本"，确认弹窗正确加载了评测集列表。
8. 确认"复制当前结果 chunk_id 到 relevant_ids 草稿"按钮能正确填充数据。
9. 保存样本后，前往评测集页面确认样本已创建。
10. 人为制造一个后端返回缺失字段的情况，确认 Contract gap 机制正常工作。

## 取舍与后续优化

当前这套检索实验室页面已经具备完整主链路，但还有几个很值得继续优化的点：

1. 把 evidence gate 和 citation check 的结果展示到页面上，让用户知道检索是否被门控拒绝、引用是否被支持。
2. 在检索结果区域增加"策略信息"展示，让用户知道本次检索使用的是什么策略版本、是否在实验中。
3. 支持在页面上选择不同的检索策略配置进行 A/B 对比测试。
4. 保存评测样本时支持自动关联 `request_id`，方便后续回溯。
5. 调试视图增加"重新运行"功能，基于同一个 query 重新发起检索并对比结果。
6. 对 429 限流做前端特殊提示，告知用户需要等待。

这篇文档的核心结论可以概括成一句话：

检索实验室真正管理的不是"搜一下看看"的临时测试，而是"检索效果可观测、可追溯、可沉淀"的完整运维闭环。
