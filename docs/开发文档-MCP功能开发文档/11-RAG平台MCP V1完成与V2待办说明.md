# RAG 平台 MCP V1 完成与 V2 待办说明

## 1. 文档目的

本文档用于记录 RAG 平台 MCP Server 当前 V1 已完成的能力，以及后续 V2 计划开发的待办能力。

当前结论：

- MCP V1 主体功能已经完成。
- 本地 Docker Compose 已启动成功。
- Trae 已成功通过 MCP 调用 `retrieve_knowledge` 工具。
- 真实 API Key + 授权知识库的主链路已经验证通过。
- V2 暂不作为当前上线阻塞项，可放入下一次综合迭代中推进。

## 2. 当前 MCP V1 的定位

MCP V1 的目标是：

> 把 RAG 平台已有的知识库检索能力，通过标准 MCP Tool 暴露给 Agent、AI IDE 和其他 MCP Client 使用。

V1 不重做检索系统，也不新建一套权限系统。

V1 的核心设计是：

```text
MCP Client / Agent / AI IDE
        │
        │ MCP 调用 retrieve_knowledge
        ▼
RAG MCP Server
        │
        │ 透传 Authorization
        ▼
RAG Server
        │
        ├─ API Key 校验
        ├─ 租户识别
        ├─ 知识库权限校验
        ├─ 检索执行
        └─ 审计记录
```

也就是说：

- MCP Server 负责协议适配。
- RAG Server 继续负责鉴权、权限、检索和审计。
- MCP Server 不接收工具参数中的 `api_key`、`tenant_id`、`api_key_id` 等身份字段。
- 多租户身份只从 `Authorization: Bearer <token>` 中解析。

## 3. V1 已完成能力

### 3.1 HTTP MCP 接入

已完成 Streamable HTTP MCP 入口：

```text
POST /mcp
```

本地默认地址：

```text
http://localhost:8898/mcp
```

该入口支持 MCP Client 通过标准 JSON-RPC 调用 MCP 工具。

当前已验证：

- MCP initialize 正常。
- tools/list 正常。
- tools/call 正常。
- Trae 可以成功接入并调用 MCP 工具。

### 3.2 `retrieve_knowledge` 工具

V1 已提供核心工具：

```text
retrieve_knowledge
```

作用：

> 根据用户问题，在授权知识库范围内检索可引用的知识证据，返回结构化片段供 Agent 生成答案或展示引用。

典型参数：

```json
{
  "query": "知识库里关于 Go 并发的内容是什么？",
  "kb_ids": [3],
  "top_k": 5
}
```

当前已验证：

- 可以通过 Trae 调用。
- 可以返回检索结果。
- 可以返回 `request_id`。
- 可以返回 `kb_id`、`document_id`、`chunk_index`、`score` 等检索信息。

### 3.3 Authorization 透传

V1 已支持通过 HTTP Header 传递凭证：

```http
Authorization: Bearer <RAG_API_KEY>
```

MCP Server 会把该 Header 透传给内部 RAG Server。

权限判断仍由 RAG Server 完成，包括：

- API Key 是否存在。
- API Key 是否有效。
- API Key 是否过期。
- API Key 是否吊销。
- API Key 属于哪个租户。
- 当前租户是否有权限访问目标知识库。

当前已验证：

- 有效 API Key + 授权 KB 调用成功。
- 不带 Authorization 会被拒绝。
- 无效 API Key 不会返回检索数据。
- 跨租户 KB 访问不会返回数据。

### 3.4 禁止身份字段注入

V1 已禁止客户端在工具参数中传递身份字段。

禁止字段包括：

```text
api_key
tenant_id
api_key_id
user_id
role
auth_type
```

目的：

- 防止客户端伪造租户身份。
- 防止工具参数覆盖服务端解析出的身份。
- 保证租户、API Key、权限只来自 RAG Server 的认证结果。

当前已验证：

- 工具参数中传入 `tenant_id` 会被 schema 拒绝。
- 工具参数中传入 `api_key` 会被 schema 拒绝。

### 3.5 Origin 白名单保护

V1 已实现 HTTP Origin 白名单机制。

配置项：

```env
MCP_ALLOWED_ORIGINS=https://agent.example.com,https://admin.example.com
```

作用：

- 限制哪些前端、Web Agent、AI IDE 或网关来源可以访问 MCP HTTP 入口。
- 防止未授权网页来源跨站调用 MCP 服务。

