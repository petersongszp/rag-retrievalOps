# RAG 平台 MCP Server 接入设计文档

> 版本：v1.0 | 日期：2026-06-10 | 状态：待审核

---

## 一、背景与目标

### 1.1 背景

当前 RAG 平台对外暴露检索能力的方式有两种：

1. **HTTP REST API**：`POST /v1/retrieve`，支持 API Key / JWT / Legacy app_id 三种认证
2. **Go SDK**：`pkg/ragsdk/`，封装了 HTTP 调用

这两种方式都需要 Agent 开发者手动集成代码。随着 MCP（Model Context Protocol）成为 AI 工具互联的事实标准（2024.11 Anthropic 发布 → 2025 年 Linux Foundation 治理 → 2026 年企业级广泛采用），将 RAG 平台暴露为 MCP Server 可以让所有支持 MCP 的 AI 客户端（Claude Desktop、Cursor、Windsurf、自研 Agent 等）零代码接入检索能力。

### 1.2 目标

将 RAG 平台的检索能力封装为 **MCP Server**，使任何符合 MCP 协议的客户端可以直接调用 RAG 检索，无需编写集成代码。

### 1.3 非目标（业务边界）

以下能力 **不在** 本次设计范围内：

| 排除项 | 原因 |
|--------|------|
| 知识库 CRUD（创建/删除/文档上传） | 管理操作属于 Admin 职责，不应通过 MCP 暴露给 AI 工具 |
| 评测、策略、成本、审计等运维 API | 属于平台内部运维能力，非检索消费场景 |
| RAG 平台作为 MCP Client 调用外部工具 | 本次只做 Server 侧，不做 Client 侧 |
| 多模态检索（图片/音频） | 当前 RAG 平台仅支持文本检索 |
| 流式检索结果 | 检索是一次性返回，无需流式 |

---

## 二、技术选型

### 2.1 MCP Go SDK 对比

| SDK | 来源 | 版本 | 传输方式 | 特点 |
|-----|------|------|----------|------|
| `github.com/modelcontextprotocol/go-sdk/mcp` | 官方（Anthropic/Linux Foundation） | v1.2.0+ | stdio, SSE, HTTP | 官方维护，合规性最佳 |
| `github.com/localrivet/gomcp` | 社区 | v1.5.0 | stdio, HTTP, WebSocket, SSE | 功能丰富，API 已锁定 |
| `github.com/llmcontext/gomcp` | 社区 | 早期 | stdio | 轻量，配置驱动 |

**选型决策：`github.com/modelcontextprotocol/go-sdk/mcp`（官方 SDK）**

理由：
- 官方维护，协议合规性有保障
- 与 TypeScript/Python SDK 一致的 API 设计
- 支持 stdio + SSE 两种传输
- 社区活跃，微软等大厂已在使用

### 2.2 MCP 协议基础

MCP 基于 **JSON-RPC 2.0** 协议，不是独立协议，而是在 JSON-RPC 之上定义了标准化的工具发现和调用语义。

```
┌─────────────────────────────────────────────┐
│              MCP 协议层                       │
│  (tools/list, tools/call, resources/*, etc.) │
├─────────────────────────────────────────────┤
│              JSON-RPC 2.0                    │
├─────────────────────────────────────────────┤
│  传输层：stdio | SSE (HTTP) | Streamable HTTP │
└─────────────────────────────────────────────┘
```

**传输方式**：

| 传输 | 通信方式 | 适用场景 |
|------|----------|----------|
| **stdio** | 标准输入输出 | 本地客户端（Claude Desktop、Cursor）将 MCP Server 作为子进程启动 |
| **SSE** | HTTP Server-Sent Events | 远程客户端，需要网络访问 |
| **Streamable HTTP** | HTTP POST + 可选 SSE 响应 | MCP 2025-03-26 新增，替代 SSE |

---

## 三、架构设计

### 3.1 整体架构

