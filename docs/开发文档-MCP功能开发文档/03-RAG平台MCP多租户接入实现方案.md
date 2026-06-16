# RAG 平台 MCP 多租户接入实现方案

本文档面向后端开发、平台架构、测试、运维和交付同学，承接《RAG平台MCP多租户接入业务说明》，把 MCP 接入从业务方案拆成可开发、可测试、可上线的阶段任务。

## 1. 实现目标

本次 V1 的目标不是重做 RAG 检索能力，而是在现有 `/v1/retrieve` 之上增加一个 MCP 标准工具入口。

核心目标：

1. 提供 MCP Tool：`retrieve_knowledge`
2. 支持企业共享部署下的多租户并发调用
3. 复用现有 RAG 平台的 API Key / JWT / 租户 / KB 权限 / 审计能力
4. 保留现有 HTTP `/v1/retrieve` 接入方式
5. stdio 仅用于本地调试，不作为生产共享入口

非目标：

1. V1 不新增知识库管理类 MCP Tool
2. V1 不新增独立租户体系
3. V1 不让 MCP Server 自己做 KB 授权判断
4. V1 不负责最终答案生成

## 2. 当前代码基础

当前项目已经具备以下基础：

- 公共检索接口：`POST /v1/retrieve`
- 请求字段：`app_id`、`kb_id`、`kb_ids`、`query`、`top_k`、`strategy_profile`、`metadata_filter`
- 认证链路：API Key / JWT / legacy app_id
- 租户和知识库授权表：`rag_tenant_kb_permission`
- 检索审计日志：`kb_retrieve_log`
- 审计字段：`tenant_id`、`app_id`、`api_key_id`、`auth_type`、`source_api`、`permission_result`

需要注意：

- `strategy_profile` 和 `metadata_filter` 已出现在公共请求契约里，但当前底层 KB 检索主流程主要使用 `kb_id`、`kb_ids`、`query`、`top_k`
- legacy `app_id` 只适合作为旧链路兼容，不建议作为 MCP 生产接入方式
- 当前 `docker-compose.yml` 中 `rag-server` 默认不暴露宿主机端口，容器间通过内部网络访问

## 3. 推荐架构

V1 推荐采用“独立 MCP Server + 内部调用 RAG Server”的方式。

```text
企业 Agent / MCP Client
        │
        │ Streamable HTTP / stdio
        ▼
RAG MCP Server
        │
        │ POST /v1/retrieve
        │ Authorization: Bearer <RAG_API_KEY 或 RAG_JWT>
        ▼
RAG Server
        │
        ├─ 凭证校验
        ├─ tenant 解析
        ├─ KB 权限校验
        ├─ 检索执行
        └─ 审计记录
```

推荐独立 MCP Server 的原因：

- MCP 协议升级不会直接影响 RAG 核心服务
- stdio 和 HTTP MCP 可以复用同一套工具实现
- 方便单独扩缩容、限流和灰度
- 部署边界清晰，MCP Server 只做协议适配

可选方案：

- 方案 A：在 Go 后端新增 `cmd/rag-mcp-server`，复用项目配置和日志体系
- 方案 B：使用官方 TypeScript / Python MCP SDK 新建轻量 adapter 服务

如果团队希望减少技术栈，优先选方案 A。如果希望最快接入 MCP SDK 生态，优先选方案 B。无论采用哪种方案，MCP Server 都只通过 HTTP 调用现有 `/v1/retrieve`。

## 4. MCP Transport 约束

### 4.1 HTTP MCP

生产共享部署推荐使用 Streamable HTTP。

官方 MCP 2025-06-18 规范要求：

- MCP 基于 JSON-RPC 消息
- 标准 transport 包括 stdio 和 Streamable HTTP
- Streamable HTTP 应提供单一 MCP endpoint，例如 `/mcp`
- 客户端 POST JSON-RPC 消息时，需要声明可接受 `application/json` 和 `text/event-stream`
- HTTP 场景需要处理 `MCP-Protocol-Version`
- 服务端需要校验 `Origin`，避免 DNS rebinding 风险
- 服务端应对所有连接实施认证

参考：

- https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
- https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization

V1 建议：

- endpoint：`/mcp`
- 协议版本：优先支持 `2025-06-18`
- 响应模式：先返回 `application/json`，暂不强制 SSE
- session：V1 可做无状态，暂不强依赖 `Mcp-Session-Id`
- GET SSE：V1 可返回 405，后续需要服务端通知时再支持
- Origin：生产环境配置允许域名白名单
- 认证：所有 tool call 必须携带 `Authorization: Bearer <token>`

### 4.2 stdio MCP

stdio 仅用于：

