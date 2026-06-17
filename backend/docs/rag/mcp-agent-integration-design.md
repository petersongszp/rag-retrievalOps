# RAG Platform MCP Agent 接入设计稿

## 1. 文档目的

本文用于评审“是否应为 RAG Platform 提供 MCP 接入层，以及应该如何落地”。

本文重点回答以下问题：

1. 外部项目的 Agent 能否通过 MCP 接入当前 RAG Platform。
2. MCP 在整体架构里应该承担什么职责，不应该承担什么职责。
3. 如何在不重写现有检索链路的前提下，快速落地一个可用、可监控、可灰度的 MCP Server。
4. MCP 接入后，如何复用当前平台已有的 API Key、租户隔离、知识库权限、检索日志、调试视图和监控能力。

一句话结论：

> 可以做，而且值得做；但 MCP 应作为外部 Agent 的协议适配层，而不是新的检索能力实现层。底层能力仍应统一收敛到现有 `POST /v1/retrieve`。

---

## 2. 背景与现状

当前项目已经具备面向外部业务的基础平台能力：

1. 对外检索接口：`POST /v1/retrieve`
2. Agent / 服务端鉴权方式：API Key
3. 多租户与知识库权限校验
4. 检索日志、调试视图、审计日志、Prometheus 指标
5. Go SDK 和 HTTP 接入方式

当前项目尚未具备的能力：

1. 原生 MCP Server 实现
2. 面向 MCP Client 的工具发现与工具契约
3. 外部 Agent 会话 ID 与平台 `request_id` 的统一映射规范
4. 面向 MCP 维度的独立指标和灰度发布策略

因此，本设计稿的目标不是新增第二套检索系统，而是在现有公开 API 之上补一层 MCP 协议适配。

---

## 3. 设计目标与非目标

## 3.1 设计目标

1. 让支持 MCP 的外部 Agent 可以零侵入或低侵入接入本平台。
2. 保持现有平台能力边界稳定，避免 MCP Server 复制检索逻辑。
3. 继续复用 API Key、租户隔离、知识库授权和检索日志体系。
4. 支持按 `app_id`、`agent_name`、`scene`、`external_session_id` 对请求进行追踪和监控。
5. 支持后续扩展更多工具，而不仅限于检索。

## 3.2 非目标

1. 不在 Phase 1 重写 `api/handler/rag/retrieve.go` 的业务逻辑。
2. 不在 Phase 1 把 MCP 直接接到内部 `milvus` 或 `kb` handler。
3. 不在 Phase 1 让终端用户直接持有 API Key。
4. 不在 Phase 1 实现完整 MCP 生态下的所有资源、prompt、streaming 能力。
5. 不在 Phase 1 替代现有 HTTP SDK 接入方式。

---

## 4. 核心结论

推荐架构如下：

```text
外部 Agent / IDE / Workflow
  -> MCP Client
  -> RAG MCP Server
  -> RAG Platform HTTP API (/v1/retrieve 等)
  -> 检索日志 / 调试 / 审计 / 指标
```

职责边界固定如下：

1. RAG Platform 负责真实检索能力、鉴权、权限校验、日志、调试、监控。
2. MCP Server 负责协议适配、参数校验、工具暴露、上下文透传、错误映射。
3. 外部 Agent 负责自身工作流编排、Prompt 组装、最终回答生成。

不推荐架构：

```text
外部 Agent
  -> MCP Server
  -> 直接调用内部 milvus / retrieval 包
```

原因：

1. 会绕过现有 API Key 与租户权限边界。
2. 会绕过现有 `request_id`、检索日志和调试视图。
3. 会让 MCP Server 重新承担平台内部实现细节，后续维护成本高。

---

## 5. 总体架构

## 5.1 逻辑分层

建议分成 4 层：

```text
Layer 1: MCP Protocol Layer
  - 初始化 MCP Server
  - tool 注册
  - schema 输出
  - 错误映射

Layer 2: MCP Application Layer
  - 请求上下文组装
  - 参数归一化
  - 观测字段注入
  - 调用策略控制

Layer 3: RAG HTTP Client Layer
  - 调用 /v1/retrieve
  - 处理 API Key
  - 处理 timeout / retry / error

Layer 4: RAG Platform
  - 检索能力
  - 授权校验
  - request_id 生成
  - 日志 / 指标 / 调试
```

