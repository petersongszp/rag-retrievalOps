# 04-MCP 协议接入-面试题和参考答案

> 面向场景：基于 MCP 协议封装 RAG 检索能力，复用现有 API Key、租户隔离、检索日志体系，支持 Claude Desktop / Cursor 等 AI 客户端原生互操作，并按 agent 维度追踪监控与灰度发布。

---

## 面试官提问路径

```text
"你们这个 MCP 接入是怎么设计的？"
    ↓
"为什么选择在 /v1/retrieve 上面封一层，而不是 MCP 直接打 Milvus？"
    ↓
"工具的 Schema 和参数校验是怎么做的？top_k、kb_ids 这些边界怎么定的？"
    ↓
"Authorization 是怎么从 MCP 透传到 RAG 平台、又是怎么落到日志里的？"
    ↓
"HTTP 模式下 Origin、CORS、Bearer 这套安全策略是怎么联动的？"
    ↓
"agent 维度的监控指标和灰度发布是怎么实现的？"
    ↓
"线上 MCP 调用突然大面积 401/超时，你怎么排？"
```

---

## Q1：你们 MCP 接入这块整体是怎么设计的？为什么这么分层？

**我的回答：**

核心结论一句话：MCP 在我们项目里**只做协议适配层，不做检索实现层**。所有真正的检索、鉴权、日志、灰度都收敛在 `POST /v1/retrieve`，MCP Server 只是把 `tools/call` 翻译成 HTTP 调用再把响应翻译回 MCP `CallToolResult`。