- 本地开发
- IDE / 桌面客户端调试
- 单用户验证知识库检索效果

stdio 模式建议：

- 从环境变量读取 `RAG_BASE_URL`
- 从环境变量读取 `RAG_ACCESS_TOKEN`
- 不支持多租户共享并发
- 不读取用户输入中的 token
- 不输出任何非 MCP JSON-RPC 消息到 stdout

## 5. MCP Tool 契约

### 5.1 Tool 名称

```text
retrieve_knowledge
```

### 5.2 Tool 描述

根据用户问题，在授权知识库范围内检索可引用的知识证据，返回结构化片段供 Agent 生成答案或展示引用。

### 5.3 输入 Schema

```json
{
  "type": "object",
  "required": ["query"],
  "properties": {
    "query": {
      "type": "string",
      "description": "用户问题或检索 query"
    },
    "kb_ids": {
      "type": "array",
      "items": { "type": "integer" },
      "description": "知识库 ID 列表，推荐使用"
    },
    "kb_id": {
      "type": "integer",
      "description": "单知识库 ID，兼容字段"
    },
    "top_k": {
      "type": "integer",
      "minimum": 1,
      "maximum": 20,
      "description": "期望召回数量，最终上限由 RAG 平台控制"
    },
    "strategy_profile": {
      "type": "string",
      "description": "预留字段，只有底层检索链路支持时才保证生效"
    },
    "metadata_filter": {
      "type": "object",
      "description": "预留字段，只有底层检索链路支持时才保证生效"
    }
  }
}
```

业务校验：

- `query` 必填，去除空白后不能为空
- `kb_ids` 和 `kb_id` 至少提供一个
- 不接受 `tenant_id`
- 不接受 `api_key_id`
- 不接受 `user_id`
- `top_k` 不允许超过平台上限

### 5.4 输出结构

```json
{
  "request_id": "string",
  "items": [
    {
      "content": "string",
      "score": 0.92,
      "citation": {
        "kb_id": 1,
        "document_id": 10,
        "chunk_id": "chunk-1",
        "file_name": "example.md",
        "chunk_index": 3
      },
      "source": {
        "route": "dense",
        "collection": "kb_collection",
        "retriever_version": "hybrid-v1"
      }
    }
  ],
  "strategy_version": "optional",
  "request_cost": "optional",
  "citation_check": "optional",
  "refusal": "optional",
  "evidence_gate_result": "optional"
}
```

稳定字段：

- `request_id`
- `items`
- `items[].content`
- `items[].score`
- `items[].citation`
- `items[].source`

增强字段：

- `strategy_version`
- `request_cost`
- `citation_check`
- `refusal`
- `evidence_gate_result`

## 6. 认证与权限设计

### 6.1 HTTP MCP 认证

HTTP MCP 请求必须携带：

```http
Authorization: Bearer <RAG_API_KEY 或 RAG_JWT>
```

MCP Server 处理方式：

1. 读取 `Authorization` header
2. 不解析业务身份
3. 不打印明文 token
4. 原样转发给 RAG Server `/v1/retrieve`
5. 根据 RAG Server 返回结果转换 MCP tool response 或 MCP error

RAG Server 负责：

- API Key 格式和状态校验
- JWT 校验
- 租户状态校验
- KB 归属校验
- KB read 权限校验
- 审计记录

### 6.2 stdio MCP 认证

stdio 模式使用环境变量：

```bash
RAG_BASE_URL=http://localhost:8899
RAG_ACCESS_TOKEN=rag_xxx
```

stdio 模式不支持每次请求切换 token。如果需要多租户并发，请使用 HTTP MCP。

### 6.3 legacy app_id

legacy `app_id` 仅用于旧 HTTP 接入兼容。MCP V1 不建议开放通过 `app_id` 无凭证访问。

如果短期必须兼容 legacy，需要加显式开关：

```bash
MCP_ENABLE_LEGACY_APP_ID=false
```

默认必须为 false。

## 7. 错误映射

MCP Server 不应该把所有错误都包装成“调用失败”。建议保留可诊断语义。

| RAG HTTP 状态 | 场景 | MCP 错误建议 |
|---|---|---|
| 400 | 参数错误、缺少 query、缺少 kb_id/kb_ids | invalid_request |
| 401 | 缺少凭证、凭证无效、API Key 过期/吊销 | unauthorized |
| 403 | 租户停用、KB 未授权、跨租户访问 | forbidden |
| 404 | KB 不存在 | not_found |
| 429 | 限流 | rate_limited |
| 503 | RAG 或 Milvus 不可用 | backend_unavailable |
| 504 / timeout | 检索超时 | backend_timeout |
| 5xx | 未预期服务异常 | backend_error |