```
                        ┌─────────────────────────────────────┐
                        │          MCP 客户端                  │
                        │  Claude Desktop / Cursor / 自研 Agent │
                        └──────────┬──────────┬───────────────┘
                                   │          │
                          stdio    │          │  SSE / HTTP
                          (子进程)  │          │  (网络)
                                   │          │
                        ┌──────────▼──────────▼───────────────┐
                        │          MCP Server 模块             │
                        │  ┌───────────────────────────────┐  │
                        │  │  transport/                    │  │
                        │  │  ├── stdio.go   (stdin/stdout) │  │
                        │  │  └── sse.go     (HTTP+SSE)     │  │
                        │  ├───────────────────────────────┤  │
                        │  │  handler/                      │  │
                        │  │  └── retrieve.go (工具处理器)   │  │
                        │  ├───────────────────────────────┤  │
                        │  │  tools/                        │  │
                        │  │  └── rag_retrieve.go (工具定义) │  │
                        │  ├───────────────────────────────┤  │
                        │  │  auth/                         │  │
                        │  │  └── apikey.go  (认证适配)      │  │
                        │  └───────────────────────────────┘  │
                        └──────────────┬──────────────────────┘
                                       │ 内部调用
                                       │ (复用现有 retrieve 逻辑)
                        ┌──────────────▼──────────────────────┐
                        │     现有 RAG 检索服务                 │
                        │  ┌─────────┐  ┌─────────┐          │
                        │  │ 改写    │  │ 混合检索 │          │
                        │  │ Rewrite │  │ Hybrid  │          │
                        │  └────┬────┘  └────┬────┘          │
                        │       └──────┬─────┘               │
                        │         ┌────▼────┐                │
                        │         │ 融合+重排 │                │
                        │         │ Fusion   │                │
                        │         └────┬────┘                │
                        │         ┌────▼────┐                │
                        │         │ Milvus  │                │
                        │         └─────────┘                │
                        └─────────────────────────────────────┘
```

### 3.2 模块划分

```
backend/
├── internal/
│   └── mcp/                        # MCP Server 模块（新增）
│       ├── server.go               # MCP Server 核心：初始化、注册工具、启动传输
│       ├── server_test.go          # 单元测试
│       ├── config.go               # MCP 配置结构体
│       ├── transport/
│       │   ├── stdio.go            # stdio 传输实现
│       │   └── sse.go              # SSE 传输实现
│       ├── handler/
│       │   ├── retrieve.go         # rag_retrieve 工具处理器
│       │   └── retrieve_test.go    # 处理器单元测试
│       ├── tools/
│       │   └── definition.go       # 工具 schema 定义（输入/输出）
│       └── auth/
│           └── adapter.go          # MCP 认证适配：从参数/环境变量获取 API Key
├── cmd/
│   └── mcp-server/
│       └── main.go                 # MCP Server 独立入口（stdio 模式）
```

### 3.3 与现有代码的关系

| 组件 | 复用/新增 | 说明 |
|------|-----------|------|
| 检索逻辑 | **复用** | 直接调用 `internal/ragplatform/application.RetrieveService` |
| API Key 认证 | **复用** | 复用 `internal/auth` 包的 API Key 验证逻辑 |
| 多租户权限 | **复用** | 复用 `internal/service.KBPermissionService` |
| 检索日志 | **复用** | 检索请求自动进入现有日志链路 |
| MCP 协议层 | **新增** | 使用官方 Go SDK 实现 MCP Server |
| MCP 传输层 | **新增** | stdio + SSE 两种传输 |
| MCP 配置 | **新增** | 在 `config.yaml` 中增加 `mcp` 配置段 |

---

## 四、MCP 工具设计

### 4.1 工具定义

**工具名称**：`rag_retrieve`

**工具描述**：从 RAG 知识库中检索与查询相关的文档片段。支持指定知识库、调整返回数量、使用策略配置和元数据过滤。适用于需要从企业知识库中获取上下文信息以辅助回答问题的场景。

### 4.2 输入参数 Schema

```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "检索查询文本，描述需要查找的信息"
    },
    "kb_ids": {
      "type": "array",
      "items": { "type": "integer" },
      "description": "指定检索的知识库 ID 列表。不传则使用 API Key 绑定的默认知识库"
    },
    "top_k": {
      "type": "integer",
      "default": 5,
      "minimum": 1,
      "maximum": 20,
      "description": "返回结果数量，默认 5，最大 20"
    },
    "strategy_profile": {
      "type": "string",
      "default": "default",
      "description": "检索策略配置名称，如 default、aggressive、conservative"
    },
    "metadata_filter": {
      "type": "object",
      "description": "元数据过滤条件，如 {\"file_type\": \"pdf\", \"department\": \"engineering\"}"
    }
  },
  "required": ["query"]
}
```

### 4.3 输出格式

MCP 工具调用返回的是文本内容，需要将结构化的检索结果序列化为可读文本：

