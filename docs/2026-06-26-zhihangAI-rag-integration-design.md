# zhihangAI 对接 rag-retrievalOps 的 Agent-RAG 集成设计

## 1. 背景

当前我们已经有两个方向明确但职责不同的项目：

- `zhihangAI`：（`zhihangAI是`我们公司的名字，学员们可以替换成自己公司的名字），面向最终用户和业务方的 Agent 应用层，已经具备 Agent 管理、对话、Prompt 配置、Skill、Tool、Workflow、会话管理等通用能力。
- `rag-retrievalOps`：面向企业级知识问答场景的 RAG 中台，已经具备知识库管理、检索策略、权限隔离、可观测性、评测治理等能力。

项目建设的目标不是重复造轮子，而是把两个项目的优势整合起来：

1. 把项目开发好，形成真正可落地、可扩展的 Agent + RAG 中台方案。
2. 把项目和设计讲明白，让项目能服务于本人和学员的学习、面试、求职与项目包装。

因此，后续所有设计与开发都只服务于两个目标：

- **目标一：把项目开发好。**
- **目标二：把项目和学员们讲明白，并及时沉淀文档。**

## 2. 项目定位

### 2.1 总体定位

整体方案定位为：

> 一个面向行业知识问答与业务咨询场景的多租户 Agent 平台，其中 `zhihangAI` 负责 Agent 应用层，`rag-retrievalOps` 负责 RAG 检索与治理中台。

### 2.2 分层定位

#### zhihangAI

`zhihangAI` 负责 Agent 的应用与编排能力，包括：

- 对话入口
- Agent 配置
- Prompt 配置
- Skill 安装与管理
- Tool 管理
- Workflow 编排
- 会话管理
- 多 Agent 组合能力

#### rag-retrievalOps

`rag-retrievalOps` 负责 RAG 中台能力，包括：

- 多租户知识库管理
- 检索 API
- 检索策略与策略版本
- 权限隔离
- 引用溯源
- 检索日志
- 可观测性
- 质量评测与回归

### 2.3 对外讲法

对本人和学员而言，项目可以分两层来讲：

- **上层：`zhihangAI`** **是 Agent 平台。**
- **下层：`rag-retrievalOps`** **是企业级 RAG 检索与治理中台。**

这样既能讲应用层落地，又能讲平台层深度。

## 3. 本次集成目标

本次设计聚焦于第一阶段集成，不追求一步到位，而是优先打通最小可用链路。

### 3.1 本次要实现的目标

- 让 `zhihangAI` 的 Agent 可以直接调用 `rag-retrievalOps` 做知识检索。
- 保持 `zhihangAI` 现有 Agent、Skill、Tool、Workflow 框架不推倒重来。
- 保持 `rag-retrievalOps` 现有公开检索接口不大改。
- 让第一版能支撑两个样板 Agent：
  - 智能客服 Agent
  - 企业制度 / 流程问答 Agent

### 3.2 本次不做的内容

第一版暂不包含：

- 检索结果精排可视化
- 前端完整 citation 面板
- 检索评测页与 Agent 页联动
- 本地 knowledge 与外部 RAG 的混合融合排序
- 租户级精细化密钥托管体系

## 4. 设计原则

本次集成遵循以下原则：

### 4.1 最小侵入

尽量不重构 `zhihangAI` 现有 Agent 主链路，只在检索入口增加 provider 分发能力。

### 4.2 可灰度切换

保留 `zhihangAI` 原有本地 knowledge 能力，允许 Agent 在 `local` 与 `retrievalops` 两种检索来源之间切换。

### 4.3 不重复造轮子

不重新开发一个新的 Agent 平台，也不在 `rag-retrievalOps` 中重做对话工作台。

### 4.4 先打通主链路，再逐步增强

第一版先解决“能接、能用、能讲清楚”，后续再逐步增强 citation 展示、可观测性联动、评测回挂等能力。

### 4.5 文档同步更新

项目开发过程中，文档必须同步维护，并保存在 `docs/` 目录下，用于本人和学员理解项目设计、演进路线与落地方案。

## 5. 方案选择与结论

### 5.1 候选方案

#### 方案 A：最小侵入集成