Tool 返回中建议包含：

- `code`
- `message`
- `request_id`，如果 RAG 平台已返回
- `retryable`

## 8. 日志与审计

MCP Server 需要记录：

- `mcp_request_id`
- `rag_request_id`
- tool name
- duration
- upstream status
- error code
- token prefix hash 或脱敏标识

MCP Server 禁止记录：

- 明文 Authorization
- 明文 API Key
- 明文 JWT
- 用户完整敏感 query，如果生产合规要求不允许

RAG Server 审计继续作为最终事实来源，需要确认每次检索能记录：

- `request_id`
- `tenant_id`
- `app_id`
- `api_key_id`
- `auth_type`
- `source_api`
- `permission_result`
- `kb_ids`
- `query`
- `result_status`

## 9. 部署方案

### 9.1 Docker Compose 建议

新增服务：

```yaml
rag-mcp-server:
  build:
    context: ./backend
    dockerfile: Dockerfile.mcp
  restart: unless-stopped
  environment:
    MCP_TRANSPORT: http
    MCP_HOST: 0.0.0.0
    MCP_PORT: 8898
    MCP_ENDPOINT: /mcp
    RAG_BASE_URL: http://rag-server:8899
    MCP_ALLOWED_ORIGINS: ${MCP_ALLOWED_ORIGINS:-}
  ports:
    - "8898:8898"
  depends_on:
    rag-server:
      condition: service_healthy
  networks:
    - rag-network
```

说明：

- `rag-mcp-server` 对外暴露 MCP endpoint
- `rag-server` 继续只在内部网络访问
- 生产环境必须配置 `MCP_ALLOWED_ORIGINS`
- 如果 MCP Server 放在企业网关后面，网关需要透传 `Authorization`、`MCP-Protocol-Version`、`Mcp-Session-Id`

### 9.2 环境变量

| 变量 | 说明 | 默认值 |
|---|---|---|
| `MCP_TRANSPORT` | `http` 或 `stdio` | `http` |
| `MCP_HOST` | HTTP 监听地址 | `127.0.0.1` |
| `MCP_PORT` | HTTP 监听端口 | `8898` |
| `MCP_ENDPOINT` | MCP endpoint | `/mcp` |
| `RAG_BASE_URL` | RAG Server 地址 | 无 |
| `RAG_ACCESS_TOKEN` | stdio 模式固定凭证 | 无 |
| `MCP_ALLOWED_ORIGINS` | HTTP Origin 白名单 | 无 |
| `MCP_UPSTREAM_TIMEOUT_MS` | 调用 RAG 超时 | `5000` |
| `MCP_ENABLE_LEGACY_APP_ID` | 是否允许 legacy app_id | `false` |

## 10. 分阶段开发计划

### Phase 0：契约冻结与技术选型

目标：

- 冻结 V1 Tool 名称、输入、输出、错误语义
- 选择 MCP Server 实现技术栈
- 明确 HTTP MCP 与 stdio MCP 的边界

开发任务：

- 新增本实现文档并评审
- 明确是否使用 Go / TypeScript / Python MCP SDK
- 明确 endpoint、端口、部署方式
- 确认 `/v1/retrieve` 当前 API Key / JWT 鉴权行为

验收标准：

- Tool schema 评审通过
- 错误码映射评审通过
- 部署拓扑评审通过
- 明确 V1 不承诺 `strategy_profile`、`metadata_filter` 完整生效

### Phase 1：HTTP MCP 最小可用版本

目标：

- 提供 `/mcp` HTTP endpoint
- 注册 `retrieve_knowledge` tool
- 完成调用 `/v1/retrieve` 的最小闭环

开发任务：

- 新增 MCP Server 启动入口
- 实现 MCP initialize / tools/list / tools/call
- 实现 `retrieve_knowledge` 参数校验
- 转发 `Authorization` 到 RAG Server
- 转换 RAG Server 成功响应为 MCP tool result
- 转换 RAG Server 错误响应为 MCP error
- 增加基础日志

验收标准：

- MCP Client 能发现 `retrieve_knowledge`
- 有效 API Key 调用授权 KB 成功
- 无 token 调用失败
- 未授权 KB 调用失败
- RAG `request_id` 能返回给 MCP Client

### Phase 2：多租户安全与审计闭环

目标：

- 验证 MCP 场景下租户隔离完整
- 保证审计字段可回查
- 补齐安全防护

开发任务：

- 增加 Origin 校验
- 增加 token 脱敏日志
- 禁止 tool 入参包含身份覆盖字段
- 增加 upstream timeout
- 增加 MCP Server 级别限流或接入网关限流说明
- 验证 RAG 审计日志写入 `tenant_id`、`api_key_id`、`auth_type`