```text
检索结果（共 3 条，策略版本: baseline）:

[1] 相关度: 0.92 | 来源: jvm-tuning.md (知识库 #1, 文档 #10, 分块 #3)
内容: JVM 调优的核心在于合理配置堆内存大小、选择合适的垃圾回收器...

[2] 相关度: 0.85 | 来源: java-concurrency.md (知识库 #1, 文档 #15, 分块 #7)
内容: Java 并发编程中的线程池配置直接影响系统性能...

[3] 相关度: 0.78 | 来源: performance-guide.md (知识库 #2, 文档 #3, 分块 #1)
内容: 系统性能优化应从 CPU、内存、IO 三个维度综合考虑...
```

### 4.4 认证方式

MCP Server 复用现有 API Key 认证，通过以下方式传递：

| 方式 | 适用场景 | 说明 |
|------|----------|------|
| **环境变量** `RAG_API_KEY` | stdio 模式（Claude Desktop 配置） | 启动时注入，所有请求共享 |
| **工具参数** `api_key` | SSE 模式（多租户远程访问） | 每次调用时传递，支持不同租户 |

认证流程：

```
MCP Client 调用 rag_retrieve
    │
    ├── 检查工具参数中是否有 api_key
    │   ├── 有 → 使用该 key 验证
    │   └── 无 → 检查环境变量 RAG_API_KEY
    │       ├── 有 → 使用环境变量 key 验证
    │       └── 无 → 返回认证错误
    │
    └── 验证通过 → 调用现有检索逻辑
```

---

## 五、配置设计

### 5.1 config.yaml 新增配置段

```yaml
# MCP Server 配置
mcp:
  enabled: true                    # 是否启用 MCP Server
  transport:
    type: "both"                   # 传输方式：stdio | sse | both
    sse:
      host: "0.0.0.0"             # SSE 监听地址
      port: 8082                   # SSE 监听端口
      path: "/mcp"                 # SSE 端点路径
  auth:
    default_api_key: ""            # 默认 API Key（仅用于开发/测试）
    require_auth: true             # 是否强制要求认证
  tools:
    rag_retrieve:
      enabled: true                # 是否启用该工具
      max_top_k: 20                # top_k 最大值
      default_top_k: 5             # top_k 默认值
      timeout_ms: 5000             # 工具执行超时
  logging:
    level: "info"                  # 日志级别
    log_requests: true             # 是否记录每次 MCP 请求
```

### 5.2 环境变量覆盖

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `MCP_ENABLED` | 是否启用 MCP Server | `false` |
| `MCP_TRANSPORT_TYPE` | 传输类型 | `both` |
| `MCP_SSE_PORT` | SSE 端口 | `8082` |
| `RAG_API_KEY` | 默认 API Key（stdio 模式用） | 空 |

### 5.3 客户端配置示例

**Claude Desktop (`claude_desktop_config.json`)**：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "command": "/path/to/rag-mcp-server",
      "args": [],
      "env": {
        "RAG_API_KEY": "your-api-key-here",
        "CONFIG_PATH": "/path/to/config.yaml"
      }
    }
  }
}
```

**Cursor (`.cursor/mcp.json`)**：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "command": "/path/to/rag-mcp-server",
      "env": {
        "RAG_API_KEY": "your-api-key-here"
      }
    }
  }
}
```