- `zhihangAI` 保持 Agent 前台定位
- `rag-retrievalOps` 作为外部检索中台
- 第一版只引入 `ragConfig + retrieval client + provider 切换`

#### 方案 B：统一知识中台

- 逐步废弃 `zhihangAI` 本地 knowledge
- 所有检索都切换到 `rag-retrievalOps`

#### 方案 C：双知识源混合检索

- Agent 同时检索本地 knowledge 和 `rag-retrievalOps`
- 再做合并、去重和排序

### 5.2 最终选择

本次选择 **方案 A：最小侵入集成**。

### 5.3 选择理由

- 改动面最小，容易落地。
- 风险最低，不会破坏现有 Agent 能力。
- 便于教学和求职表达，能同时讲清楚 Agent 层与 RAG 中台层。
- 后续可以平滑演进到方案 B 或方案 C。

## 6. 目标架构

### 6.1 架构职责划分

#### zhihangAI 职责

- Agent 管理
- Agent 对话
- Prompt 配置
- Skill / Tool / Workflow 编排
- 会话管理
- Retrieval Provider 选择
- 检索结果在对话中的展示

#### rag-retrievalOps 职责

- 多租户知识库
- 检索 API
- 引用溯源
- 检索日志
- 权限控制
- 可观测性与评测

### 6.2 调用关系

整体调用链路如下：

1. 用户进入 `zhihangAI` 的 Agent 对话页。
2. 用户发送消息。
3. `zhihangAI` 读取 Agent 的 `ragConfig`。
4. 若 `provider=local`，则调用本地 knowledge 检索。
5. 若 `provider=retrievalops`，则调用 `rag-retrievalOps /v1/retrieve`。
6. 检索结果被转换为 `ragContext`。
7. `ragContext` 注入大模型 prompt。
8. 大模型生成回答。
9. 回答通过 SSE 流式返回前端。
10. 检索参考内容可作为中间消息在对话中展示。

## 7. 核心链路设计

### 7.1 Agent 对话主链路

`zhihangAI` 保持现有 Agent 对话主链路不变：

- 接收用户消息
- 加载 Agent 配置
- 构建模型与 Tool / Skill / Workflow 能力
- 构建 RAG 上下文
- 发起 LLM 推理
- SSE 返回前端

本次改造只聚焦于“构建 RAG 上下文”的实现来源。

### 7.2 RAG 上下文构建

RAG 上下文生成流程：

1. 读取 Agent 的 `ragConfig`
2. 根据 `provider` 选择检索实现
3. 发起知识检索
4. 将检索结果转换为统一结果结构
5. 取前若干条结果拼接成 `ragContext`
6. 将 `ragContext` 追加到系统提示词模板中
7. 将检索片段作为一条系统中间消息回传给前端

### 7.3 Provider 分发策略

建议支持以下两类 provider：

- `local`
- `retrievalops`

第一版不支持同一轮对话中同时调用两个 provider 并做融合。

## 8. 数据模型设计

### 8.1 Agent 增加 RAG 配置

建议为 `zhihangAI` 的 Agent 增加统一 JSON 配置字段：

```json
{
  "provider": "local | retrievalops",
  "externalKbIds": [101, 102],
  "topK": 4,
  "strategyProfile": "default",
  "apiKeyRef": "tenant_default"
}
```

### 8.2 字段说明

- `provider`：检索来源，支持 `local` 与 `retrievalops`
- `externalKbIds`：对接 `rag-retrievalOps` 时使用的外部知识库 ID 列表
- `topK`：召回条数
- `strategyProfile`：检索策略 profile
- `apiKeyRef`：预留给后续多租户 / 多 Agent 密钥绑定能力

### 8.3 第一版默认值

建议默认值如下：

- `provider = local`
- `topK = 4`
- `strategyProfile = default`

## 9. zhihangAI 后端改造设计

### 9.1 模型层改造

需要在 `Agent` 模型中增加：

- `ragConfig`

推荐实现：

- 字段类型使用 JSON
- 存储在 `agents` 表的 `rag_config` 字段中

### 9.2 请求结构改造

`CreateAgentReq` 与 `UpdateAgentReq` 需要支持：

- `ragConfig`

