# 07-统一检索接口与 Agent 接入 SDK-面试题和参考答案

> 面向场景：对外封装统一检索 OpenAPI（`POST /v1/retrieve`）+ MCP Server（`retrieve_knowledge` 工具）+ Agent 专用 Go SDK（`pkg/ragsdk`），屏蔽底层 Milvus、混合召回、重排、引用一致性等实现细节，让上游智能体只对一份契约编程，按需切换接入形态。

---

## 面试官提问路径

```text
"你们这个统一检索接口是怎么设计的？为啥同时给 OpenAPI、MCP、SDK 三种入口？"
    ↓
"这三个入口下面共用同一个 handler，那入参/出参契约是怎么对齐的？"
    ↓
"上游接 RAG，最关心的就是认证。三种身份（API Key / JWT / 老 app_id）怎么共存？"
    ↓
"为什么响应里要塞 strategy_version、request_cost、citation_check 这些字段？Agent 用得上吗？"
    ↓
"Milvus / 混合召回换实现，你怎么保证不破坏上游？"
    ↓
"Agent 集成接口超时/限流，你怎么排？SDK 这层做了哪些兜底？"
    ↓
"为什么不直接让 Agent 调 Milvus？包一层有什么收益？"
```

---

## Q1：你们这个统一检索接口为什么要做三种形态（OpenAPI / MCP / SDK）？是不是过度设计？

**我的回答：**

核心结论：**一份契约，三种皮**——三种形态共用同一个 `POST /v1/retrieve` handler，不是三套实现。

入口我们这么分：HTTP OpenAPI 是给后端服务、第三方系统用的；MCP Server 是给 Claude Desktop / Cursor 这类 AI 客户端原生集成用的；Go SDK（`pkg/ragsdk`）是给我们自己 Go 写的 Agent 用的，相当于把 HTTP 调用、API Key Header、错误解构这些样板代码封一层。