当前状态：

- 代码能力已完成。
- 生产环境已要求必须显式配置 Origin 白名单。
- 本地测试时可按需跳过。

### 3.6 token 脱敏日志

V1 已实现 token 脱敏日志。

目的：

- 避免完整 API Key / JWT 出现在 MCP Server 日志中。
- 降低日志平台、排障截图、运维输出中的凭证泄露风险。

当前已验证：

- MCP 日志中没有出现完整 API Key。
- 日志中只出现类似 `auth=sha256:xxxx` 的指纹信息。

### 3.7 健康检查与就绪检查

V1 已提供：

```text
GET /healthz
GET /readyz
```

作用：

- `/healthz`：判断 MCP Server 进程是否存活。
- `/readyz`：判断 MCP Server 是否已准备好服务请求，包含对上游 RAG Server 的连通检查。

当前已验证：

- Docker 环境下 `/healthz` 正常。
- Docker 环境下 `/readyz` 正常。

### 3.8 指标暴露

V1 已提供 MCP 指标能力。

用途：

- 统计 MCP 请求量。
- 观察工具调用成功率。
- 观察错误分布。
- 观察上游 RAG Server 调用耗时。
- 后续可接入 Prometheus 或其他监控系统。

### 3.9 Docker Compose 部署

V1 已支持 Docker Compose 部署。

已完成内容包括：

- `rag-mcp-server` 服务。
- `backend/Dockerfile.mcp`。
- Compose 中与 `rag-server`、MySQL、Redis、Milvus 的联动。
- 容器内健康检查。
- MCP Server 停止不影响原 RAG Server。

当前已验证：

- Docker Compose 可以启动。
- `rag-mcp-server` 可以连接 `rag-server`。
- MCP Server 停止后，原 RAG Server 仍保持健康。

### 3.10 stdio 本地调试模式

V1 保留 stdio 模式，主要用于本地调试。

适用场景：

- Claude Desktop 本地调试。
- Cursor 本地调试。
- 单用户本地 MCP 验证。

限制：

- 不适合生产。
- 不适合多租户共享部署。
- 不支持每次请求动态切换 Authorization。

生产保护：

- `APP_ENV=prod && MCP_TRANSPORT=stdio` 时启动失败。

### 3.11 生产启动保护

V1 已补齐关键生产启动保护。

当前保护包括：

```text
APP_ENV=prod && MCP_TRANSPORT=http && MCP_ALLOWED_ORIGINS 为空 -> 启动失败
APP_ENV=prod && MCP_TRANSPORT=stdio -> 启动失败
MCP_ENABLE_LEGACY_APP_ID=true -> 启动失败
```

目的：

- 防止生产环境漏配 Origin 白名单。
- 防止生产误用 stdio。
- 防止 MCP 走 legacy app_id 模式绕过 API Key 多租户鉴权。

### 3.12 Trae 真实接入验证

当前已经通过 Trae 完成真实 MCP 调用验证。

验证问题：

```text
知识库里关于 Go 并发的内容是什么？
```

调用参数：

```json
{
  "query": "知识库里关于 Go 并发的内容是什么？",
  "kb_ids": [3],
  "top_k": 5
}
```

验证结果：

- Trae 成功发现 MCP 工具。
- Trae 成功调用 `retrieve_knowledge`。
- MCP 返回 5 条检索结果。
- 返回了 `request_id`。
- 说明 `Trae -> MCP Server -> RAG Server -> KB -> 检索结果` 链路已打通。

## 4. V1 当前结论

当前 V1 可以认为已经达到：

```text
内部试点可用
```

也就是说：

- 可以给内部 Agent、Trae、AI IDE、小范围业务场景使用。
- 可以继续收集问题、延迟、错误分布和召回质量反馈。
- 暂不需要为了 V2 阻塞其他业务开发。

但如果要扩大到更多租户或更多业务 Agent，建议在 V2 中补齐知识库发现、审计查询和调试追踪能力。

## 5. V2 待办总览

V2 不是对 V1 的重做，而是在 V1 可用基础上的增强。

V2 候选能力包括：

1. `list_authorized_knowledge_bases`
2. `get_retrieve_audit`
3. `get_retrieve_debug_trace`
4. 更完整的 `metadata_filter`
5. 策略 profile 白名单选择
6. 抽取统一 `RetrieveApplicationService`