这样 Agent 创建与更新时即可保存外部 RAG 配置。

### 9.3 检索适配层

建议新增一个很薄的适配层，例如：

- `internal/integrations/retrievalops/client`

其职责仅包括：

- 配置 `baseURL`
- 配置 `apiKey`
- 控制 `timeout`
- 发起 retrieve 请求
- 解析响应
- 映射为 `zhihangAI` 内部统一结构

### 9.4 检索入口改造

建议保留原有本地检索方法，并新增外部检索方法：

- `searchKnowledgeBase()`：本地实现
- `retrieveFromRetrievalOps()`：远程实现

然后在 `buildRagContext()` 中按 `ragConfig.provider` 做分发。

### 9.5 错误处理策略

第一版采用保守策略：

- 远程 RAG 调用失败时，不中断整条对话主链路。
- 降级为“空 RAG 上下文”继续执行。
- 同时在日志中记录错误，便于排查。

## 10. zhihangAI 统一结果结构设计

### 10.1 现状问题

`zhihangAI` 当前知识检索结果结构过于简单，仅包含：

- `content`

这会导致后续无法平滑接入：

- score
- citation
- source
- request\_id

### 10.2 建议结构

建议扩展为：

```go
type SearchKnowledgeBaseResult struct {
    Content    string                 `json:"content"`
    Score      float64                `json:"score,omitempty"`
    SourceName string                 `json:"sourceName,omitempty"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
