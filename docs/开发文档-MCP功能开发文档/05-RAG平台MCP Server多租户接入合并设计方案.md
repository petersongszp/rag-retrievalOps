# RAG 平台 MCP Server 多租户接入合并设计方案

> 版本：v1.0  
> 日期：2026-06-10  
> 状态：待评审  
> 适用对象：后端开发、架构、测试、运维、交付、业务 Agent 接入方

---

## 1. 背景与目标

当前 RAG 平台已经通过 `POST /v1/retrieve` 对外提供检索能力，并具备 API Key、JWT、租户、知识库授权、检索审计等基础能力。现有 HTTP API 和 Go SDK 能满足服务端集成，但对越来越多采用工具调用模式的 Agent 平台来说，仍然需要业务方理解接口、鉴权、错误处理和结果拼装。

MCP Server 的目标，是把 RAG 平台的检索能力包装成标准工具，让支持 MCP 的客户端和 Agent 平台可以通过工具发现和工具调用方式接入。

### 1.1 核心目标

1. 提供 MCP Tool：`retrieve_knowledge`
2. 支持企业共享部署下的多租户并发调用
3. 支持本地 stdio 调试
4. 复用现有 RAG 平台的 API Key / JWT / tenant / KB permission / audit 能力
5. 保留现有 HTTP `/v1/retrieve` 和 SDK 接入方式
6. 不让 MCP Server 成为新的权限中心

### 1.2 非目标

| 非目标 | 说明 |
|---|---|
| 知识库 CRUD | 创建、删除、上传文档仍属于后台管理能力 |
| 评测、策略、成本、审计后台能力 | V1 不暴露为 MCP Tool |
| RAG 平台作为 MCP Client | 本次只做 MCP Server |
| 多模态检索 | 当前聚焦文本检索 |
| 最终答案生成 | MCP Tool 只返回证据，不替业务方生成最终答案 |
| 新租户体系 | 继续复用现有租户与权限模型 |

---

## 2. 当前代码基础与约束

### 2.1 已具备能力

当前项目已经具备：

- 公共检索接口：`POST /v1/retrieve`
- 请求字段：`app_id`、`kb_id`、`kb_ids`、`query`、`top_k`、`strategy_profile`、`metadata_filter`
- 认证链路：API Key / JWT / legacy app_id
- 租户与知识库授权表：`rag_tenant_kb_permission`
- 检索审计日志：`kb_retrieve_log`
- 审计字段：`tenant_id`、`app_id`、`api_key_id`、`auth_type`、`source_api`、`permission_result`
- Docker Compose 内部网络：`rag-server` 默认不暴露宿主机端口，容器间通过 `rag-network` 访问

### 2.2 需要注意的实现约束

1. `strategy_profile` 和 `metadata_filter` 已出现在公共请求契约里，但当前底层 KB 检索主流程主要使用 `kb_id`、`kb_ids`、`query`、`top_k`。V1 中这两个字段应标注为预留或透传字段，不承诺完整生效。
2. legacy `app_id` 只适合作为旧链路兼容，不建议作为 MCP 生产接入方式。
3. 生产共享部署不能把 API Key 放在 MCP Tool 参数里传递，应使用 HTTP `Authorization` header。
4. stdio 模式只能作为本地单用户调试方式，不承诺多租户并发隔离。
5. V1 优先通过 HTTP 调用 `/v1/retrieve`，避免绕过现有鉴权、审计、限流和身份上下文。

---

## 3. 总体架构

### 3.1 推荐架构

V1 推荐采用“独立 MCP Server + 内部调用 RAG Server”的架构。

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
        ├─ API Key / JWT 校验
        ├─ tenant 解析
        ├─ KB 权限校验
        ├─ 检索执行
        └─ 审计记录