**为什么三种皮但底下只有一份肉**：看代码就知道，[mcp/server.go](../../backend/internal/mcp/server.go#L47-L60) 里 MCP 的 `RetrieverFactory` 调的是 `ragclient.NewWithAuthorization`，这个 client 打的就是 `<base>/v1/retrieve`（见 [rag_client.go](../../backend/internal/mcp/client/rag_client.go#L92)）。SDK 一样，[ragsdk/client.go](../../backend/pkg/ragsdk/client.go#L75) 拼的也是 `BaseURL + "/v1/retrieve"`。**MCP 在我们项目里只是协议适配层，不是检索实现层**。

这么干的收益是上游 Agent 想接哪种就接哪种，但租户隔离、API Key 鉴权、`kb_retrieve_log` 落库、Prometheus 指标只在 [api/handler/rag/retrieve.go](../../backend/api/handler/rag/retrieve.go#L192-L298) 这一处做。**改一次，三种入口同时生效**。

> 相关代码：[ragrouter/register.go#L91-L98](../../backend/api/ragrouter/register.go#L91-L98)、[mcp/server.go](../../backend/internal/mcp/server.go)、[pkg/ragsdk/client.go](../../backend/pkg/ragsdk/client.go)

### 🔍 深挖追问兜底

> **Q：MCP 不能直接调 Milvus 吗？非要绕一跳 HTTP？**
> 不能。直接调 Milvus 会绕过 `kb_permission` 授权、API Key 配额、`request_id` 落库、灰度策略路由，等于 MCP 自己再实现一遍治理。多一跳本地 HTTP 跳转大概几毫秒，换治理一致性完全划算。

> **Q：SDK 这么薄（就 110 行），为啥不让用户自己写 HTTP？**
> 三件事用户写起来累：API Key Header 拼 `Bearer rag_xxx`、`*APIError` 错误类型断言、超时默认 10s。SDK 把这些固化下来，配合 [README.md](../../backend/pkg/ragsdk/README.md) 列的状态码表，Agent 集成基本只剩业务逻辑。

> **Q：未来会不会再加 gRPC 入口？**
> 设计上是允许的，再加一层薄壳调 `/v1/retrieve` 就行；但目前没有上游有 gRPC 诉求，YAGNI，**没做不假装做**。

---

## Q2：`/v1/retrieve` 这份契约的请求/响应字段是怎么定的？为什么响应里要塞 `strategy_version`、`request_cost`、`citation_check`？

**我的回答：**

请求体定义在 [api/handler/rag/retrieve.go#L20-L28](../../backend/api/handler/rag/retrieve.go#L20-L28)，6 个字段：`app_id`（兼容老通道）、`kb_id`、`kb_ids`、`query`（必填）、`top_k`、`strategy_profile`、`metadata_filter`。响应体在同文件 L31-L65，主体是 `items[]`，每个 item 包含 `content`、`score`、`citation`、`source`。

**关键决策：响应里塞了一堆"治理字段"**——`request_id`、`strategy_version`、`request_cost.estimated_cost`、`citation_check`、`refusal`、`evidence_gate_result`。从 [mcp/handler/retrieve.go#L49-L57](../../backend/internal/mcp/handler/retrieve.go#L49-L57) 的 `RetrieveKnowledgeOutput` 也能看到这些字段是直接透传的。

**为什么 Agent 用得上**：

第一，`request_id` 是排障锚点。Agent 出问题给我 request_id，我反查 `kb_retrieve_log`、Prometheus、链路日志一条龙。

第二，`strategy_version` 让 Agent **知道这次走的是哪条策略路径**——比如 `phase2-hybrid-v3` 还是 `phase1-dense-v1`。出现召回质量异常，Agent 自己就能判断是不是策略灰度撞上了。

第三，`citation_check` / `refusal` / `evidence_gate_result` 是**反幻觉证据**。Agent 拿到 `evidence_gate_result=block`，就知道这次召回不够支撑回答，应该走拒答模板而不是硬编。

**反直觉的点**：很多团队会觉得"接口返回值越精简越好"。我们反过来——**多塞 200 字节的元信息，省掉 Agent 后续 5 个排障接口**。

> 相关代码：[api/handler/rag/retrieve.go#L20-L65](../../backend/api/handler/rag/retrieve.go#L20-L65)、[mcp/handler/retrieve.go#L40-L57](../../backend/internal/mcp/handler/retrieve.go#L40-L57)、[mcp/client/rag_client.go#L26-L55](../../backend/internal/mcp/client/rag_client.go#L26-L55)

### 🔍 深挖追问兜底

> **Q：`kb_id` 和 `kb_ids` 同时存在不冗余吗？**
> `kb_id` 是兼容老 Agent 的单库写法，新接入推荐 `kb_ids`。归一化在 [retrieve.go#L113-L135](../../backend/api/handler/rag/retrieve.go#L113-L135) 的 `resolveRequestedKBIDs`，用 `seen` map 去重合并，handler 内部只处理 `kb_ids`。

> **Q：`strategy_profile`、`metadata_filter` 是 V1 字段还是预留？**
> 预留。tools 定义里写得很清楚（[tools/definition.go#L42-L49](../../backend/internal/mcp/tools/definition.go#L42-L49)）："只有底层检索链路支持时才保证生效"，**不保证语义稳定但保证字段稳定**，避免上游频繁改协议。

> **Q：响应里为什么 `citation`、`source` 是嵌套对象不是平铺字段？**
> 引用是面向人的（kb_id、file_name、chunk_index 给前端展示用），source 是面向系统的（route=dense/sparse/hybrid，retriever_version 给排障用），两个面向的字段拆开，未来某一边演进不会污染另一边。

---

## Q3：三种身份（API Key / JWT / Legacy app_id）怎么在同一个 handler 里共存？为什么这么设计？

**我的回答：**

入口在 [api/handler/rag/retrieve.go#L242-L286](../../backend/api/handler/rag/retrieve.go#L242-L286)，是一个 switch-case 三分支：`AuthTypeAPIKey` → `AuthTypeJWT` → `AuthTypeLegacyAppID`，**优先级从高到低**。

具体逻辑：先看请求里有没有 `Authorization: Bearer` 头，有就走 [L207-L240](../../backend/api/handler/rag/retrieve.go#L207-L240)，用 `auth.ValidateAPIKeyFormat(token)` 判断是不是 `rag_` 开头，是就查 `rag_api_key` 表，命中则注入 API Key 身份；JWT 由更上层的 OptionalAuth 中间件提前注入；都没有就 fallback 到请求体里的 `app_id`，匹配静态白名单 `interview-agent / mianshiba-web / mianshiba-admin`（见 [L179-L188](../../backend/api/handler/rag/retrieve.go#L179-L188)）。

**关键决策：用 `Identity` 抽象统一三种身份**。无论哪条分支，最终都构造一个 `*auth.Identity`，包含 `TenantID`、`UserID`、`AppID`、`APIKeyID`、`AuthType`、`IsLegacy` 字段，写进 `ctx` 和 `c.Set`。下游 `kb.Retrieve` 从 ctx 拿 Identity，**不关心来源是哪一种**——这是认证逻辑能统一收口的关键。

**为什么留 Legacy**：项目里有几个老前端（mianshiba-web/admin）历史上用 `app_id` 鉴权，这次改造**不能 breaking**。所以做了双轨：新接入只发 API Key，老接入用 `app_id`，handler 里打日志 `Legacy app_id=%s, deprecated after Phase 2`（[L278](../../backend/api/handler/rag/retrieve.go#L278)），等灰度替换完整再删。

> 相关代码：[retrieve.go#L101-L111](../../backend/api/handler/rag/retrieve.go#L101-L111)（setIdentityContext）、[retrieve.go#L137-L175](../../backend/api/handler/rag/retrieve.go#L137-L175)（authorizeRetrieveKBIDs）

### 🔍 深挖追问兜底

> **Q：API Key 校验为什么要 `HashAPIKey` 后查表？不能直接存明文？**
> Key 数据库泄漏后明文 Key 直接可用是灾难。我们在 [L213-L214](../../backend/api/handler/rag/retrieve.go#L213-L214) 算 SHA256 哈希再查 `key_hash` 字段，库里**只存哈希**。Key 本身只在创建时返回一次给用户。

> **Q：API Key 状态机怎么管的？吊销/过期怎么处理？**
> 命中后立即检查 `apiKey.Status == "revoked"` 和 `IsAPIKeyExpired(apiKey.ExpiresAt)`（[L216-L223](../../backend/api/handler/rag/retrieve.go#L216-L223)），任意一个不通过直接 401。`UpdateLastUsed` 异步更新 `last_used_at`，方便审计页看哪些 Key 还活跃。

> **Q：多个知识库同时请求，权限怎么校验？**
> `authorizeRetrieveKBIDs` 里 for 循环每个 kb_id 都查两道：`GetByIDForTenant`（KB 是否属于该租户）+ `CheckPermission(read)`（租户是否有读权限）。**任何一个失败整个请求就 403**，不做"部分成功"，避免上游误以为结果是完整的。

---

## Q4：`top_k=20` 这个上限为什么不是 50 或 100？关键参数是怎么定的？

**我的回答：**

上限定在 [mcp/handler/retrieve.go#L17-L24](../../backend/internal/mcp/handler/retrieve.go#L17-L24) 这一组常量：`defaultTopK=5`、`maxTopK=20`、`maxQueryRunes=2000`、`maxKBIDs=100`、`maxMetadataFilterBytes=16KB`、`maxMetadataFilterDepth=8`。OpenAPI 侧 [kb/handler.go#L46-L48](../../backend/api/handler/kb/handler.go#L46-L48) 也是 `defaultRetrieveTopK=5 / maxRetrieveTopK=20`，**两边硬对齐**。

**为什么 top_k 上限是 20**：

第一，**Token 预算约束**。我们后端有个 `dynamic_topk` 策略基于 `token_budget` 反推 top_k，单 chunk 平均 400 tokens，20 条就 8K tokens，再加 query、system prompt、上下文，刚好顶到主流模型 16K-32K context 的安全水位。**给 50 没意义，模型也吃不下**。

第二，**Milvus 召回成本**。混合召回会跑 dense + sparse 两路，再 RRF 融合再重排。top_k 翻倍，重排模型推理时间近似线性涨。线上 P95 控制在 2.5s 以内，20 是测出来的甜点。

第三，**业务实际不需要**。线上看 retrieve_log，**90% 的请求 top_k ≤ 5**，超过 10 的几乎都是 evaluation 跑批。

`maxQueryRunes=2000`：query 太长 embedding 噪声大且语义被稀释，超过 2000 字基本是"塞段落进来"，用户体验不会好。

`maxKBIDs=100`：单租户最多挂 100 个 KB 同时检索，超了基本是配置错误或攻击行为。

`maxMetadataFilterBytes=16KB / depth=8`：见 [validateMetadataFilter](../../backend/internal/mcp/handler/retrieve.go#L162-L177)，序列化后限大小、递归算深度，**防 Agent 塞奇怪嵌套对象打挂 Milvus 解析**。

> 相关代码：[mcp/handler/retrieve.go#L17-L24](../../backend/internal/mcp/handler/retrieve.go#L17-L24)、[kb/handler.go#L44-L53](../../backend/api/handler/kb/handler.go#L44-L53)

### 🔍 深挖追问兜底

> **Q：上层想要 `top_k=50` 怎么办？**
> 不放开 maxTopK，让用户走 `strategy_profile` 让后端 `dynamic_topk` 决定，或者拆多次请求。**底线参数不松口**，否则 SLA 守不住。

> **Q：这些常量为什么不放配置中心动态调？**
> 这些是协议层硬约束，不是策略层参数。改了等于改契约，Agent 要重新对接。**契约稳定优先于灵活**。策略层的参数（rerank 模型、融合权重）才走配置。

> **Q：Schema 校验和 Handler 校验为什么要写两遍？**
> Schema 是给规矩客户端（Claude/Cursor）的第一道闸，能在客户端就报错；Handler 是防伪造请求的第二道闸。**两道独立**，schema 漏了 handler 还能挡，handler 漏了 schema 也能限流早期攻击。

---

## Q5：底层从纯 dense 检索切到混合召回（dense+sparse+rerank），上游 Agent 一行代码没改就生效了，这是怎么做到的？

**我的回答：**

这就是统一接口的核心收益：**契约层零改动，实现层随意演进**。

整个切换路径在 [kb/handler.go#L967-L969](../../backend/api/handler/kb/handler.go#L967-L969)：

```go
phase2Available := config.Global.RAG.FeatureFlags.EnableHybridRetrieval && manager.GetHybridRetriever() != nil
releaseDecision := release.Decide(config.Global.RAG, phase2Available, userID, middleware.GetUserRole(c))
useHybrid := releaseDecision.UsePhase2 && manager.GetHybridRetriever() != nil
```

逻辑是：先看 feature flag（`EnableHybridRetrieval`）+ 实例可用性（`GetHybridRetriever() != nil`），再过 release 灰度决策（按 userID 取模分桶），最终 `useHybrid` 决定走 phase1 dense 还是 phase2 hybrid。

**关键是返回结构没变**——不管走哪条分支，`items[].content/score/citation/source` 这套字段是稳定的。Agent 只能从 `source.route` 字段里看出来这次是 `dense` 还是 `hybrid`，但**不需要为此改代码**。

**实战收益**：上线 phase2 那次，我们先按租户白名单灰度 5% 流量，再 20%，再 50%，最后 100%。**整个过程上游 Agent 团队完全不知情**，他们只是在 Grafana 看到自己 P95 微涨 100ms（多了 sparse + rerank）、recall@5 涨了 8 个点。**有问题随时回滚 feature flag，秒级生效**。

这就是抽象的价值：把 "Milvus index 怎么建、HNSW 还是 IVF、rerank 用 BGE 还是 Cohere" 这些**实现决策关在内部**，Agent 永远只对 `/v1/retrieve` 这一份契约编程。

> 相关代码：[release/controller.go](../../backend/internal/rag/release/controller.go)、[mcp-agent-integration-design.md](../../backend/docs/rag/mcp-agent-integration-design.md)

### 🔍 深挖追问兜底

> **Q：万一返回结构必须变怎么办？比如新增一个 `rerank_score` 字段？**
> JSON 字段是**加号兼容**的——只加字段不删字段、不改字段含义。Agent 用 Go struct 解析忽略未知字段，用 Python 字典也能容忍。真要破坏性改造就开 `/v2/retrieve` 并行跑。

> **Q：灰度发现 phase2 有问题，怎么快速回滚？**
> Feature flag 改 false 就行，不用发版。`release.Decide` 每次请求都重新读 flag。如果是 Milvus index 本身坏了，[index lifecycle](../../backend/internal/rag/indexlifecycle/service.go) 还能切回前一个 index 版本。

> **Q：dense 和 hybrid 召回结果差异大，Agent 不会感知到吗？**
> 会感知召回质量变化，但不会感知 schema 变化。我们在响应里塞 `strategy_version` 就是让 Agent **能观测到**这种变化——召回明显变差时上游可以告警，但**不会因此跑不通**。

---

## Q6：线上突然有个 Agent 报"调 /v1/retrieve 大面积 401"，你怎么排？

**我的回答：**

按链路从外到内一段段排，**不要乱猜**。

**第一步，确认是哪一类 401**。RAG 平台 401 一共四种：`Authentication required`（没认证）、`API key is revoked`、`API key is expired`、`Permission denied`（这个是 403 不是 401）。让 Agent 同学截一条 response body 看 message 字段就能定位。

**第二步，看 request_id 反查日志**。每次响应都带 `request_id`，去 ELK 拉 `[Auth]` 和 `[RAG Public API]` 两行（[L278](../../backend/api/handler/rag/retrieve.go#L278) 和 [L288-L295](../../backend/api/handler/rag/retrieve.go#L288-L295)），能看到 `auth_type=api_key tenant_id=X app_id=Y`，立刻知道是哪个租户、哪个 Key。

**第三步，分情况处置**：

- 如果 `auth_type=""`：客户端根本没带 Authorization 头，让 Agent 检查 SDK 配置；MCP 客户端则检查 [transport/http.go#L65](../../backend/internal/mcp/transport/http.go#L65) 的 `RequireBearerToken` 中间件是不是拒了空 token。

- 如果 `api_key_repo.GetByKeyHash` miss：Key 不存在或被删了，让用户重新创建。

- 如果 `apiKey.Status == "revoked"`：管理员主动吊销了，看审计日志确认是不是误操作。

- 如果 `IsAPIKeyExpired`：到期了，走 `/v1/api-keys/:id/rotate` 续期。

**第四步，看 Prometheus**。MCP 这层有 `mcp_auth_missing_total`、`mcp_forbidden_total` 计数器（[mcp/metrics/metrics.go#L53-L70](../../backend/internal/mcp/metrics/metrics.go#L53-L70)），陡增就是大面积事故。RAG 主链路有 `rag_retrieve_total{status="error",error_code="unauthorized"}`，按 `app_id` 分组能定位是单 Agent 问题还是平台问题。

**线上踩过一次坑**：某次 DB 主从延迟，新创建的 API Key 在主库写入但从库没同步，`GetByKeyHash` 走从库查不到，全部 401。后来在 `apiKey_repo` 里加了**主库强读**兜底（创建后 5 分钟内的 Key 强制读主），问题再没出现过。

> 相关代码：[retrieve.go#L207-L240](../../backend/api/handler/rag/retrieve.go#L207-L240)、[mcp/metrics/metrics.go](../../backend/internal/mcp/metrics/metrics.go)

### 🔍 深挖追问兜底

> **Q：如果是 SDK 这层超时怎么办？10s 是不是太短？**
> [ragsdk/client.go#L36-L38](../../backend/pkg/ragsdk/client.go#L36-L38) 默认 10s，用户可以传 `ClientConfig.Timeout` 覆盖。后端 P99 一般 3s 内，10s 留 3 倍 buffer。要再短的话客户端自己设。MCP 那边默认 5s（`defaultUpstreamTimeout`），更紧。

> **Q：怎么区分是 SDK 客户端问题还是服务端问题？**
> SDK 报 `*APIError` 一定是收到了 HTTP 响应（服务端可达），看 StatusCode 排服务端；如果是 `context.DeadlineExceeded` 或 `connection refused`，那就是网络/服务不可达。MCP 这层 [client/rag_client.go#L114-L128](../../backend/internal/mcp/client/rag_client.go#L114-L128) 把 timeout 映射成 `backend_timeout`、连不上映射成 `backend_unavailable`，**带 retryable 标志**给 Agent 自己决定要不要重试。

> **Q：401 大量出现会不会反过来打挂 DB？**
> 会有压力。`GetByKeyHash` 每次都 hit DB。**目前没做 API Key 缓存**——未来思考是 Redis 缓存 Key→Identity 映射，TTL 1 分钟，吊销时主动 invalidate。这个改造在 backlog 里。

---

## Q7：为什么不让 Agent 直接 SDK 调 Milvus，包这一层有什么真实收益？

**我的回答：**

直接调 Milvus 看着省事，实际会失去 5 样东西，每一样都是上线必备：

第一，**多租户隔离**。Milvus collection 是物理隔离，但租户哪些 KB 能读、读多少次、配额是多少，**Milvus 不知道**。我们这层有 `kb_permission` 表 + `quota` 模块（[internal/quota/quota.go](../../backend/internal/quota/quota.go)）兜底。

第二，**统一身份**。三种 auth 在同一个 `Identity` 抽象下融合，Milvus 只认连接级用户。

第三，**可观测**。`request_id` 串全链路、`kb_retrieve_log` 落每次调用、Prometheus 出指标、Grafana 看大盘——**全是这一层做的**，绕过了就全没。

第四，**策略和灰度**。混合召回、查询改写、引用一致性、拒答模板、动态 topK——这些都在 handler 之后串起来跑，Agent 直连 Milvus 等于回到原始召回。

第五，**演进自由**。Milvus 换 ES、换 Qdrant、换自研向量库，**只要 `/v1/retrieve` 契约不变，Agent 一行不改**。这个收益看着抽象，**真到要换底层那天就是救命的**。

**反直觉的点**：很多人觉得 SDK 越薄越好、抽象越少越好。我们项目里 SDK [client.go](../../backend/pkg/ragsdk/client.go) 本身就 110 行很薄，**真正的厚度在服务端 handler 里**。SDK 薄、契约稳、服务端厚——这个搭配让 Agent 集成成本最低，平台演进空间最大。

> 相关代码：[pkg/ragsdk/client.go](../../backend/pkg/ragsdk/client.go)、[api/handler/rag/retrieve.go](../../backend/api/handler/rag/retrieve.go)、[mcp-agent-integration-design.md](../../backend/docs/rag/mcp-agent-integration-design.md)

### 🔍 深挖追问兜底

> **Q：那 SDK 不该再厚一点？比如把重试、限流也封进去？**
> **目前没做客户端重试**——HTTP `*APIError` 带 StatusCode，由调用方决定是否重试（429/503 重试、4xx 不重试）。MCP 那层 [UpstreamError.Retryable](../../backend/internal/mcp/client/rag_client.go#L57-L63) 字段透出来了，SDK 这层未来思考是加 `retry_on=[429,503]` 默认策略。

> **Q：如果 Agent 就是想做"高级用法"，比如自定义 reranker，怎么办？**
> 不开放。这种诉求让团队走 `strategy_profile` 提需求，由我们在后端加策略。**让 Agent 自定义 reranker 等于把策略治理外包给上游**，治理就崩了。

> **Q：包这一层会不会成为性能瓶颈？**
> handler 本身耗时 < 5ms（仅做权限校验和参数归一），Milvus + rerank 才是大头（150-500ms）。包这层**纯粹是治理税，不是性能税**。

---

## 主动引导的钩子

把节奏往熟悉话题拉的几个口子：

💡 "说到统一契约，我们 MCP 接入也是这套思路——MCP Server 只做协议适配不做检索实现，全部走 `/v1/retrieve`..." → 引导到 04-MCP 协议接入

💡 "讲到灰度切实现，我们 phase1→phase2 这次切换就是靠 `release.Decide` + feature flag，按租户/用户分桶..." → 引导到 03-策略治理与灰度发布

💡 "API Key 这套设计还涉及到吊销、过期、轮换，我们专门有 `/v1/api-keys/:id/rotate` 接口..." → 引导到 API Key 生命周期

💡 "提到上游排障，我们其实做了完整的 request_id 串链路 + 检索调试视图，每条调用都能回放..." → 引导到 10-全链路可观测

💡 "聊到响应里塞 `evidence_gate_result`，背后是我们做的反幻觉证据闸门，结合 citation_consistency 校验..." → 引导到引用一致性 / 拒答模板