代码上我们拆了 4 个目录：`tools/` 出工具定义、`handler/` 出业务校验和调用编排、`client/` 出 RAG HTTP 客户端、`transport/` 出 stdio 和 http 两套传输。入口在 [server.go](../../backend/internal/mcp/server.go#L16-L38)，只有 38 行，干的事就是 `NewServer` + `AddTool`，把 `retrieve_knowledge` 工具挂上去。

**为什么坚持不直接调 `internal/milvus`**：第一会绕过现有 API Key 校验和 `kb_permission` 这层授权；第二会让 `request_id`、`kb_retrieve_log`、Prometheus 指标全部要重新做一遍；第三 MCP Server 就变成第二套检索实现，长期维护会爆炸。所以我们**强制走公网 HTTP 接口**，宁可多一跳网络，也要保证治理一致。

部署上提供两种形态：独立 MCP Gateway（推荐生产用）和内嵌 MCP（PoC 用）。我们生产是独立部署的，对应 [Dockerfile.mcp](../../backend/Dockerfile.mcp)，跑在 8898 端口。

> 相关代码：[server.go](../../backend/internal/mcp/server.go)、[cmd/rag-mcp-server/main.go](../../backend/cmd/rag-mcp-server/main.go)、[mcp-agent-integration-design.md](../../backend/docs/rag/mcp-agent-integration-design.md)

### 🔍 深挖追问兜底

> **Q：为什么不在 RAG 主进程里直接挂一个 MCP endpoint？**
> 主进程是 Hertz HTTP 框架，MCP 用的是 `github.com/modelcontextprotocol/go-sdk`，连接是 Streamable HTTP + Stateless 会话语义，混进 Hertz 路由会污染中间件链。独立进程也方便单独灰度和回滚，挂了不影响主检索。

> **Q：内嵌形态和独立形态怎么选？**
> 单体演示、私有化轻量交付用内嵌；多 Agent 共用、要统一审计就用独立 Gateway。我们生产走独立，开发联调可以 `--transport stdio` 直接给 Claude Desktop 用。

> **Q：MCP Server 自己挂了会怎么样？**
> 主链路 `/v1/retrieve` 完全不受影响，Web、SDK、Legacy app_id 通道都正常。MCP 只是新通道。健康检查见 [transport/http.go#L83-L106](../../backend/internal/mcp/transport/http.go#L83-L106) 里的 `/readyz`，它会主动探活上游 RAG 的 `/readyz`。

---

## Q2：`retrieve_knowledge` 这个工具的 Schema 和参数校验怎么定的？为什么这么定？

**我的回答：**

工具定义在 [tools/definition.go](../../backend/internal/mcp/tools/definition.go)，对外只暴露一个工具 `retrieve_knowledge`，描述是中文的，Claude/Cursor 看到 description 才知道"这是用来检索知识证据的"。

InputSchema 我们做了三件事：

第一，**字段一律收口到 RAG 平台的 V1 契约**，`query`、`kb_ids`、`kb_id`、`top_k`、`strategy_profile`、`metadata_filter`，不引入 MCP 专属字段，这样未来切回 HTTP SDK 不用改后端。

第二，**关键边界写死在 schema 里**：`query` `maxLength: 2000`、`kb_ids` `maxItems: 100`、`top_k` `minimum: 1 / maximum: 20`。这些值是和 RAG 后端对齐的，见 [handler/retrieve.go#L17-L24](../../backend/internal/mcp/handler/retrieve.go#L17-L24) 里 `maxTopK = 20`、`maxQueryRunes = 2000`、`maxKBIDs = 100`、`maxMetadataFilterBytes = 16KB`、`maxMetadataFilterDepth = 8`。**Schema 是第一道闸，Handler 是第二道闸**，schema 挡掉 Claude 这种规矩客户端，handler 挡掉伪造请求。

第三，`metadata_filter` 用 `map[string]interface{}` 接，校验时序列化一次拿到字节数，再递归算嵌套深度，超 16KB 或超 8 层就拒。这是为了防止 Agent 把奇怪的大对象塞进来打挂 Milvus。

> 相关代码：[tools/definition.go](../../backend/internal/mcp/tools/definition.go)、[handler/retrieve.go#L109-L177](../../backend/internal/mcp/handler/retrieve.go#L109-L177)

### 🔍 深挖追问兜底

> **Q：为什么同时提供 `kb_id` 和 `kb_ids`？不冗余吗？**
> `kb_id` 是兼容字段，外部老 Agent 习惯传单个；归一化时把它合并进 `kb_ids` 集合并去重，见 `normalizeInput` 里的 `seen` map。

> **Q：`metadata_filter` 嵌套深度为什么是 8？**
> 经验值。线上观察用户 filter 一般 2-3 层，留 8 层给极端 OR/AND 嵌套场景；再深基本就是构造攻击或写错了，直接拒比让 Milvus 慢慢报错友好。

> **Q：top_k 上限 20，业务想要 50 怎么办？**
> 走 RAG 平台 dynamic_topk 策略，由后端基于 token_budget 决定，MCP 这层不松口。Agent 想要更多就调多次或换 `strategy_profile`。

---

## Q3：Authorization 是怎么从 MCP Client 透传到 RAG 平台、最后又落到日志的？

**我的回答：**

这是整个项目里最关键的一条链。一共三跳：

**第一跳：MCP Client → MCP Server。** HTTP 模式下我们在 [transport/http.go#L64-L66](../../backend/internal/mcp/transport/http.go#L64-L66) 套了 `mcpauth.RequireBearerToken`，强制带 `Authorization: Bearer rag_xxx`。但我们用的是 `passThroughTokenVerifier`，**不在 MCP 层验 Token 真伪**，只检查"不为空"——真伪交给 RAG 平台，避免双重校验逻辑漂移。

**第二跳：MCP Server → RAG 平台。** 在 [server.go#L47-L60](../../backend/internal/mcp/server.go#L47-L60) 的 `RetrieverFactoryFromConfig` 里，对每个 `CallToolRequest` 从 `req.Extra.Header` 拿到 Authorization，**原封不动**作为 HTTP 客户端的 Authorization 透传给 `/v1/retrieve`。stdio 模式下没有请求头，就 fallback 到环境变量 `RAG_ACCESS_TOKEN`。

**第三跳：RAG 平台落库。** RAG 平台 [api/handler/rag/retrieve.go#L205-L240](../../backend/api/handler/rag/retrieve.go#L205-L240) 识别 `Bearer rag_` 前缀走 API Key 分支，查 `rag_api_key` 表拿到 `tenant_id`、`user_id`、`app_id`，注入 `Identity` 上下文。最后写 `kb_retrieve_log` 时 `tenant_id`、`app_id`、`api_key_id`、`auth_type`、`source_api` 这五个字段都会落，见 [kb_retrieve_log.go#L122-L128](../../backend/internal/model/kb_retrieve_log.go#L122-L128)。

**反直觉的点**：MCP 自己不存任何租户信息，**完全无状态**，会话级别的 `tenant_id` 推断全靠 API Key 在 RAG 侧做。这样 MCP Server 可以横向扩多副本，不用同步状态。

> 相关代码：[server.go#L47-L60](../../backend/internal/mcp/server.go#L47-L60)、[transport/http.go#L108-L118](../../backend/internal/mcp/transport/http.go#L108-L118)、[middleware/auth.go#L138-L221](../../backend/internal/middleware/auth.go#L138-L221)

### 🔍 深挖追问兜底

> **Q：MCP 层不验 Token，会不会有人乱发请求打爆 RAG？**
> `RequireBearerToken` 至少会拒掉空 Token；剩下的让 RAG 的 `APIKeyAuth` 中间件统一处理，错的会 401。如果担心刷量，可以在 MCP 前再加 nginx/api-gateway 限流，但这不是 MCP 这一层的职责。

> **Q：日志里怎么知道这条请求是从 MCP 来的，不是 SDK 来的？**
> 看 `source_api` 字段。SDK 走 `/v1/retrieve` 是 `v1`，MCP 也走 `/v1/retrieve` 所以也是 `v1`，但 `app_id` 通常是 Agent 在创建 API Key 时填的（比如 `claude-desktop`），加上 HTTP transport 那一层会打 `auth=sha256:xxxx` 的指纹日志（[transport/http.go#L120-L138](../../backend/internal/mcp/transport/http.go#L120-L138)），两边一关联就能区分。

> **Q：Authorization 头会不会泄漏到日志？**
> 不会。`security/redact.go` 的 `AuthorizationFingerprint` 只取 SHA256 前 4 字节十六进制（共 8 字符），既能在日志里关联同一个 Key 的请求，又不可逆推。

---

## Q4：为什么 prod 环境强制 `MCP_TRANSPORT=http`，还强制 `MCP_ALLOWED_ORIGINS`？这两个参数的取舍是什么？

**我的回答：**

这两个限制写在 [config.go#L96-L103](../../backend/internal/mcp/config.go#L96-L103) 的 `Validate()` 里，prod 环境只要违反就直接启动失败。

**`stdio` 在 prod 禁掉**是因为 stdio 模式没有 HTTP Header，Authorization 只能靠环境变量 `RAG_ACCESS_TOKEN`，相当于**一个进程一个固定 Token**，没法做多 Agent 隔离、没法动态轮换。stdio 是给开发者本地把 MCP Server 挂到 Claude Desktop 用的，生产必须 http。

**`MCP_ALLOWED_ORIGINS` 强制要**是 MCP SDK 推荐的 DNS Rebinding 防护。MCP 协议天然是 SSE/Streamable，浏览器或网页插件类客户端会带 Origin，我们在 [security/origin.go#L18-L54](../../backend/internal/mcp/security/origin.go#L18-L54) 校验 Origin 是否在白名单。空 Origin（比如服务端调用）默认放行，要更严就开 `MCP_REQUIRE_ORIGIN_HEADER=true`。

**反直觉的点**：CORS 设置在我们这里**不是为了方便前端跨域，是为了限制谁能跨域**。`Access-Control-Allow-Origin` 永远回显请求的 Origin（前提是它在白名单），不是 `*`，避免任何站点的 JS 都能调 MCP。

`top_k` 上限 20 也是同样思路：**所有边界值都按"最坏情况能扛"来选，不按"最常用情况"选**。

> 相关代码：[config.go#L89-L133](../../backend/internal/mcp/config.go#L89-L133)、[security/origin.go](../../backend/internal/mcp/security/origin.go)

### 🔍 深挖追问兜底

> **Q：HTTP transport 的 SessionTimeout 5 分钟是怎么定的？**
> 我们在 [transport/http.go#L56-L61](../../backend/internal/mcp/transport/http.go#L56-L61) 用 `StreamableHTTPOptions{Stateless: true, SessionTimeout: cfg.SessionTimeout}`，默认 5 分钟。`Stateless` 模式下 SDK 不强依赖 session，但 SSE 长连接还是按这个超时清理，避免空闲连接占内存。

> **Q：Origin 白名单为空时为啥不拒？**
> 服务端到服务端调用（curl、Go SDK）不带 Origin 是正常的，拒了就误伤。要严防就把 `MCP_REQUIRE_ORIGIN_HEADER` 开成 true，强制必须带。

> **Q：MCP_ENABLE_LEGACY_APP_ID 为啥直接 return error？**
> Phase 1 设计就明确不再支持 legacy app_id 走 MCP，避免把已经在淘汰的认证方式带进新通道。代码里 `if c.EnableLegacyAppID { return ... not supported }` 这是硬约束。

---

## Q5：agent 维度的监控和指标是怎么做的？灰度发布又是怎么和 MCP 联动的？

**我的回答：**

监控分两层。

**MCP 自己这一层**有专属指标，在 [metrics/metrics.go](../../backend/internal/mcp/metrics/metrics.go) 里注册了 7 个：

- `mcp_tool_call_total{tool, status, error_code}`：调用总数
- `mcp_tool_call_duration_ms`：延迟直方图，桶是 `5, 10, 25, ..., 10000` 共 12 档
- `mcp_tool_call_result_count`：返回结果数分布
- `mcp_upstream_error_total{error_code}`：上游错误码分布
- `mcp_auth_missing_total` / `mcp_forbidden_total` / `mcp_backend_timeout_total`：三个独立计数

这些指标走标准 `/metrics` 端点，Prometheus 拉走就行。**关键设计是 `error_code` 标签**：把上游 RAG 的 401/403/429/504 全映射成 `unauthorized/forbidden/rate_limited/backend_timeout` 这种语义码（见 [client/rag_client.go#L219-L237](../../backend/internal/mcp/client/rag_client.go#L219-L237)），告警可以直接按错误码维度配。

**RAG 平台那一层**复用现有的 `app_id` 维度。每个 Agent 创建 API Key 时填 `app_id`（如 `claude-desktop`、`cursor-ide`），落到 `kb_retrieve_log.app_id` 索引字段。监控面板按 `app_id` group by 就能拿到每个 Agent 的 QPS、P95、错误率。

**灰度这块比较坦诚**：MCP 这一层目前**没有独立的灰度开关**。我们复用的是 RAG 平台已有的 `release.Controller`，灰度按 `userID` 分桶（FNV hash mod 100），见 [release/controller.go#L233-L243](../../backend/internal/rag/release/controller.go#L233-L243)。MCP 透传过来的 API Key 对应一个固定 `user_id`，所以**同一个 Agent 的所有流量要么全在 Phase2 要么全在 Phase1，不会请求级别分桶**。这个限制目前可接受，未来要做 agent 级灰度需要在 `evaluateStage` 里加 `app_id` 维度的桶逻辑。

> 相关代码：[metrics/metrics.go](../../backend/internal/mcp/metrics/metrics.go)、[release/controller.go#L44-L76](../../backend/internal/rag/release/controller.go#L44-L76)

### 🔍 深挖追问兜底

> **Q：`mcp_tool_call_duration_ms` 用 ms 不用 s，是不是和主指标体系不一致？**
> 确实不一致，RAG 主指标是 `_seconds`，MCP 是 `_ms`。当时考虑 MCP 调用大多在 100-2000ms，毫秒桶可读性更好。代价是 Grafana 模板要分两套，是个**已知遗留**，未来对齐到 seconds 比较合理。

> **Q：runtime override 灰度怎么生效？MCP 不重启也能切吗？**
> 能。`SetRuntimeOverride` 改的是全局变量配 `sync.RWMutex`，每次 `Decide` 拿读锁读最新值。MCP 调 `/v1/retrieve` 时 RAG 侧重新算一遍 Decision，对 MCP 完全透明。

> **Q：怎么知道某个 Agent 的检索质量是不是退化了？**
> 看 `kb_retrieve_log` 按 `app_id` 聚合的 `evidence_gate_result`、`refusal_reason`、`empty_reason` 三个字段的比例变化。这些字段在 RAG 主链路写入，MCP 不参与，但天然按 Agent 维度可切片。

---

## Q6：线上突然大量 MCP 调用返回 401/403，或者 P95 飙到 8 秒以上，你怎么排？

**我的回答：**

我会按"自下而上"的顺序排，因为 MCP 是最外层，链路最长。

**第一步：先看 MCP 的 Prometheus 指标**。`mcp_auth_missing_total` 涨说明 Client 没带 Token；`mcp_forbidden_total` 涨说明 API Key 在 RAG 侧被拒（吊销/过期/租户禁用）；`mcp_backend_timeout_total` 涨就是上游 RAG 慢或挂了。三个独立计数就是为了**一眼定位是哪一段出问题**。

**第二步：看 MCP HTTP 日志**。每条请求会打 `method=POST path=/mcp status=xxx duration_ms=xx origin=xxx auth=sha256:xxxx`（[transport/http.go#L128-L137](../../backend/internal/mcp/transport/http.go#L128-L137)）。**用 auth 指纹去 `kb_retrieve_log` 里关联**——指纹是 Authorization 头的 SHA256 前 8 字符，配合 `api_key_id` 能锁定是哪个 Agent。

**第三步：调 MCP 的 `/readyz`**。这个端点会主动打上游 RAG 的 `/readyz`（见 [transport/http.go#L83-L106](../../backend/internal/mcp/transport/http.go#L83-L106)），如果返回 `degraded`，问题在 RAG 不在 MCP，直接转到 RAG 告警群。

**第四步：看 RAG 平台的 `kb_retrieve_log`**。按 `app_id` 过滤目标 Agent，看 `result_status`、`error_code`、`duration_ms`、`auth_type`。如果 `auth_type=api_key` 且大量 401，多半是 Agent 那边换 Key 忘了同步；如果 `result_status=timeout`，看 `embedding_ms / search_ms / rerank_ms` 哪个段拉长。

**踩过的坑**：之前一次 P95 飙升其实是 Agent 把 `metadata_filter` 写成了几千行的 OR，命中我们的 16KB 上限被拒，但 Agent 重试逻辑没退避，**每秒打 50 个 400**。后来在 schema 描述里加了 filter 复杂度建议，并且监控 `mcp_tool_call_total{status="invalid_request"}` 单独配告警。

> 相关代码：[metrics/metrics.go](../../backend/internal/mcp/metrics/metrics.go)、[client/rag_client.go#L190-L237](../../backend/internal/mcp/client/rag_client.go#L190-L237)

### 🔍 深挖追问兜底

> **Q：401 时 MCP 怎么把错误返回给 Claude 的？会不会让 Claude 死循环重试？**
> 我们在 [handler/retrieve.go#L227-L261](../../backend/internal/mcp/handler/retrieve.go#L227-L261) `mapToolError` 里把 401 映射成 `{code: "unauthorized", retryable: false}`，作为 `CallToolResult{IsError: true}` 返回。MCP SDK 规范里 `retryable=false` 客户端不应该重试。

> **Q：上游 RAG 返回的 body 太大会怎么样？**
> 客户端读响应限制 `maxResponseBodyBytes = 8MB`（[client/rag_client.go#L16](../../backend/internal/mcp/client/rag_client.go#L16)），超了直接返回 `backend_error / retryable=true`，避免 MCP Server OOM。

> **Q：上游超时设多少？怎么和 Claude 的等待对齐？**
> `MCP_UPSTREAM_TIMEOUT_MS` 默认 5000ms（[config.go#L17](../../backend/internal/mcp/config.go#L17)）。Claude Desktop 一般给工具 30s，我们留足 6 倍冗余，超时一定是上游真的慢，不是网络抖动。

---

## Q7：MCP 这套现在还有什么没做？后续要做什么？为什么没一上来做？

**我的回答：**

坦诚地说，V1 目前**只暴露了 `retrieve_knowledge` 一个工具**，[server.go#L31-L37](../../backend/internal/mcp/server.go#L31-L37) 注册的就这一个。还没做的有几块：

**第一，Resources 和 Prompts 没做。** MCP 协议里除了 Tools 还有 Resources（知识库列表）和 Prompts（预置提问模板）。我们 V1 决定先验证 Tools 跑通，避免一次性铺得太大。后续做 Resources 可以把 `GET /v1/kbs` 包成 `kb://{tenant_id}/{kb_id}` 这种 URI。

**第二，agent 维度的独立灰度没做。** 上面说过，灰度还是按 RAG 平台 `user_id` 分桶，**MCP 没法对单个工具调用做百分比放量**。要做需要在 `release.Decide` 里加 `app_id` / `agent_name` 维度的桶，并且要把这个上下文从 MCP Header 传到 RAG。

**第三，工具调用的级联/链式没做。** 目前是单跳 Agent → MCP → RAG，没有让 RAG 反向通过 MCP 调用 Agent 工具（MCP Server-initiated requests）。也不打算做，越界了。

**第四，多工具版本管理没做。** 现在工具名 `retrieve_knowledge` 是硬编码，要发新版只能改名 `retrieve_knowledge_v2`。后续如果工具多了，可能引入 `tools/registry` 做版本路由。

**为什么这么排**：我们坚持的原则是 [mcp-agent-integration-design.md](../../backend/docs/rag/mcp-agent-integration-design.md) 里写的——**MCP 是协议适配层，不是新检索系统**。任何新能力先在 RAG 主链路落地、跑稳定，再考虑要不要在 MCP 上暴露。这样能保证 Web、SDK、MCP 三个通道的能力一致，不分裂。

> 相关代码：[server.go#L31-L37](../../backend/internal/mcp/server.go#L31-L37)、[mcp-agent-integration-design.md](../../backend/docs/rag/mcp-agent-integration-design.md)

### 🔍 深挖追问兜底

> **Q：为什么不直接对 Claude 暴露写入工具（ingest）？**
> 安全边界。Ingest 改的是数据本身，一旦 Agent 误调代价大。检索是读操作，错了重试就行。先把读跑稳一年再考虑写。

> **Q：未来要支持 SSE streaming 的检索结果，能加吗？**
> 能。SDK 的 `StreamableHTTPHandler` 本来就支持 SSE，我们只是 `Stateless: true` 没开流式。要做需要把 RAG `/v1/retrieve` 也改成流式，工程量在 RAG 侧不在 MCP 侧。

> **Q：V2 你最想先做哪个？**
> Resources。Claude/Cursor 用户体验上很需要"能看到有哪些知识库可选"，目前要靠 prompt 引导用户先问"我能查哪些库"。做 Resources 之后客户端 UI 可以直接列出来。

---

## 主动引导的钩子

💡 "说到 MCP 的 Authorization 透传，我们 **API Key + 多租户隔离** 模块也是这一套统一身份上下文，要不展开聊聊 `Identity` 是怎么在 JWT / API Key / Legacy app_id 三种认证下做归一化的？" → 引导到 **02-多租户管理与权限管控**

💡 "MCP 的 `mcp_tool_call_duration_ms` 这一类指标接到的是我们 **全链路可观测** 体系，Grafana 面板和告警规则在 `deploy/monitoring` 下都有现成的，可以聊聊 SLO 是怎么定的。" → 引导到 **10-全链路可观测体系**

💡 "MCP 这边的灰度是借 RAG 主链路的 `release.Controller`，那一套有 runtime override、stable bucket、回滚 playbook，要不顺着聊聊 P0 故障下我们是怎么 30 秒回滚的？" → 引导到 **灰度发布与回滚**

💡 "MCP 输入 schema 里 `top_k` 上限 20 是和 RAG 的 dynamic_topk 联动的，那一套 token_budget 决策可以单独聊，那是我们 Phase 2 检索质量的核心。" → 引导到 **04-检索核心**

💡 "MCP 把上游 RAG 错误码映射成语义码这块，其实和我们整个项目的错误分级体系一脉相承，可以聊聊我们怎么把 HTTP 状态码、业务码、可重试性三个维度收口的。" → 引导到 **错误处理与重试策略**