```

### 3.2 为什么 V1 先走 HTTP Adapter

你的原设计里提到直接复用 `internal/ragplatform/application.RetrieveService`，这是一个长期更整洁的方向。但 V1 建议先让 MCP Server 调用 `/v1/retrieve`，原因是：

- 最小化对现有检索核心的侵入
- 避免绕开 HTTP 入口已有的认证和审计逻辑
- 回滚简单，下线 MCP Server 不影响原有 HTTP 接入
- 更容易验证 MCP 只做协议适配
- 后续可以在 V2 再把 HTTP handler 和 MCP handler 收敛到同一个 application service

### 3.3 V2 可演进方向

当 V1 稳定后，可以把 `/v1/retrieve` 和 MCP Tool 共同依赖的逻辑下沉到统一 application service：

```text
HTTP Handler     ┐
                 ├─ RetrieveApplicationService
MCP Tool Handler ┘
```

这样可以减少一次 HTTP hop，但必须确保不会丢失 API Key、JWT、tenant、KB permission、audit、rate limit 等能力。

---

## 4. 技术选型

### 4.1 推荐实现语言

优先使用 Go 实现 MCP Server。

原因：

- 当前 RAG 后端是 Go 技术栈
- 可以复用配置、日志、部署和测试习惯
- 团队维护成本低
- 后续如果抽 application service，Go 内部调用更方便

### 4.2 MCP SDK 选择

优先选择官方 Go SDK：

```text
github.com/modelcontextprotocol/go-sdk/mcp
```

选型理由：

- 官方维护，协议适配风险较低
- 与 TypeScript / Python 官方 SDK 的语义一致
- 后续协议升级时更容易跟进

落地前需要在 Phase 0 确认：

- 当前官方 Go SDK 的最新可用版本
- 是否已稳定支持 stdio
- 是否已稳定支持 Streamable HTTP
- 如果 Streamable HTTP 支持不足，是否需要先做轻量 HTTP JSON-RPC adapter

### 4.3 Transport 选择

| Transport | V1 定位 | 说明 |
|---|---|---|
| Streamable HTTP | 生产共享主路径 | 企业多租户、统一部署、网关接入 |
| stdio | 本地调试路径 | IDE / 桌面客户端 / 单用户验证 |
| SSE | 兼容项，不作为主路径 | 旧客户端需要时再补 |

生产远程接入应优先使用 Streamable HTTP，而不是 SSE。SSE 可以作为兼容能力放到后续阶段。

---

## 5. MCP Transport 设计

### 5.1 Streamable HTTP

生产共享部署推荐使用 Streamable HTTP。

建议约束：

- endpoint：`/mcp`
- 协议版本：优先支持 `2025-06-18`
- 请求：JSON-RPC 2.0
- 响应：V1 优先返回 `application/json`，暂不强制 SSE streaming
- session：V1 可无状态，暂不强依赖 `Mcp-Session-Id`
- GET SSE：V1 可返回 405，后续需要服务端通知时再支持
- Origin：生产环境必须配置白名单
- 认证：所有 tool call 必须携带 `Authorization: Bearer <token>`

网关需要透传：

- `Authorization`
- `MCP-Protocol-Version`
- `Mcp-Session-Id`，如果后续启用 session
- `Accept`
- `Content-Type`

### 5.2 stdio

stdio 仅用于：

- 本地开发
- IDE / 桌面客户端调试
- 单人验证知识库检索效果

stdio 模式约束：

- 从环境变量读取 `RAG_BASE_URL`
- 从环境变量读取 `RAG_ACCESS_TOKEN`
- 不支持每次请求切换 token
- 不支持多租户共享并发
- stdout 只能输出 MCP JSON-RPC 消息
- 普通日志必须输出到 stderr 或文件

示例：

```bash
RAG_BASE_URL=http://localhost:8899
RAG_ACCESS_TOKEN=rag_xxx
```

### 5.3 SSE

SSE 不作为 V1 生产主路径。只有当目标客户端明确不支持 Streamable HTTP 但支持 SSE 时，再作为兼容项实现。

---

## 6. MCP Tool 设计

### 6.1 Tool 名称

统一使用：

```text
retrieve_knowledge
```

不建议使用 `rag_retrieve` 作为 V1 对外工具名，原因是 `retrieve_knowledge` 更贴近 Agent 对工具语义的理解，也和业务说明文档保持一致。

### 6.2 Tool 描述

根据用户问题，在授权知识库范围内检索可引用的知识证据，返回结构化片段供 Agent 生成答案或展示引用。

### 6.3 输入 Schema

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

### 6.4 输入校验规则

| 字段 | 规则 |
|---|---|
| `query` | 必填，去除空白后不能为空，建议最大 2000 字符 |
| `kb_ids` | 推荐字段，数组元素必须为正整数 |
| `kb_id` | 兼容字段，必须为正整数 |
| `kb_ids` / `kb_id` | 至少提供一个 |
| `top_k` | 1 到平台上限，默认 5，最大 20 |
| `strategy_profile` | V1 预留，不保证生效 |
| `metadata_filter` | V1 预留，不保证生效，需限制对象大小和嵌套深度 |

禁止接收以下字段：

- `api_key`
- `tenant_id`
- `api_key_id`
- `user_id`
- `role`
- `auth_type`

这些字段只能由 RAG 平台根据凭证解析，不能由 MCP Client 传入。

### 6.5 输出格式

MCP Tool 返回建议同时包含可读文本和结构化 JSON。

可读文本用于 LLM 快速消费：

```text
检索结果（共 3 条）：