**SSE 远程接入**：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "url": "http://your-rag-server:8082/mcp"
    }
  }
}
```

---

## 六、安全设计

### 6.1 认证与授权

| 安全措施 | 说明 |
|----------|------|
| API Key 认证 | 复用现有 API Key 验证（SHA256 哈希、过期检查、撤销检查） |
| 租户隔离 | API Key 绑定租户，检索时自动校验知识库权限 |
| 最小权限 | MCP 工具只暴露检索能力，不暴露管理操作 |

### 6.2 输入验证

| 验证项 | 规则 |
|--------|------|
| query | 必填，非空，最大长度 1000 字符 |
| kb_ids | 可选，每个 ID 必须是正整数 |
| top_k | 可选，范围 [1, 20] |
| strategy_profile | 可选，白名单校验 |
| metadata_filter | 可选，深度限制 3 层 |

### 6.3 速率限制

复用现有 Hertz 框架的速率限制中间件，MCP 请求与 HTTP API 共享配额。

### 6.4 网络安全

| 场景 | 措施 |
|------|------|
| stdio 模式 | 本地通信，无网络暴露 |
| SSE 模式 | 建议部署在内网，通过反向代理暴露；支持 TLS |

---

## 七、监控与可观测性

### 7.1 复用现有监控

MCP 请求走现有检索链路，自动享有：

- **检索日志**：`kb_retrieve_log` 表记录每次检索
- **成本追踪**：按知识库/应用/策略归因
- **Prometheus 指标**：`rag_metrics.go` 中的检索指标

### 7.2 新增 MCP 特定指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `mcp_requests_total` | Counter | MCP 工具调用总数 |
| `mcp_request_duration_seconds` | Histogram | MCP 请求耗时 |
| `mcp_errors_total` | Counter | MCP 错误总数（按错误类型分） |
| `mcp_active_connections` | Gauge | 当前活跃的 SSE 连接数 |

### 7.3 结构化日志

```json
{
  "level": "info",
  "ts": "2026-06-10T10:30:00Z",
  "caller": "mcp/handler/retrieve.go:45",
  "msg": "mcp tool call",
  "tool": "rag_retrieve",
  "transport": "stdio",
  "tenant_id": 1,
  "query": "什么是 JVM 调优",
  "kb_ids": [1, 2],
  "top_k": 5,
  "result_count": 3,
  "latency_ms": 120,
  "request_id": "uuid-xxx"
}
```

---

## 八、错误处理

### 8.1 错误分类

| 错误码 | MCP Error Type | 场景 | 用户提示 |
|--------|----------------|------|----------|
| -32600 | Invalid Request | 参数格式错误 | 参数格式不正确，请检查 query 是否为非空字符串 |
| -32601 | Method Not Found | 调用了不存在的工具 | 工具不存在，可用工具: rag_retrieve |
| -32602 | Invalid Params | 参数值不合法 | top_k 必须在 1-20 之间 |
| -32001 | Unauthorized | API Key 无效/缺失 | 请提供有效的 RAG API Key |
| -32002 | Forbidden | 无权访问指定知识库 | 无权访问知识库 #kb_id |
| -32003 | Resource Not Found | 知识库不存在 | 知识库 #kb_id 不存在 |
| -32004 | Timeout | 检索超时 | 检索请求超时，请稍后重试 |
| -32005 | Internal Error | 内部服务异常 | 服务内部错误，请联系管理员 |

### 8.2 错误响应格式

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32001,
    "message": "Unauthorized",
    "data": {
      "detail": "API Key 无效或已过期",
      "request_id": "uuid-xxx"
    }
  }
}
```

---

## 九、分阶段实施计划

### Phase 1：基础 MCP Server（stdio 传输）

**目标**：验证 MCP 协议集成可行性，支持 Claude Desktop / Cursor 本地接入

**范围**：
- 实现 MCP Server 核心（使用官方 Go SDK）
- 实现 `rag_retrieve` 工具定义和处理器
- 实现 stdio 传输
- 通过环境变量传递 API Key
- 基本错误处理和日志
- 独立可执行文件 `rag-mcp-server`

**验收标准**：
- Claude Desktop 配置后可以调用 `rag_retrieve` 工具
- 返回的检索结果与 HTTP API `/v1/retrieve` 一致
- 认证、权限校验正常工作

### Phase 2：SSE 传输 + 集成部署

**目标**：支持远程接入，可集成到现有 RAG 服务

**范围**：
- 实现 SSE 传输
- MCP Server 集成到现有 RAG 服务（可选启用）
- 配置文件支持（`config.yaml` 增加 `mcp` 段）
- 环境变量覆盖支持
- Prometheus 监控指标
- 结构化日志

**验收标准**：
- SSE 模式下远程客户端可以接入
- `config.yaml` 配置生效
- Grafana 可以看到 MCP 相关指标

### Phase 3：企业级加固

**目标**：满足生产环境安全和运维要求

**范围**：
- Streamable HTTP 传输支持（MCP 最新规范）
- 速率限制（与现有 HTTP API 共享配额）
- 健康检查端点
- 端到端测试
- 完整文档和配置示例
- Docker 镜像支持

**验收标准**：
- 端到端测试覆盖所有工具和传输方式
- 生产环境安全审计通过
- 文档完整，新用户可在 10 分钟内完成接入

---

## 十、目录结构与代码边界

### 10.1 新增文件清单

```
backend/
├── cmd/
│   └── mcp-server/
│       └── main.go                    # [新增] MCP Server 独立入口（stdio 模式）
├── internal/
│   └── mcp/                           # [新增] MCP 模块
│       ├── server.go                  # Server 初始化、工具注册、生命周期管理
│       ├── server_test.go
│       ├── config.go                  # MCPConfig 结构体和加载逻辑
│       ├── handler/
│       │   ├── retrieve.go            # rag_retrieve 工具处理器
│       │   └── retrieve_test.go
│       ├── tools/
│       │   └── definition.go          # 工具 schema 定义
│       └── auth/
│           └── adapter.go             # 认证适配器
```