## 5.2 部署形态

建议支持两种部署形态：

### 形态 A：独立 MCP Gateway

```text
外部 Agent -> MCP Gateway -> RAG Platform
```

适用场景：

1. 多个外部 Agent 共用一个 MCP 出口
2. 希望统一管理工具定义、鉴权映射、审计扩展
3. 希望将协议适配与平台核心进程解耦

### 形态 B：平台内嵌 MCP Server

```text
外部 Agent -> 内嵌 MCP Server -> 同进程 RAG Platform API
```

适用场景：

1. 单体部署、快速 PoC
2. 本地演示或私有化轻量交付

推荐顺序：

1. Phase 1 先做独立 MCP Gateway
2. 如后续确有部署简化需求，再评估内嵌形态

---

## 6. 接口设计

## 6.1 Phase 1 必备工具

### Tool 1: `rag.retrieve`

用途：执行知识库检索，返回证据和 `request_id`。

输入建议：

```json
{
  "query": "什么是 JVM 调优？",
  "kb_ids": [1, 2],
  "top_k": 5,
  "strategy_profile": "default",
  "metadata_filter": {},
  "scene": "interview",
  "agent_name": "interview-agent",
  "external_session_id": "session-123",
  "external_request_id": "turn-7"
}
```

输出建议：

```json
{
  "request_id": "uuid",
  "items": [
    {
      "content": "...",
      "score": 0.91,
      "citation": {
        "kb_id": 1,
        "document_id": 10,
        "chunk_id": "doc-10-child-003",
        "file_name": "jvm-tuning.md",
        "chunk_index": 3
      },
      "source": {
        "route": "hybrid",
        "collection": "kb_1_docs",
        "retriever_version": "hybrid-v1"
      }
    }
  ],
  "count": 1,
  "agent_context": {
    "scene": "interview",
    "agent_name": "interview-agent",
    "external_session_id": "session-123"
  }
}
```

说明：

1. `request_id` 必须原样返回给外部 Agent。
2. `count` 由 MCP Server 补充，便于通用 Agent 使用。
3. `agent_context` 用于回显透传字段，便于双端排障。

### Tool 2: `rag.list_kbs`

用途：列出当前 API Key 或当前映射应用可访问的知识库。

输入建议：

```json
{
  "keyword": "java",
  "page": 1,
  "page_size": 20
}
```

输出建议：

```json
{
  "items": [
    {
      "kb_id": 1,
      "name": "Java 基础知识库",
      "description": "...",
      "permission": "read"
    }
  ],
  "total": 1
}
```

说明：

1. 如果平台尚无稳定公开的 KB 列表接口，Phase 1 可暂缓该工具。
2. 如暂缓，MCP Server 可以配置静态 `default_kb_ids` 作为过渡方案。

### Tool 3: `rag.get_trace`

用途：按 `request_id` 获取检索调试摘要。

输入建议：

```json
{
  "request_id": "uuid"
}
```

输出建议：

```json
{
  "request_id": "uuid",
  "debug_available": true,
  "original_query": "什么是 JVM 调优？",
  "rewritten_query": "JVM tuning",
  "route_hits": [
    { "route": "dense", "contribution": 2 },
    { "route": "sparse", "contribution": 1 }
  ],
  "topk_decision": {
    "candidate_topk": 10,
    "final_topk": 5
  },
  "evidence_gate": {
    "evidence_gate_result": "pass"
  }
}
```

说明：

1. 该工具主要服务开发调试，不建议默认暴露给所有生产 Agent。
2. 可通过 MCP Server 配置控制是否启用。

---

## 7. 认证与授权设计

## 7.1 认证原则

MCP Server 不应替代平台认证，只应承接认证材料并转发到平台。

推荐原则：

1. MCP Server 到 RAG Platform 使用 API Key。
2. API Key 只保存在 MCP Server 或可信服务端，不下发给终端用户。
3. 外部 Agent 不直接感知平台内部 JWT。
4. 新接入禁止依赖 legacy `app_id` 白名单。

## 7.2 推荐认证模式

### 模式 A：单业务单 Key

```text
support-agent -> MCP Server -> Bearer rag_xxx_support
interview-agent -> MCP Server -> Bearer rag_xxx_interview
```

优点：

1. 边界清晰
2. 便于按业务审计与限流
3. 出问题时易隔离