[1] 相关度: 0.92 | 来源: jvm-tuning.md | 知识库: 1 | 文档: 10 | 分块: 3
JVM 调优的核心在于合理配置堆内存大小、选择合适的垃圾回收器...

[2] 相关度: 0.85 | 来源: java-concurrency.md | 知识库: 1 | 文档: 15 | 分块: 7
Java 并发编程中的线程池配置直接影响系统性能...
```

结构化内容用于引用展示、审计排查和 Agent 编排：

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

---

## 7. 认证与权限设计

### 7.1 HTTP MCP 认证

HTTP MCP 请求必须携带：

```http
Authorization: Bearer <RAG_API_KEY 或 RAG_JWT>
```

MCP Server 处理方式：

1. 读取 `Authorization` header
2. 不解析业务身份
3. 不保存明文 token
4. 不打印明文 token
5. 原样转发给 RAG Server `/v1/retrieve`
6. 根据 RAG Server 返回结果转换 MCP tool result 或 MCP error

RAG Server 负责：

- API Key 格式校验
- API Key 过期 / 吊销校验
- JWT 校验
- tenant 解析
- tenant 状态校验
- KB 归属校验
- KB read 权限校验
- 检索审计记录

### 7.2 stdio 认证

stdio 模式通过环境变量读取固定凭证：

```bash
RAG_ACCESS_TOKEN=rag_xxx
```

stdio 不支持每次请求切换 token。如果需要多租户并发，请使用 HTTP MCP。

### 7.3 禁止工具参数传 API Key

远程 HTTP MCP 不允许通过 Tool 参数传递 `api_key`。

原因：

- Tool 参数可能被客户端日志记录
- Tool 参数可能进入对话上下文
- Tool 参数可能被 Agent 平台持久化
- 多租户身份边界会变模糊

生产凭证必须走 HTTP Authorization header。

### 7.4 legacy app_id

legacy `app_id` 仅用于旧 HTTP 接入兼容。MCP V1 默认不允许通过 legacy `app_id` 无凭证访问。

如果短期必须兼容，需要显式开关：

```bash
MCP_ENABLE_LEGACY_APP_ID=false
```

默认必须为 `false`。

---

## 8. 模块划分

### 8.1 推荐目录结构

```text
backend/
├── cmd/
│   └── rag-mcp-server/
│       └── main.go                    # MCP Server 独立入口
├── internal/
│   └── mcp/
│       ├── server.go                  # Server 初始化、工具注册、生命周期管理
│       ├── server_test.go
│       ├── config.go                  # MCP 配置结构体
│       ├── client/
│       │   ├── rag_client.go          # 调用 /v1/retrieve 的 HTTP client
│       │   └── rag_client_test.go
│       ├── handler/
│       │   ├── retrieve.go            # retrieve_knowledge handler
│       │   └── retrieve_test.go
│       ├── tools/
│       │   └── definition.go          # Tool schema 定义
│       ├── transport/
│       │   ├── http.go                # Streamable HTTP
│       │   └── stdio.go               # stdio
│       └── security/
│           ├── origin.go              # Origin 白名单
│           └── redact.go              # token 脱敏
```

### 8.2 新增文件

| 文件 | 说明 |
|---|---|
| `cmd/rag-mcp-server/main.go` | MCP Server 独立启动入口 |
| `internal/mcp/server.go` | 初始化 MCP Server，注册 tools |
| `internal/mcp/config.go` | MCP 配置 |
| `internal/mcp/client/rag_client.go` | 内部调用 `/v1/retrieve` |
| `internal/mcp/handler/retrieve.go` | `retrieve_knowledge` 处理器 |
| `internal/mcp/tools/definition.go` | Tool schema |
| `internal/mcp/transport/http.go` | Streamable HTTP transport |
| `internal/mcp/transport/stdio.go` | stdio transport |
| `internal/mcp/security/origin.go` | Origin 校验 |
| `internal/mcp/security/redact.go` | 日志脱敏 |

### 8.3 需要修改的文件

| 文件 | 修改内容 |
|---|---|
| `backend/config.example.yaml` | 增加 MCP 配置示例 |
| `backend/config.yaml` | 增加本地 MCP 配置 |
| `.env.example` | 增加 MCP 环境变量 |
| `docker-compose.yml` | 增加 `rag-mcp-server` 服务 |
| `README.md` 或集成文档 | 增加 MCP 接入说明 |

### 8.4 V1 尽量不修改的文件

| 文件 | 原因 |
|---|---|
| `api/handler/rag/retrieve.go` | V1 通过 HTTP adapter 复用 |
| `api/ragrouter/register.go` | MCP 路由独立部署 |
| `internal/milvus/retrieval/*` | 检索核心不改 |
| `internal/auth/*` | 认证逻辑不重做 |
| `admin/*` | 管理后台不受影响 |

---

## 9. 配置设计

### 9.1 config.yaml

```yaml
mcp:
  enabled: true
  transport: "http"          # http | stdio
  http:
    host: "0.0.0.0"
    port: 8898
    endpoint: "/mcp"
    allowed_origins: []
  upstream:
    rag_base_url: "http://rag-server:8899"
    timeout_ms: 5000
  auth:
    require_auth: true
    enable_legacy_app_id: false
  tools:
    retrieve_knowledge:
      enabled: true
      default_top_k: 5
      max_top_k: 20
      timeout_ms: 5000
  logging:
    level: "info"
    log_requests: true
    log_query: false
```

### 9.2 环境变量

| 变量 | 说明 | 默认值 |
|---|---|---|
| `MCP_ENABLED` | 是否启用 MCP Server | `false` |
| `MCP_TRANSPORT` | `http` 或 `stdio` | `http` |
| `MCP_HOST` | HTTP 监听地址 | `127.0.0.1` |
| `MCP_PORT` | HTTP 监听端口 | `8898` |
| `MCP_ENDPOINT` | MCP endpoint | `/mcp` |
| `MCP_ALLOWED_ORIGINS` | Origin 白名单 | 空 |
| `MCP_UPSTREAM_TIMEOUT_MS` | 调用 RAG 超时 | `5000` |
| `MCP_ENABLE_LEGACY_APP_ID` | 是否允许 legacy app_id | `false` |
| `RAG_BASE_URL` | RAG Server 地址 | 无 |
| `RAG_ACCESS_TOKEN` | stdio 模式固定凭证 | 无 |

### 9.3 不推荐配置项

不建议在生产配置中保留：

```yaml
mcp:
  auth:
    default_api_key: "..."
```

如果为了本地开发保留，也必须明确：

- 仅 dev/test 环境允许
- prod 环境启动时如果检测到默认 API Key，应直接失败

---

## 10. Docker Compose 部署建议

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
    MCP_ENABLE_LEGACY_APP_ID: "false"
  ports:
    - "8898:8898"
  depends_on:
    rag-server:
      condition: service_healthy
  networks:
    - rag-network
```

说明：

- `rag-mcp-server` 是对外 MCP 入口
- `rag-server` 继续只在内部网络访问
- 生产环境必须配置 `MCP_ALLOWED_ORIGINS`
- 如果走企业网关，网关需要透传 MCP 相关 header 和 Authorization

---

## 11. 客户端配置示例

### 11.1 stdio：Claude Desktop

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "command": "/path/to/rag-mcp-server",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_xxx"
      }
    }
  }
}
```

### 11.2 stdio：Cursor

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "command": "/path/to/rag-mcp-server",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_xxx"
      }
    }
  }
}
```

### 11.3 HTTP MCP：企业 Agent

```http
POST https://rag.example.com/mcp
Authorization: Bearer rag_xxx
Content-Type: application/json
Accept: application/json, text/event-stream
MCP-Protocol-Version: 2025-06-18
```

---

## 12. 错误处理

### 12.1 错误映射

| RAG HTTP 状态 | 场景 | MCP 错误建议 | 是否可重试 |
|---|---|---|---|
| 400 | 参数错误、缺少 query、缺少 kb_id/kb_ids | invalid_request | 否 |
| 401 | 缺少凭证、凭证无效、API Key 过期/吊销 | unauthorized | 否 |
| 403 | 租户停用、KB 未授权、跨租户访问 | forbidden | 否 |
| 404 | KB 不存在 | not_found | 否 |
| 429 | 限流 | rate_limited | 是 |
| 503 | RAG 或 Milvus 不可用 | backend_unavailable | 是 |
| 504 / timeout | 检索超时 | backend_timeout | 是 |
| 5xx | 未预期服务异常 | backend_error | 是 |

### 12.2 MCP 错误响应建议

```json
{
  "code": "unauthorized",
  "message": "API Key 无效或已过期",
  "request_id": "uuid-if-any",
  "retryable": false
}
```

### 12.3 错误处理原则

- 不吞掉 RAG 平台关键错误语义
- 不把所有错误都包装成“调用失败”
- 如果 RAG 平台返回 `request_id`，必须透出
- 认证、授权错误默认不可重试
- 超时、限流、后端不可用可以标记为可重试

---

## 13. 日志、审计与监控

### 13.1 MCP Server 日志

建议记录：

- `mcp_request_id`
- `rag_request_id`
- `tool`
- `transport`
- `duration_ms`
- `upstream_status`
- `error_code`
- token 脱敏标识，例如 hash 前缀

禁止记录：

- 明文 Authorization
- 明文 API Key
- 明文 JWT
- 未脱敏敏感 query

### 13.2 RAG 审计

RAG Server 仍然是审计事实来源。MCP 调用必须能在检索审计中回查：

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

### 13.3 MCP 指标

建议新增：

| 指标名 | 类型 | 说明 |
|---|---|---|
| `mcp_tool_call_total` | Counter | Tool 调用总数 |
| `mcp_tool_call_duration_ms` | Histogram | Tool 调用耗时 |
| `mcp_upstream_error_total` | Counter | 上游 RAG 错误数 |
| `mcp_auth_missing_total` | Counter | 缺少认证次数 |
| `mcp_forbidden_total` | Counter | 权限拒绝次数 |
| `mcp_backend_timeout_total` | Counter | 上游超时次数 |

---

## 14. 安全设计

### 14.1 核心安全原则

1. MCP Server 不做租户决策
2. MCP Server 不保存明文 token
3. MCP Tool 入参不允许覆盖身份
4. 生产 HTTP MCP 必须鉴权
5. 生产 HTTP MCP 必须校验 Origin
6. legacy `app_id` 默认关闭
7. stdio 不作为共享多租户入口

### 14.2 输入防护

| 项目 | 规则 |
|---|---|
| query | 长度限制，去空白，不能为空 |
| kb_ids | 数量限制，正整数 |
| top_k | 上限限制 |
| metadata_filter | 大小限制、深度限制 |
| strategy_profile | 后续如果生效，必须白名单 |
| 禁止字段 | `api_key`、`tenant_id`、`api_key_id`、`user_id` |

### 14.3 网络安全

| 场景 | 措施 |
|---|---|
| stdio | 本地通信，不暴露网络 |
| HTTP MCP | TLS / 网关 / Origin 白名单 / Authorization |
| 内部调用 RAG | 使用 Docker 内部网络或 K8s service |

---

## 15. 分阶段开发计划

### Phase 0：契约冻结与技术验证

目标：

- 冻结 Tool 名称、输入、输出、错误语义
- 确认 Go MCP SDK 可用性
- 确认 `/v1/retrieve` 当前 API Key / JWT 鉴权行为

开发任务：

- 评审本文档
- 验证官方 Go SDK 的 stdio / Streamable HTTP 能力
- 冻结 `retrieve_knowledge` schema
- 明确 `strategy_profile`、`metadata_filter` V1 仅预留

验收标准：

- Tool schema 评审通过
- 错误映射评审通过
- Transport 选择评审通过
- 确认 V1 不通过 tool 参数传 API Key

### Phase 1：stdio 最小可用版本

目标：

- 快速验证 MCP 协议和工具调用可行性
- 支持 Claude Desktop / Cursor 本地调试

开发任务：

- 新增 `cmd/rag-mcp-server`
- 实现 stdio transport
- 注册 `retrieve_knowledge`
- 从 `RAG_BASE_URL`、`RAG_ACCESS_TOKEN` 读取配置
- 调用 `/v1/retrieve`
- 返回可读文本 + 结构化 JSON

验收标准：

- 本地 MCP Client 能发现 `retrieve_knowledge`
- 有效 API Key 能检索授权 KB
- 缺少环境变量时启动失败并给出清晰错误
- stdout 不输出普通日志
- 返回结果与 `/v1/retrieve` 基本一致

### Phase 2：Streamable HTTP 多租户版本

目标：

- 支持企业共享部署
- 支持多租户并发调用
- 形成生产主链路

开发任务：

- 实现 `/mcp` HTTP endpoint
- 支持 JSON-RPC initialize / tools/list / tools/call
- 从 HTTP header 读取 Authorization
- 转发 Authorization 到 RAG Server
- 增加 Origin 白名单
- 增加 upstream timeout
- 增加错误映射
- 增加 token 脱敏日志

验收标准：

- 有效 API Key 调授权 KB 成功
- 无 Authorization 调用失败
- API Key 过期 / 吊销调用失败
- 未授权 KB 调用失败
- 跨租户 KB 调用失败
- Origin 不在白名单时被拒绝
- RAG `request_id` 能透出
- MCP Server 日志无明文 token

### Phase 3：审计、限流与监控加固

目标：

- 满足生产观测和安全审计要求
- 支持稳定试点

开发任务：

- 增加 MCP 指标
- 增加健康检查 `/healthz`、`/readyz`
- 明确限流策略：MCP Server 自有限流或网关限流
- 验证 RAG 审计字段完整
- 增加集成测试和安全测试

验收标准：

- 审计日志可通过 `request_id` 回查
- 审计字段包含 `tenant_id`、`api_key_id`、`auth_type`
- Prometheus 或结构化日志可定位错误
- 关闭 MCP Server 不影响原有 HTTP 检索

### Phase 4：Docker Compose 与交付文档

目标：

- 支持测试环境和演示环境部署
- 交付同学可按文档完成接入

开发任务：

- 新增 `Dockerfile.mcp`
- 修改 `docker-compose.yml`
- 更新 `.env.example`
- 增加客户端配置示例
- 增加排障文档
- 增加回滚方案

验收标准：

- Docker Compose 能启动 MCP Server
- `rag-mcp-server` 能访问内部 `rag-server`
- 健康检查通过
- 文档能指导新用户完成本地 stdio 和 HTTP MCP 接入

### Phase 5：内部试点与 V2 评估

目标：

- 选择 1 到 2 个业务 Agent 试点
- 收集调用质量、错误分布、延迟和接入反馈
- 决定是否进入 V2

观察项：

- 接入耗时是否下降
- Agent 是否能正确使用引用
- 错误是否可理解
- 多租户权限是否稳定
- MCP Client 兼容性问题

V2 候选能力：

- `list_authorized_knowledge_bases`
- `get_retrieve_audit`
- `get_retrieve_debug_trace`
- 更完整的 `metadata_filter`
- 策略 profile 白名单选择
- 抽取统一 `RetrieveApplicationService`

---

## 16. 测试计划

### 16.1 单元测试

- Tool schema 校验
- 空 query
- 缺少 `kb_id` / `kb_ids`
- `kb_id` 和 `kb_ids` 合并
- 禁止身份覆盖字段
- `top_k` 边界
- 错误映射
- token 脱敏

### 16.2 集成测试

- stdio tools/list
- stdio tools/call
- HTTP initialize
- HTTP tools/list
- HTTP tools/call
- 无 Authorization
- 无效 API Key
- 过期 API Key
- 吊销 API Key
- 未授权 KB
- 跨租户 KB
- RAG Server 503
- RAG Server timeout

### 16.3 安全测试

- Origin 白名单
- Authorization 不落日志
- tool 参数不能覆盖 tenant
- tool 参数不能传 api_key
- `metadata_filter` 大对象
- 并发调用下无 token 串用

### 16.4 回归测试

- 原有 `/v1/retrieve` 不受影响
- 管理后台不受影响
- API Key 管理不受影响
- 检索审计不受影响
- Docker Compose 原有服务不受影响

---

## 17. 上线检查清单

- [ ] Tool 名称统一为 `retrieve_knowledge`
- [ ] V1 不通过 tool 参数传 API Key
- [ ] HTTP MCP 使用 Authorization header
- [ ] stdio 仅用于本地调试
- [ ] 生产默认关闭 legacy app_id
- [ ] Origin 白名单已配置
- [ ] token 不落日志
- [ ] RAG_BASE_URL 指向内部服务地址
- [ ] 跨租户访问测试通过
- [ ] 未授权 KB 测试通过
- [ ] API Key 过期 / 吊销测试通过
- [ ] 审计日志可通过 `request_id` 回查
- [ ] 关闭 MCP Server 不影响 `/v1/retrieve`
- [ ] Docker Compose 部署验证通过
- [ ] 回滚方案验证通过

---

## 18. 推荐落地顺序

如果目标是尽快开始开发，推荐顺序是：

1. Phase 0：先冻结契约和 SDK 可用性
2. Phase 1：先做 stdio，快速让 Claude Desktop / Cursor 跑起来
3. Phase 2：做 Streamable HTTP，完成企业多租户主链路
4. Phase 3：补审计、限流、监控和安全测试
5. Phase 4：补部署和交付文档
6. Phase 5：试点后决定 V2

如果目标是优先保障企业共享部署，可以把 Phase 1 和 Phase 2 合并推进，但安全约束不能后置：

- 不允许 tool 参数传 API Key
- 不允许 tool 参数传 tenant
- 必须走 Authorization header
- 必须能通过 RAG 审计回查

---

## 19. 参考资料

- MCP 官方规范：https://modelcontextprotocol.io/specification
- MCP Transport 规范：https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
- MCP Authorization 规范：https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization
- MCP Go SDK：https://github.com/modelcontextprotocol/go-sdk