建议优先级：

| 优先级 | 功能 | 建议 |
|---|---|---|
| P0 | `list_authorized_knowledge_bases` | 优先做 |
| P1 | `get_retrieve_audit` | 第二批做 |
| P1 | `get_retrieve_debug_trace` | 第二批做 |
| P2 | 更完整的 `metadata_filter` | 根据业务需求做 |
| P2 | 策略 profile 白名单选择 | 根据检索策略成熟度做 |
| P3 | 抽取统一 `RetrieveApplicationService` | 重构型任务，最后做 |

## 6. V2 待办功能说明

### 6.1 `list_authorized_knowledge_bases`

#### 功能作用

该功能用于让 Agent 查询当前 API Key 有权限访问哪些知识库。

简单说：

> 让 Agent 在检索前先知道“我能查哪些知识库”。

#### 为什么需要

V1 中调用 `retrieve_knowledge` 必须传：

```json
{
  "kb_ids": [3]
}
```

这意味着 Agent 必须提前知道知识库 ID。

真实场景中会有这些问题：

- Agent 不知道当前租户有哪些知识库。
- 接入方必须人工告诉 Agent `kb_id`。
- 多个租户的知识库 ID 不同，难以硬编码。
- Agent 可能传错知识库 ID。
- 用户不知道自己能访问哪些知识库。

`list_authorized_knowledge_bases` 可以解决这些问题。

#### 预期调用方式

工具名：

```text
list_authorized_knowledge_bases
```

入参：

```json
{}
```

出参示例：

```json
{
  "items": [
    {
      "kb_id": 3,
      "name": "MCP业务开发文档",
      "description": "RAG平台 MCP Server 设计、配置、部署和验收文档",
      "permission": "read",
      "status": "active"
    }
  ]
}
```

#### Agent 使用方式

Agent 可以先调用：

```text
list_authorized_knowledge_bases
```

获得可访问知识库列表，然后根据用户问题选择合适的 `kb_id`，再调用：

```text
retrieve_knowledge
```

#### 安全要求

- 只返回当前 API Key 有权限访问的知识库。
- 不返回其他租户知识库。
- 不返回无权限知识库。
- 不允许通过工具参数传 `tenant_id`、`api_key`、`app_id`。
- 权限判断必须复用 RAG Server 现有认证结果或统一权限逻辑。

#### 建议优先级

P0。

这是 V2 最建议优先开发的功能。

原因：它能显著降低 Agent 接入成本，避免硬编码 `kb_id`。

### 6.2 `get_retrieve_audit`

#### 功能作用

该功能用于根据 `request_id` 查询一次检索调用的审计信息。

简单说：

> 让运维、开发者或 Agent 平台能查清楚“这次检索是谁发起的、查了哪个库、权限是否通过、结果是否成功”。

#### 为什么需要

V1 中 `retrieve_knowledge` 会返回 `request_id`。

但是排查问题时，需要进入数据库或后台日志中手动查。

有了 `get_retrieve_audit` 后，可以直接通过 MCP 工具查询审计结果。

#### 预期调用方式

工具名：

```text
get_retrieve_audit
```

入参示例：

```json
{
  "request_id": "d6350edb-e3cd-42f3-b58c-d5df84e63392"
}
```

出参示例：

```json
{
  "request_id": "d6350edb-e3cd-42f3-b58c-d5df84e63392",
  "tenant_id": 3,
  "api_key_id": 3,
  "auth_type": "api_key",
  "source_api": "v1",
  "kb_ids": [3],
  "permission_result": "allowed",
  "result_status": "success",
  "created_at": "2026-06-11T10:00:00Z"
}
```

#### 典型使用场景

- 用户反馈“Agent 没查到内容”。
- 开发者根据 `request_id` 查询这次调用是否成功。
- 排查 API Key 是否正确。
- 排查是否查错知识库。
- 排查是否因为权限被拒绝。

#### 安全要求

- 只能查询当前 API Key 所属租户范围内的审计记录。
- 不允许跨租户查询其他租户的 `request_id`。
- 返回字段要做脱敏，不能泄露完整 API Key。
- 如果查不到或无权限，返回 `not_found` 或 `forbidden`。

#### 建议优先级

P1。

适合在 `list_authorized_knowledge_bases` 之后开发。