缺点：

1. Key 管理数量更多

### 模式 B：MCP Gateway 统一 Key + 应用路由

```text
多个 Agent -> MCP Gateway -> 统一平台凭证 -> 内部映射 app_id
```

优点：

1. 外部接入简单
2. 统一治理方便

缺点：

1. 如果平台侧只看到一个 API Key，业务归因会变粗
2. 必须额外记录 `agent_name`、`scene`、`external_session_id`

推荐：

1. 生产优先模式 A
2. 内部 PoC 或演示环境可接受模式 B

## 7.3 授权边界

权限校验继续由平台完成：

1. `tenant_id` 隔离
2. `app_id -> kb_ids` 授权
3. `top_k` 上限
4. QPS / 配额 / timeout

MCP Server 不应单独维护一份最终授权逻辑，只能维护轻量缓存或静态兜底配置。

---

## 8. 可观测性与监控设计

## 8.1 设计原则

MCP 接入成功的标准，不只是“能调通”，而是“出了问题能定位到具体业务、具体 Agent、具体会话、具体平台 request”。

## 8.2 必须透传的观测字段

建议在 MCP Server 内部统一生成或透传以下字段：

1. `app_id`
2. `agent_name`
3. `scene`
4. `external_session_id`
5. `external_request_id`
6. `request_id`（平台返回）
7. `kb_ids`
8. `top_k`
9. `duration_ms`
10. `result_count`

## 8.3 观测链路

建议形成如下映射：

```text
外部 Agent session_id/turn_id
  <-> MCP log: external_session_id/external_request_id
  <-> RAG Platform: request_id
  <-> Retrieval Debug / Trace Logs / Metrics
```

## 8.4 平台侧最少要支持的观测能力

1. 检索日志按 `request_id` 查询
2. 检索日志按 `app_id` 聚合
3. 调试视图按 `request_id` 打开
4. Prometheus 指标按 `app_id` 或等价业务维度拆分
5. 审计日志能关联关键操作与请求来源

## 8.5 MCP 侧新增指标建议

建议新增：

1. `mcp_tool_requests_total{tool,status,agent_name,scene}`
2. `mcp_tool_duration_seconds{tool,agent_name,scene}`
3. `mcp_upstream_errors_total{tool,error_code}`
4. `mcp_request_bridge_total{app_id,agent_name}`

用途：

1. 区分是 MCP 层报错还是平台层报错
2. 识别某个 Agent 是否存在异常调用模式
3. 识别 MCP 协议适配是否成为瓶颈

---

## 9. 错误处理设计

## 9.1 错误分层

建议把错误分为三层：

1. 协议错误：MCP tool 入参不合法、缺少必填字段
2. 网关错误：MCP Server 到平台超时、网络失败、序列化失败
3. 业务错误：平台返回 `401/403/404/429/5xx`

## 9.2 错误映射原则

1. 不吞掉平台错误
2. 不把所有错误都包装成“空结果”
3. 对超时、权限不足、无效 API Key 提供稳定错误码
4. 如果平台返回了 `request_id`，即使失败也应尽量回传给调用方

建议错误映射：

1. `401 invalid_api_key` -> 上游认证失败
2. `401 api_key_revoked` -> 凭证已吊销
3. `401 api_key_expired` -> 凭证已过期
4. `403 forbidden` -> 业务未授权访问目标知识库
5. `404 not_found` -> 知识库不存在或跨租户不可见
6. `429 rate_limited` -> 平台限流
7. `504 upstream_timeout` -> 平台或网关超时

---

## 10. 实施路线

## 10.1 Phase 1：最小可用 MCP 检索接入

目标：

> 让一个外部 Agent 能通过 MCP 调通 `rag.retrieve`，并且平台侧能按 `request_id` 追踪。

任务：

1. 新增独立 `cmd/mcp-server` 或独立轻量服务目录。
2. 实现 `rag.retrieve`。
3. MCP Server 底层调用现有 `/v1/retrieve`。
4. 平台 API Key 通过环境变量注入。
5. MCP 日志记录 `agent_name/scene/external_session_id/request_id`。
6. 输出本地运行说明和最小示例。

验收标准：