### 10.2 修改文件清单

| 文件 | 修改内容 |
|------|----------|
| `internal/config/config.go` | 新增 `MCPConfig` 结构体，`Config` 中增加 `MCP` 字段 |
| `config.yaml` / `config.example.yaml` | 新增 `mcp` 配置段 |
| `cmd/rag-server/main.go` | 可选：在主进程中启动 MCP Server |
| `deploy/monitoring/prometheus/prometheus.yml` | 可选：新增 MCP 指标抓取配置 |

### 10.3 不修改的文件

| 文件 | 原因 |
|------|------|
| `api/handler/rag/retrieve.go` | MCP 模块通过内部 service 层调用，不修改 HTTP handler |
| `api/ragrouter/register.go` | MCP 路由独立于 HTTP 路由 |
| `internal/milvus/retrieval/*` | 检索核心逻辑不修改 |
| `internal/auth/*` | 认证逻辑不修改，MCP 模块作为调用方复用 |
| `admin/*` | 前端不受影响 |

---

## 十一、协议兼容性

### 11.1 MCP 规范版本

| 规范版本 | 发布日期 | 支持状态 |
|----------|----------|----------|
| 2024-11-05 | 2024-11 | 完全支持（基线版本） |
| 2025-03-26 | 2025-03 | 支持 Streamable HTTP |
| 未来版本 | - | 官方 SDK 更新后跟进 |

### 11.2 向后兼容策略

- MCP 工具名称、参数、返回格式在版本锁定后不做破坏性变更
- 如需变更，通过版本化工具名称（如 `rag_retrieve_v2`）实现平滑迁移
- MCP 协议本身支持版本协商，Server 会自动适配不同版本的 Client

---

## 十二、性能设计

### 12.1 性能目标

| 指标 | 目标值 | 说明 |
|------|--------|------|
| MCP 协议解析开销 | < 1ms | JSON-RPC 解析和序列化 |
| 端到端延迟 | 与 HTTP API 一致 | 检索逻辑完全相同 |
| 并发连接数 | 与 HTTP API 一致 | 共享速率限制配额 |

### 12.2 性能优化

| 优化项 | 说明 |
|--------|------|
| 连接池复用 | SSE 模式下复用数据库和 Milvus 连接池 |
| 零拷贝 | MCP 请求解析后直接构造检索请求，避免中间复制 |
| 异步日志 | MCP 请求日志异步写入，不阻塞响应 |

---

## 十三、风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 官方 Go SDK 不稳定 | 可能需要频繁更新依赖 | 锁定 SDK 版本，定期评估更新 |
| MCP 协议变更 | 可能需要适配新版本 | 使用官方 SDK 的版本协商机制 |
| stdio 模式下的进程管理 | 子进程崩溃影响客户端 | 实现优雅退出和 panic 恢复 |
| SSE 模式下的连接泄漏 | 资源耗尽 | 实现连接超时和心跳检测 |
| 恶意 MCP 客户端 | 资源滥用 | 速率限制 + 输入验证 + 认证 |

---

## 十四、附录

### A. MCP 协议核心概念

| 概念 | 说明 |
|------|------|
| **Tool** | 暴露给 LLM 调用的函数，有名称、描述、输入 schema |
| **Resource** | 暴露给 LLM 读取的数据源（本次不实现） |
| **Prompt** | 预定义的提示模板（本次不实现） |
| **Server** | 提供 Tools/Resources/Prompt 的服务端 |
| **Client** | 发现和调用 Server 的客户端 |
| **Transport** | Server 和 Client 之间的通信方式 |

### B. 与现有 Agent 接入方式的对比

| 维度 | HTTP API | Go SDK | MCP Server |
|------|----------|--------|------------|
| 接入成本 | 需要编写 HTTP 调用代码 | 需要引入 Go 依赖 | 零代码，配置即用 |
| 客户端支持 | 任何语言 | 仅 Go | 所有 MCP 客户端 |
| 工具发现 | 需要阅读文档 | 需要阅读文档 | 自动发现（`tools/list`） |
| 认证方式 | Header Bearer Token | SDK 配置 | 环境变量/参数 |
| 适用场景 | 自研 Agent | Go Agent | AI IDE / 通用 Agent |

### C. 参考资料

- [MCP 官方规范](https://modelcontextprotocol.io/specification)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [MCP 规范版本历史](https://modelcontextprotocol.io/specification/2025-03-26)