### 6.3 `get_retrieve_debug_trace`

#### 功能作用

该功能用于查询一次检索的调试追踪信息。

简单说：

> 用来分析“为什么这次检索结果不准”。

它比 `get_retrieve_audit` 更偏技术排障。

#### 为什么需要

V1 可以知道一次请求成功或失败，但很难解释：

- query 被如何处理。
- 查了哪些知识库。
- 召回了哪些 chunk。
- score 为什么这么低。
- 是否经过 rerank。
- 最终为什么返回这些片段。

`get_retrieve_debug_trace` 可以帮助开发者分析召回质量问题。

#### 预期调用方式

工具名：

```text
get_retrieve_debug_trace
```

入参示例：

```json
{
  "request_id": "d6350edb-e3cd-42f3-b58c-d5df84e63392"
}
```

出参示例：

```json
{
  "request_id": "d6350edb-e3cd-42f3-b58c-d5df84e63392",
  "query": "知识库里关于 Go 并发的内容是什么？",
  "kb_ids": [3],
  "top_k": 5,
  "candidates": [
    {
      "document_id": 7,
      "chunk_index": 2,
      "score": 0.0082,
      "selected": true
    }
  ],
  "latency_ms": 123
}
```

#### 典型使用场景

- 召回内容不相关。
- 用户问题和知识库内容匹配不好。
- `top_k` 太小或太大。
- 需要分析 chunk、score、rerank 效果。
- 需要优化 embedding、切片或检索策略。

#### 安全要求

- 只能查询当前租户自己的 debug trace。
- 不返回其他租户内容。
- 不返回完整敏感请求头。
- 对 chunk 内容返回要谨慎，必要时只返回摘要或 ID。

#### 建议优先级

P1。

如果后续试点中频繁出现“检索不准”，该功能优先级应提高。

### 6.4 更完整的 `metadata_filter`

#### 功能作用

该功能用于让 Agent 在检索时增加更细粒度的过滤条件。

简单说：

> 不只是按知识库检索，还可以按文档属性、标签、时间、来源等条件过滤。

#### 为什么需要

V1 中已有 `metadata_filter` 预留字段，但能力较基础。

真实业务中可能需要：

- 只查某类文档。
- 只查某个项目。
- 只查某个版本。
- 只查最近更新的内容。
- 只查某种来源的文件。

#### 预期调用方式

示例：

```json
{
  "query": "MCP Origin 白名单怎么配置？",
  "kb_ids": [3],
  "top_k": 5,
  "metadata_filter": {
    "document_type": "design_doc",
    "tags": ["MCP", "配置"],
    "updated_after": "2026-01-01"
  }
}
```

#### 典型使用场景

- 多文档类型混在同一个知识库。
- 只想检索正式文档，不想检索草稿。
- 只想查某个业务线或项目模块。
- 需要按版本或时间过滤。

#### 安全要求

- 过滤字段必须白名单化。
- 不允许客户端构造任意数据库查询条件。
- 限制 filter 对象大小和嵌套深度。
- 避免通过 filter 推断其他租户数据。

#### 建议优先级

P2。

建议等基础 MCP 使用稳定后，再结合真实业务需求设计。

### 6.5 策略 profile 白名单选择

#### 功能作用

该功能用于让 Agent 在检索时选择预设的检索策略。

简单说：

> 让不同场景使用不同的检索策略，但只能从服务端允许的白名单中选择。

#### 为什么需要

不同问题可能需要不同检索策略。

例如：

- 精准问答：更重视高相关度。
- 宽泛总结：需要更多上下文。
- 面试问答：可能需要召回问答型内容。
- 代码文档：可能需要更偏精确匹配。

如果所有场景都用同一套策略，效果可能不稳定。

#### 预期调用方式

示例：

```json
{
  "query": "这个项目的熔断机制是怎么设计的？",
  "kb_ids": [3],
  "top_k": 5,
  "strategy_profile": "technical_qa"
}
```

服务端只允许白名单中的 profile，例如：

```text
default
technical_qa
summary
interview
```

#### 典型使用场景

- 技术问答。
- 项目总结。
- 面试辅助。
- 客服知识库。
- 代码文档检索。

#### 安全要求

- profile 必须服务端白名单化。
- 不允许客户端传任意策略配置。
- 不允许通过 profile 绕过权限或扩大检索范围。
- profile 只能影响检索策略，不能影响租户权限。