1. MCP Client 能成功发现 `rag.retrieve`。
2. 检索成功时返回 `items + request_id`。
3. 平台侧 Trace Logs 能查到该请求。
4. 平台不可用时，外部 Agent 得到明确错误，不静默吞掉。

## 10.2 Phase 2：可运维与可调试

目标：

> 让 MCP 不只是能用，而且便于排障和业务归因。

任务：

1. 新增 `rag.get_trace`。
2. 接入 MCP 自身 Prometheus 指标。
3. 统一日志格式，固定 `external_session_id <-> request_id` 映射。
4. 支持按 `agent_name/scene` 做聚合指标。
5. 支持按配置开启或关闭调试工具暴露。

验收标准：

1. 任意一次生产问题都能从外部 Agent 日志追到平台 `request_id`。
2. 可以区分是 MCP 层故障还是平台层故障。
3. 可以统计不同 Agent 的调用量、错误率和 P95。

## 10.3 Phase 3：多工具平台化

目标：

> 让 MCP 成为正式的企业接入层，而不只是单工具桥接器。

任务：

1. 评估增加 `rag.list_kbs`、`rag.get_trace`、`rag.ingest_document` 等工具。
2. 支持按业务定制工具暴露白名单。
3. 支持更细的 `strategy_profile` 与业务默认 KB 配置。
4. 支持多平台客户端接入文档。
5. 支持插件化部署或与 Demo Space 集成。

---

## 11. 代码落点建议

如在当前仓库内实现，建议新增：

```text
backend/
  cmd/
    mcp-server/
      main.go
  internal/
    mcp/
      server.go
      tools/
        retrieve_tool.go
        trace_tool.go
      transport/
        stdio.go
        http.go
      observability/
        metrics.go
      client/
        rag_client.go
      config/
        config.go
```

设计原则：

1. `internal/mcp/client` 只依赖 RAG 公共 API，不依赖内部 `milvus`。
2. `internal/mcp/tools` 只做工具编排和参数转换。
3. `cmd/mcp-server` 只负责启动与依赖装配。
4. 所有平台地址、API Key、默认 KB、工具开关通过配置注入。

---

## 12. 风险与权衡

## 12.1 主要收益

1. 对支持 MCP 的 Agent 生态更友好。
2. 降低外部业务接入门槛。
3. 不破坏现有 HTTP / SDK 路线。
4. 有利于后续形成“Webhook + MCP + SDK + Agent Tool”统一产品形态。

## 12.2 主要风险

1. 如果把授权逻辑复制到 MCP Server，后续会出现双份权限边界。
2. 如果不透传外部会话字段，问题排查仍然困难。
3. 如果把 MCP Server 直连内部检索实现，会破坏平台边界。
4. 如果平台侧日志暂时无法按 `app_id` 细分，业务归因粒度仍然不足。

## 12.3 核心权衡

最关键权衡不是“做不做 MCP”，而是“让 MCP 成为适配层，还是成为另一套平台入口实现”。

本设计明确选择前者：

> MCP 是接入层，不是能力层；`/v1/retrieve` 才是平台真实能力入口。

---

## 13. 开放问题

在正式立项前，建议先确认以下问题：

1. Phase 1 的 MCP Server 是独立部署，还是内嵌到当前服务进程。
2. 平台是否已经具备稳定可公开的 KB 列表接口，决定 `rag.list_kbs` 是否首批上线。
3. 平台日志是否已经稳定记录 `tenant_id/app_id/api_key_id/request_id`，决定业务归因粒度。
4. 目标外部 Agent 生态优先支持哪一类客户端，决定优先做 `stdio` 还是 `http` transport。
5. 调试工具是否默认对生产租户开放，还是仅对内部或管理员开放。

---

## 14. 建议决策

建议按以下决策推进：

1. 立项 MCP，但定位为“协议接入层”。
2. Phase 1 只做 `rag.retrieve`，先把最小价值闭环跑通。
3. 生产接入统一使用 API Key，不再为 MCP 单开 legacy 鉴权口子。
4. 把 `external_session_id/request_id` 映射列为 MCP 验收必选项，而不是可选项。
5. 在平台侧先补齐按 `app_id` 聚合日志和指标，再逐步扩展更多 MCP 工具。

如果以上方向成立，下一步产出应为：

1. `cmd/mcp-server` 最小 PoC
2. `rag.retrieve` Tool Schema
3. MCP 配置样例
4. 接入联调清单