验收标准：

- A 租户 token 不能访问 B 租户 KB
- API Key 过期/吊销后 MCP 调用失败
- MCP Server 日志不出现明文 token
- 每次成功检索能通过 `request_id` 回查审计
- Origin 不在白名单时 HTTP MCP 请求被拒绝

### Phase 3：stdio 本地调试版本

目标：

- 支持本地 IDE / 桌面客户端通过 stdio 调试
- 与 HTTP MCP 共用同一个 tool handler

开发任务：

- 新增 stdio transport 启动模式
- 从 `RAG_BASE_URL` 和 `RAG_ACCESS_TOKEN` 读取配置
- 确保 stdout 只输出 MCP JSON-RPC 消息
- 日志输出到 stderr
- 提供本地配置示例

验收标准：

- 本地 MCP Client 能通过 stdio 发现工具
- 本地 token 能成功调用授权 KB
- 缺少环境变量时启动失败并给出清晰错误
- stdout 不输出普通日志

### Phase 4：部署、监控与灰度

目标：

- 能在测试环境稳定部署
- 有基础观测和回滚方案
- 支持小范围业务试点

开发任务：

- 新增 Dockerfile / compose 服务
- 新增 `/healthz` 和 `/readyz`
- 增加 Prometheus 指标或结构化日志
- 增加接入示例文档
- 增加灰度开关
- 编写回滚方案：下线 MCP Server 不影响 `/v1/retrieve`

建议指标：

- `mcp_tool_call_total`
- `mcp_tool_call_duration_ms`
- `mcp_upstream_error_total`
- `mcp_auth_missing_total`
- `mcp_forbidden_total`
- `mcp_backend_timeout_total`

验收标准：

- Docker Compose 能启动 MCP Server
- 健康检查通过
- 失败时能定位是 MCP Server、网关还是 RAG Server 问题
- 关闭 MCP Server 不影响现有 HTTP 检索

### Phase 5：试点交付与 V2 评估

目标：

- 选择 1 到 2 个业务 Agent 试点
- 收集工具调用质量、错误分布、延迟和接入体验
- 决定 V2 是否扩展更多工具

试点观察项：

- 接入耗时是否下降
- Agent 是否能稳定使用引用
- 错误是否可理解
- 多租户权限是否稳定
- MCP Client 兼容性问题

V2 候选能力：

- `list_authorized_knowledge_bases`
- `get_retrieve_audit`
- `get_retrieve_debug_trace`
- 简化版 SDK
- 更完整的 metadata filter
- 策略 profile 白名单选择

## 11. 测试计划

### 11.1 单元测试

- Tool schema 校验
- 参数缺失
- 空 query
- `kb_id` / `kb_ids` 合并
- 禁止身份覆盖字段
- 错误映射
- token 脱敏

### 11.2 集成测试

- MCP tools/list
- MCP tools/call 成功
- 无 Authorization
- 无效 API Key
- 过期 API Key
- 吊销 API Key
- 未授权 KB
- 跨租户 KB
- RAG Server 超时
- RAG Server 503

### 11.3 安全测试

- Origin 白名单
- Authorization 不落日志
- tool 入参不能覆盖租户
- `top_k` 超大值
- `metadata_filter` 大对象
- 并发调用下无 token 串用

### 11.4 回归测试

- 原有 `/v1/retrieve` 不受影响
- 管理后台不受影响
- API Key 管理不受影响
- 检索审计不受影响

## 12. 上线检查清单

- [ ] MCP Server 只做协议适配
- [ ] 生产默认关闭 legacy app_id
- [ ] HTTP MCP 必须鉴权
- [ ] Origin 白名单已配置
- [ ] RAG_BASE_URL 指向内部服务地址
- [ ] token 不落日志
- [ ] 跨租户访问测试通过
- [ ] 未授权 KB 测试通过
- [ ] 审计日志可通过 `request_id` 回查
- [ ] Docker Compose / K8s 部署配置完成
- [ ] 回滚方案已验证

## 13. 推荐开发顺序

建议按下面顺序推进：

1. 先做 Phase 0，冻结契约
2. 再做 Phase 1，跑通 HTTP MCP 最小闭环
3. 然后做 Phase 2，补齐多租户、安全和审计
4. 再做 Phase 3，支持 stdio 本地调试
5. 最后做 Phase 4 和 Phase 5，上线试点并观察

如果排期紧，可以先交付 Phase 1 + Phase 2，这已经能覆盖企业共享部署的核心价值。stdio、本地示例和更多工具可以放到后续迭代。