#### 建议优先级

P2。

建议在积累足够调用样本后再做，避免提前设计过多策略。

### 6.6 抽取统一 `RetrieveApplicationService`

#### 功能作用

该功能属于代码架构优化。

简单说：

> 把 `/v1/retrieve` 和 MCP `retrieve_knowledge` 共用的检索业务逻辑抽到统一应用服务中，避免两套入口重复实现。

#### 为什么需要

随着 MCP 能力增加，可能会出现：

- HTTP API 一套检索逻辑。
- MCP 工具一套检索逻辑。
- 审计、权限、错误映射、指标处理分散在不同层。

长期看会增加维护成本。

统一 `RetrieveApplicationService` 可以让：

```text
/v1/retrieve
MCP retrieve_knowledge
未来其他入口
```

共用同一套业务编排。

#### 预期职责

`RetrieveApplicationService` 可以负责：

- 接收标准化检索请求。
- 调用权限校验。
- 调用检索引擎。
- 处理审计日志。
- 处理错误映射。
- 统一返回检索结果。

示意：

```text
HTTP Handler
MCP Tool Handler
      │
      ▼
RetrieveApplicationService
      │
      ├─ Auth / Permission
      ├─ Retrieval Engine
      ├─ Audit Log
      └─ Metrics / Error Mapping
```

#### 典型收益

- 减少重复代码。
- 避免 MCP 和 HTTP API 行为不一致。
- 后续增加新入口更容易。
- 审计、权限、错误处理更统一。

#### 风险

这是重构型任务，不是新功能。

风险包括：

- 可能影响现有 `/v1/retrieve`。
- 需要较完整的回归测试。
- 不适合和大量新功能混在同一轮做。

#### 建议优先级

P3。

建议等 V2 工具能力稳定后，再单独做一次重构。

## 7. V2 推荐开发顺序

建议不要一次性开发全部 V2 能力。

推荐顺序：

### 第一批：知识库发现能力

```text
list_authorized_knowledge_bases
```

目标：

- 解决 Agent 不知道 `kb_id` 的问题。
- 降低接入方配置成本。
- 支持多知识库、多租户场景。

### 第二批：排障与审计能力

```text
get_retrieve_audit
get_retrieve_debug_trace
```

目标：

- 让 request_id 真正可查。
- 支持排查权限、错误、召回质量问题。
- 提高内部试点和生产排障效率。

### 第三批：检索增强能力

```text
更完整的 metadata_filter
策略 profile 白名单选择
```

目标：

- 提升复杂业务场景下的检索控制能力。
- 支持不同业务 Agent 使用不同检索策略。

### 第四批：架构收敛

```text
抽取统一 RetrieveApplicationService
```

目标：

- 统一 HTTP API 与 MCP 工具的检索业务逻辑。
- 降低长期维护成本。

## 8. V2 开发前建议补充的测试

在正式推进 V2 前，建议补齐以下测试数据和验证：

1. 过期 API Key。
2. 吊销 API Key。
3. 未授权知识库。
4. 跨租户知识库。
5. 多知识库授权场景。
6. 至少两个真实 Agent 或 MCP Client。
7. 一组真实业务问题及对应期望知识库。

这些数据可以帮助判断 V2 需求的真实优先级。

## 9. 当前建议

当前建议是：

```text
MCP V1 标记为内部试点通过。
V2 进入下一次综合迭代计划。
下一次优先开发 list_authorized_knowledge_bases。
```

不建议现在一次性展开全部 V2。

如果后续时间有限，V2 最小可交付版本建议只做：

```text
list_authorized_knowledge_bases
```

如果试点过程中排障需求明显增加，再追加：

```text
get_retrieve_audit
get_retrieve_debug_trace
```

## 10. 总结

MCP V1 已经完成从“RAG 检索能力”到“Agent 可调用 MCP Tool”的关键闭环。

V1 解决的是：

```text
Agent 能不能通过 MCP 调 RAG 知识库
```

V2 要解决的是：

```text
Agent 怎么更聪明地选择知识库
开发者怎么更方便地排查调用问题
检索参数怎么更适合复杂业务场景
代码架构怎么更长期可维护
```

因此，V2 应作为体验增强、排障增强和架构增强来规划，而不是当前 V1 上线前的阻塞项。