```

### 10.3 字段映射

`rag-retrievalOps` 返回字段映射如下：

- `content <- item.content`
- `score <- item.score`
- `sourceName <- item.citation.file_name`
- `metadata <- citation + source + request_id`

这样第一版即使前端暂不展示 citation，也不会丢失数据。

## 11. rag-retrievalOps 对接协议

### 11.1 协议复用原则

第一版不为 `zhihangAI` 单独新增专用接口，直接复用 `rag-retrievalOps` 现有公开检索协议。

### 11.2 请求字段

请求体使用：

- `query`
- `kb_ids`
- `top_k`
- `strategy_profile`
- 可选 `metadata_filter`

### 11.3 鉴权方式

使用：

- `Authorization: Bearer <api_key>`

第一版建议使用服务级 API Key。后续再扩展到租户级或 Agent 级 API Key。

### 11.4 响应字段

重点复用：

- `items[].content`
- `items[].score`
- `items[].citation`
- `items[].source`
- `request_id`
- `strategy_version`

## 12. 前端改造设计

### 12.1 Agent 配置页

在 `zhihangAI` 的 Agent 配置页面增加以下项：

- 检索来源 provider
- 外部知识库 ID 列表
- topK
- strategyProfile

### 12.2 聊天页

第一版聊天页尽量少改：

- 保持现有 SSE 对话逻辑
- 在中间消息区域展示“检索参考内容”

### 12.3 后续演进

后续可以增强：

- citation 面板
- 来源文件展示
- 检索调试视图
- request\_id 跳转到 RetrievalOps 日志页

## 13. 样板 Agent 设计

### 13.1 智能客服 Agent

适用场景：

- 订单咨询
- 退款咨询
- 计费咨询
- 价格与政策咨询

重点展示能力：

- 多租户知识库支撑
- 客服场景知识问答
- 可替换策略与检索能力

### 13.2 企业制度 / 流程问答 Agent

适用场景：

- 请假制度
- 报销制度
- 审批流程说明
- 常见管理制度问答

重点展示能力：

- 企业内部知识问答
- 流程类知识解释
- Agent 配置与组织知识接入能力

## 14. 多租户与权限策略

### 14.1 第一版策略

第一版先采用较简单的方式：

- `zhihangAI` 使用服务级 API Key 调用 `rag-retrievalOps`
- `rag-retrievalOps` 负责对该 API Key 对应租户、知识库权限进行校验

### 14.2 后续演进方向

后续可以扩展为：

- 每个租户独立 API Key
- 每个 Agent 绑定独立 API Key
- 按租户自动映射 `externalKbIds`

## 15. 风险与决策

### 15.1 本地 knowledge 与外部 RAG 的边界

风险：

- 两套检索能力并存，容易产生职责混淆。

决策：

- 第一版不替换本地 knowledge，只做 provider 切换。

### 15.2 API Key 管理方式

风险：

- 若一开始就做租户级密钥管理，复杂度过高。

决策：

- 第一版先使用服务级 API Key。

### 15.3 返回字段不一致

风险：

- 本地 knowledge 与外部 RetrievalOps 返回结构不同。

决策：

- 在 `zhihangAI` 适配层做统一映射。

### 15.4 外部 RAG 不可用

风险：

- `rag-retrievalOps` 不可用会影响对话可用性。

决策：

- 第一版先降级为空上下文继续回答。
- 后续再增加 fallback 策略。

## 16. 分阶段实施计划

### 阶段一：最小可用集成

- 给 `zhihangAI` 的 Agent 增加 `ragConfig`
- 增加 RetrievalOps client
- 支持 provider 分发
- 打通客服 Agent 的检索调用

### 阶段二：第二个样板 Agent

- 接通制度 / 流程问答 Agent
- 完善提示词模板与样板配置

### 阶段三：结果可视化增强

- 展示 citation / source
- 在对话中显示更清晰的引用信息

### 阶段四：中台联动增强

- 将 `request_id` 与 RetrievalOps 日志关联
- 与检索评测、回归能力联动

## 17. 教学与求职表达方式

### 17.1 对学员的价值

该方案适合用于：

- 项目实战教学
- 求职作品包装
- 面试项目讲解
- AI Agent + RAG 中台的综合案例讲解

### 17.2 面试表达建议

项目讲解可以拆成两层：

#### 应用层

- 我们做了 `zhihangAI`，是一个支持 Prompt、Skill、Tool、Workflow 的 Agent 平台。

#### 平台层

- 我们做了 `rag-retrievalOps`，作为企业级 RAG 检索与治理中台，支持多租户知识库、权限、观测与评测。

#### 集成层

- 我们把 Agent 平台和 RAG 中台做了解耦集成，让 Agent 能灵活使用不同知识检索能力。

## 18. 结论

本次集成的核心结论如下：

- 不重写 Agent 平台。
- 不把 `rag-retrievalOps` 强行改成前台聊天产品。
- 采用 `zhihangAI + rag-retrievalOps` 的分层方案。
- 第一版走最小侵入集成。
- 所有设计与开发，都服务于“把项目开发好”和“把项目讲明白”这两个目标。

这份文档作为当前阶段的集成设计基线，后续开发和讲解都应以此为依据推进。

## 19. 当前实现状态

截至当前轮次，已经完成以下落地内容：

- `zhihangAI` 的 `Agent` 模型新增 `ragConfig`，支持保存 RAG provider 配置。
- `zhihangAI` 的创建 / 更新 Agent 请求已支持 `ragConfig`。
- `zhihangAI` 的统一检索结果结构已扩展为支持 `content`、`score`、`sourceName`、`metadata`。
- `zhihangAI` 已新增 `retrievalops` 远程检索 client，并完成最小单元测试。
- `zhihangAI` 的 Agent 聊天链路已支持根据 `ragConfig.provider` 在本地 knowledge 与 `rag-retrievalOps` 之间分发。
- `zhihangAI` 的 Agent 创建弹窗已增加最小版 RAG 配置项，包括 provider、external KB IDs、topK、strategyProfile。

### 19.1 已验证内容

- `app/internal/integrations/retrievalops` 包的 client 测试已通过。

### 19.2 当前已知阻塞

- 后端大范围 `go test` 仍然较重，完整回归还需要继续分批验证。
- 前端 `pnpm type-check` 当前存在仓库原有问题：项目中混入了多份 `*.vue.js` 文件，导致 TypeScript 在未开启 `allowJs` 时直接失败。这一问题不是本轮 `ragConfig` 改造引入的，需要在后续单独清理或调整前端类型检查配置。

### 19.3 下一步开发重点

- 补齐 `retrievalOps` 运行时配置读取与注入。
- 继续完善 Agent 编辑 / 管理页对 `ragConfig` 的完整编辑能力。
- 做后端更多定向测试和必要的服务层验证。
- 继续补齐教学交付文档，确保项目开发与学员讲解同步推进。
