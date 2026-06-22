# 统一检索接口与 Agent 接入 SDK - 7 个最容易踩的坑与避坑指南

---

## 前言

"对外封装一个统一检索 OpenAPI + MCP + SDK"听起来就是套个 HTTP 壳子的事，但只要你真做过就会发现：契约设计、认证收口、SDK 边界、灰度演进，每一项都能让你掉头发。下面这 7 个坑按踩中概率从高到低排，配合主文档 [07-统一检索接口与Agent接入SDK-面试题和参考答案.md](./07-统一检索接口与Agent接入SDK-面试题和参考答案.md) 食用更佳。

---

## 🕳️ 坑 1：让 Agent 直连 Milvus 提速 → 治理彻底崩盘

### 问题描述
有 Agent 团队为了"少一跳网络延迟"，绕过 `/v1/retrieve`，直接拿 Milvus 连接串去查向量库。

### 踩坑过程
最开始的普遍心态是"包一层 HTTP 没必要，Milvus 客户端 SDK 这么完善，直连还能省几毫秒"。

某个对延迟敏感的 Agent 团队就这么干了：拿到 Milvus 集群的地址，自己写了一份检索代码，把 collection 名硬编码进去，绕过了 `/v1/retrieve`。

跑了一段时间，**问题集中爆发**：第一，他们查不到 `kb_retrieve_log` 里的记录，因为根本没经过 handler；第二，这个 Agent 的调用量在 Prometheus 看板上是 0，排障时完全是黑盒；第三，权限管理团队加了 `kb_permission` 表的读权限校验，他们这条直连路径**不受控**——后来这个 Agent 居然能查到本不属于它的租户的 KB。

最后是审计同学翻 Milvus 慢查询日志才发现的，整改花了两周。

### 后果
- 监控失真严重：该 Agent 占了真实流量 15%，但平台看板显示 0%
- 权限越权：直连路径绕过了 `kb_permission` 校验，跨租户数据泄漏风险
- 排障盲区：用户投诉慢查询时，平台连这个调用都看不到
- 演进受阻：后端想换 Milvus 索引时，必须先找出所有直连客户挨个通知

### 避坑方案
1. **网络层封死**：Milvus 集群只允许 RAG 后端服务的 IP/Service Account 接入，从网络 ACL 层面禁止 Agent 直连，**比口头规约靠谱 100 倍**。

2. **SDK 是唯一入口**：在 [pkg/ragsdk](../../backend/pkg/ragsdk/client.go) 里只暴露 `Retrieve(ctx, RetrieveRequest)` 一个方法，连 BaseURL 都鼓励用户配成内部域名而不是直接传 IP。

3. **审计兜底**：定期跑脚本扫 Milvus 慢查询日志，把 `client_ip` 不在白名单里的 Agent 拉出来约谈。

4. **响应里藏"防伪"字段**：在 `/v1/retrieve` 响应里塞 `source.retriever_version`（[rag_client.go#L51-L55](../../backend/internal/mcp/client/rag_client.go#L51-L55)），让上游业务依赖这个字段，反向迫使他们走正经接口。

> 详见主文档 [Q7：为什么不让 Agent 直接 SDK 调 Milvus](./07-统一检索接口与Agent接入SDK-面试题和参考答案.md#q7为什么不让-agent-直接-sdk-调-milvus包这一层有什么真实收益)

### 📚 延伸知识点
- **API Gateway 模式**：把所有数据访问收敛到统一网关，是微服务治理的经典模式，对应 BFF（Backend For Frontend）和 API Gateway 两种典型形态
- **Zero Trust Network**：网络层默认不信任任何节点，业内通病是"内网随便连"，零信任要求每次访问都校验身份
- **Schema Registry**：把契约定义集中托管（如 Confluent Schema Registry、Buf Schema Registry），任何客户端都从注册中心拉契约，根本不可能自创路径

### 面试时怎么说
> "在统一检索接口这块我们踩过一个挺典型的治理坑。最开始有 Agent 团队觉得 HTTP 多一跳浪费，就直连 Milvus 了，绕过了我们的 `/v1/retrieve`。
>
> 当时我们没拦——觉得反正是内网，又是自己人。结果跑了两个月，审计同学翻 Milvus 慢查询日志才发现，这个 Agent 一直越权访问别人租户的 KB，监控看板上还显示流量是 0，完全是黑盒。
>
> 后来我们就把网络 ACL 收紧了：Milvus 只允许 RAG 后端的 Service Account 接入，Agent 的 IP 全拉黑。这件事让我意识到：**所谓"统一接口"，光靠口头约定是没用的，必须从网络层、SDK 层、审计层三道闸门同时封死，才叫真正的统一**。"

---

## 🕳️ 坑 2：API Key 明文存库 → 一次拖库全军覆没

### 问题描述
为了"调试方便"，把 API Key 明文存进 `rag_api_key.api_key` 字段，结果某次 DB 备份外泄，所有租户被迫紧急轮换。

### 踩坑过程
最开始建表的时候很多团队会犯这个错：API Key 嘛，本来就是一串随机字符串，又不是密码，明文存好像也没什么。**而且明文存还有个"好处"——客服可以直接帮用户找回 Key**。

我们项目从一开始就走了哈希存储，[retrieve.go#L213-L214](../../backend/api/handler/rag/retrieve.go#L213-L214) 里 `keyHash := auth.HashAPIKey(token)`，库里 `key_hash` 字段存的是 SHA256，明文 Key 只在创建时一次性返回给用户。

但**业内不少团队**踩过这个坑：DBA 备份脚本误传到公网 OSS、内部 BI 系统把整张表 join 进数据看板、或者运维 dump 出 SQL 文件分享给乙方。任何一种情况下，明文 Key 一旦泄漏，**攻击者拿着 Key 就能直接调 `/v1/retrieve`**，平台甚至看不出来这是攻击——因为 Key 本身是合法的。

### 后果
- 紧急轮换全租户的 Key（少则几十、多则上千个），上游 Agent 全部要重新部署
- 信任崩溃：客户会问"你们还有多少这种地方明文存？"
- 法务风险：如果 Key 关联到了用户行为日志，可能触发 GDPR / 个保法

### 避坑方案
1. **库里只存哈希**：用 SHA256 或 BLAKE3，**绝不存明文**。我们的实现见 [auth.HashAPIKey](../../backend/internal/auth/apikey.go)，查询时用 `key_hash` 字段。

2. **明文只在创建时返回一次**：参考 GitHub Personal Access Token 的设计——你点一次"显示"之后，再也看不到原文。强迫用户立刻保存。

3. **前缀可见 + 主体哈希**：Key 设计成 `rag_<8位前缀>_<随机主体>`，库里另存一份明文前缀方便用户在控制台识别"这是我哪个 Key"，但**主体永远只存哈希**。

4. **Key 状态机要全**：必须有 `active / revoked / expired` 三态，[retrieve.go#L216-L223](../../backend/api/handler/rag/retrieve.go#L216-L223) 里 revoked 立即拒、expired 立即拒，不能给"宽限期"。

5. **创建即过期日**：默认 90 天 TTL，强制走 `/v1/api-keys/:id/rotate` 轮换。**不允许永久 Key**。

### 📚 延伸知识点
- **Secret Scanning**：GitHub 等平台的内置功能，能扫到代码仓库里误提交的 API Key 并通知发行方吊销
- **HSM / KMS**：硬件加密模块，管理高敏感密钥的标准方案；轻量场景用 AWS KMS / 阿里云 KMS 即可
- **Token Binding**：把 Token 和 TLS 证书绑定，即使 Token 泄漏也无法在另一台机器使用，OAuth 2.0 DPoP 是相关标准

### 面试时怎么说
> "API Key 存储这块我们一开始就坚持只存哈希，没踩到那个最经典的坑——但业内见过太多反面教材了。
>
> 有些团队为了客服找回方便会明文存，听起来体贴用户，实际上一次拖库就是大灾难。攻击者拿着 Key 直接调接口，平台都看不出是攻击，因为 Key 本身合法。
>
> 我们的做法是：库里只存 SHA256，明文 Key 创建时返回一次，前缀 `rag_xxxx` 留可见方便用户识别，主体永远不可逆。这个设计其实抄的是 GitHub PAT。**这件事让我想明白一个点：凡是涉及凭据的，便利性必须给安全性让路，没得商量**。"

---

## 🕳️ 坑 3：响应字段加号兼容当 breaking change 改 → 上游全炸

### 问题描述
后端给 `RetrieveItem.Score` 字段从 `float64` 改成对象 `{value, normalized}`，结果所有 Go SDK 客户端反序列化失败。

### 踩坑过程
项目刚起步时，`/v1/retrieve` 响应结构很简洁，[ragsdk/client.go#L62-L67](../../backend/pkg/ragsdk/client.go#L62-L67) 里 `Score` 就是个 `float64`。

后来引入 rerank 之后，团队想要在响应里同时返回"原始分数"和"归一化分数"。最直观的改法是**把 `score` 字段从 number 改成 object**：

```json
"score": {"raw": 0.91, "normalized": 0.78}
```

这种改法在内部沟通时谁都没意识到问题，因为大家都觉得"加字段嘛，向前兼容"。

上线后**所有 Go Agent 全炸**——`json.Unmarshal` 把 object 塞不进 `float64`，直接 `cannot unmarshal object into Go struct field`。回滚再上线花了 40 分钟，期间所有走 SDK 的 Agent 检索功能完全不可用。

**后来才意识到**：「向前兼容」的正确含义是**只增字段、不改字段类型、不改字段语义**。把 `score` 从 number 改成 object，对 Go 这种强类型语言就是 breaking。

### 后果
- 全部 Go SDK 客户端 5xx，影响面 100%
- 紧急回滚 40 分钟，期间所有依赖 SDK 的 Agent 检索瘫痪
- 信任受损：之后每次响应字段调整都要拉群评审，效率下降
- 后续不得不开 `/v2/retrieve` 并行跑，运维成本翻倍

### 避坑方案
1. **JSON 兼容三铁律**：
   - ✅ 可以**新增**字段（老客户端忽略未知字段）
   - ❌ 绝不**改字段类型**（number → object、string → array 都是 breaking）
   - ❌ 绝不**改字段含义**（同名字段含义变了比类型变更更隐蔽）

2. **要扩展就加新字段**：想要 normalized score？加个 `normalized_score: float64` 字段就行，老的 `score` 不动。

3. **契约测试 (CDC)**：在 CI 里跑 [ragsdk/example_test.go](../../backend/pkg/ragsdk/example_test.go) 这种契约测试，反序列化失败就 fail build。

4. **SDK 用 `omitempty` + 强类型 struct**：[ragsdk/client.go#L62-L67](../../backend/pkg/ragsdk/client.go#L62-L67) 用 struct 显式声明字段，比 `map[string]interface{}` 更安全——至少未知字段会被忽略而不是抛错。

5. **真要 breaking 就开新版本**：[ragrouter/register.go#L91-L98](../../backend/api/ragrouter/register.go#L91-L98) 里路由按 `/v1` 前缀分组，**未来真要破坏性改造直接开 `/v2/retrieve`**，新老并行至少 6 个月。

### 📚 延伸知识点
- **Consumer-Driven Contracts (CDC)**：消费端定义契约，提供端的每次发布都跑消费端契约测试，Pact 是这个领域的事实标准
- **Protobuf 演进规则**：Google 的 Proto3 设计本身就是为了演进——字段编号永不复用、reserved 关键字防误删，是 schema 演进的范本
- **SemVer**：语义化版本，breaking change 必须升大版本号，是开源社区的通用约定
- **Field Mask**：gRPC/REST 里用 mask 显式声明客户端关心哪些字段，可以做更精细的兼容性控制

### 面试时怎么说
> "在响应契约这块我们踩过一个挺痛的坑。当时想给检索结果加一个归一化分数，最直观的改法是把 `score` 从 number 改成对象 `{raw, normalized}`，团队都觉得"加内容嘛，向前兼容"。
>
> 结果上线后所有 Go SDK 客户端全炸了——`json.Unmarshal` 把 object 塞不进 float64，直接报错。回滚花了 40 分钟。
>
> 后来我们立了个铁律：**JSON 演进只能加字段，不能改字段类型，不能改字段语义**。要 normalized score？加个 `normalized_score` 新字段就行，老字段一动不动。这件事让我学到的是：**"兼容"不是看你主观觉得"内容是不是变多了"，而是看强类型语言能不能反序列化成功——技术决策得用工具的视角，不能用人的视角**。"

---

## 🕳️ 坑 4：MCP 透传 Authorization 误验真伪 → 双重校验逻辑漂移

### 问题描述
MCP Server 自己在 [transport/http.go](../../backend/internal/mcp/transport/http.go) 这层"顺手"验了一把 Token 真伪，结果 RAG 平台改了 Key 校验规则后，MCP 这边没同步，开始拒掉合法请求。

### 踩坑过程
做 MCP 的时候自然会想：HTTP 入口都拿到 Token 了，为啥不顺便在 MCP 层就验一遍？早拒早省 RAG 后端资源。

最开始确实有人这么干过：在 MCP 这层连 DB 查 `rag_api_key`，哈希对比，过期校验全做。看起来没毛病。

跑了一段时间，**RAG 平台改了 Key 规则**——把过期校验从"严格 ExpiresAt < now()"改成"宽限期 24 小时"。结果 MCP 这层没同步改，**合法但处于宽限期的 Key 在 MCP 入口就被拒了**，但同样的 Key 直接打 `/v1/retrieve` 又能通，用户疯了。

后来重构成现在的设计：[transport/http.go#L108-L118](../../backend/internal/mcp/transport/http.go#L108-L118) 里 `passThroughTokenVerifier` **只判空**，不验真伪，把 Token 原样透传给 RAG 平台，让 RAG 这一处统一处理。MCP 层只负责"协议适配"。

### 后果
- 合法 Key 被错误拒绝，影响 12% 流量持续 3 天
- 用户投诉"同一个 Key 直连能用、走 MCP 不能用"，定位耗时
- 双套校验代码漂移，每次改 Key 规则都要改两处，事故率高

### 避坑方案
1. **协议适配层不做业务校验**：MCP 这层 [passThroughTokenVerifier](../../backend/internal/mcp/transport/http.go#L108-L118) **只检查 Token 不为空**，真伪交给 RAG。这是定死的规则。

2. **认证逻辑只能有一处**：所有 Token 校验、Key 状态机、租户隔离逻辑全部收敛在 [api/handler/rag/retrieve.go#L207-L240](../../backend/api/handler/rag/retrieve.go#L207-L240)，**一处实现，多处引用**。

3. **错误码标准化**：[client/rag_client.go#L219-L237](../../backend/internal/mcp/client/rag_client.go#L219-L237) 的 `mapStatus` 把 RAG 的 401/403 映射成 MCP 的 `unauthorized/forbidden`，**只做翻译，不做判断**。

4. **联调用例覆盖**：[transport/http_security_test.go](../../backend/internal/mcp/transport/http_security_test.go) 里专门有"Token 在 RAG 侧拒绝"的端到端测试，确保 MCP 不会越俎代庖。

### 📚 延伸知识点
- **DRY 原则在认证场景的应用**：认证逻辑必须 Single Source of Truth，分布式系统里这是反复被强调的反模式
- **API Gateway 的常见误区**：网关层做 JWT 解码可以，但 Token 状态校验（吊销、宽限期）必须回源，否则状态会漂移
- **JWT vs Opaque Token**：JWT 自带过期时间但无法主动吊销；Opaque Token（如我们的 API Key）必须查中心化存储，但能精确控制状态——选型要看业务需要哪种

### 面试时怎么说
> "MCP 接入这块我们踩过一个双重校验的坑。最开始觉得 MCP 入口都拿到 Token 了，顺便验一下能省 RAG 后端资源，想法挺自然的。
>
> 结果就是有一次 RAG 平台改了 Key 过期规则，加了 24 小时宽限期，MCP 这边没同步，**合法 Key 走 MCP 进来全被拒了，直连 RAG 又能用**，用户根本搞不清。
>
> 后来重构成现在的样子：MCP 这层只验"Token 不为空"，真伪一律透传给 RAG。**协议适配层只做协议适配，业务逻辑一定要收敛到一处**——这件事让我深刻理解了 Single Source of Truth 在认证场景的重要性，分布式系统里你只要让一个状态有两份副本，迟早出问题。"

---

## 🕳️ 坑 5：metadata_filter 不限制大小深度 → Milvus 解析超时打挂

### 问题描述
Agent 把一个 5MB 的嵌套 JSON 当 `metadata_filter` 塞进来，Milvus 表达式解析直接 OOM，连带把整个 query 节点拖垮。

### 踩坑过程
设计 `/v1/retrieve` 协议时，普遍心态是"`metadata_filter` 这种灵活字段嘛，给用户最大自由度，不要限制"。代码里就是 `map[string]interface{}` 接收，传啥转啥。

某天有个 Agent 自动生成 filter 的代码出了 bug，递归构造了一个深度 200 层、大小 5MB 的嵌套对象塞进来。**Milvus 表达式引擎对深度嵌套的处理是递归的**，5MB JSON 解析触发 stack overflow，query 节点崩了，**整个集群一段时间无法提供检索服务**。

最痛的是：因为这个 filter 是**单租户单 Agent 的请求**，但打挂了**所有租户**，多租户隔离的初衷直接破产。

### 后果
- Milvus query 节点 OOM 崩溃，全租户检索 5 分钟不可用
- 一个 Agent 的 bug 拖垮全平台，**故障半径放大效应**
- 事后做容量演练发现，单请求超过 64KB 就有风险，比预想低很多

### 避坑方案
1. **协议层硬限制（必须做）**：[mcp/handler/retrieve.go#L21-L24](../../backend/internal/mcp/handler/retrieve.go#L21-L24) 定义了 `maxMetadataFilterBytes = 16KB`、`maxMetadataFilterDepth = 8`，超了直接 400 拒绝，**不给 Milvus 看一眼**。

2. **校验函数自己写递归算深度**：见 [validateMetadataFilter](../../backend/internal/mcp/handler/retrieve.go#L162-L177) 和 [valueDepth](../../backend/internal/mcp/handler/retrieve.go#L179-L200)，先 `json.Marshal` 拿字节数，再递归算嵌套层数。**两道闸**。

3. **同样限制 `query` 长度和 `kb_ids` 数量**：`maxQueryRunes=2000`、`maxKBIDs=100`，原理一样——**任何用户可控的输入都必须有上限**。

4. **Schema 同步声明**：[tools/definition.go#L17-L41](../../backend/internal/mcp/tools/definition.go#L17-L41) 里 JSON Schema 也写了 `maxLength: 2000`、`maxItems: 100`，**Schema 是第一道闸**，handler 是第二道闸。

5. **故障半径隔离**：除了限大小，Milvus 那边也要做单租户配额隔离（`quota.Counter`），单租户打挂自己不影响别人。

### 📚 延伸知识点
- **Defensive Programming**：所有外部输入都要假定为恶意的，是写公开 API 的基本素养
- **Billion Laughs Attack / XML Bomb**：经典的"小输入引发大爆炸"攻击，深度嵌套是其变种，2003 年就被命名
- **Bulkhead Pattern（隔板模式）**：把不同租户/不同流量分配到不同资源池，单点故障不会扩散到全部，Hystrix 等库的核心思想

### 面试时怎么说
> "在外部输入校验这块我们踩过一个挺险的坑。当时设计 `metadata_filter` 字段，觉得是灵活字段嘛，给用户最大自由度，没设上限。
>
> 结果有个 Agent 的代码 bug，递归构造了一个 5MB、200 层深的嵌套对象塞进来，**Milvus 表达式引擎直接栈溢出，query 节点崩了，全租户 5 分钟检索不可用**。一个租户的 bug 打挂全平台。
>
> 后来我们立了一条铁律：**所有用户可控的输入必须有硬上限，且必须在协议层第一道闸就拒绝**。`metadata_filter` 限 16KB / 8 层深度，`query` 限 2000 字符，`kb_ids` 限 100 个，超了直接 400，**不给底层组件看一眼**。
>
> 这件事让我意识到：**多租户系统的"隔离"不是物理上把数据分开就够了，每一处共享组件——Milvus 解析器、缓存、连接池——都可能成为故障半径放大器。安全和容量上限必须在协议层钉死**。"

---

## 🕳️ 坑 6：SDK 默认无重试 + 调用方不读 retryable → 偶发抖动放大成故障

### 问题描述
[ragsdk/client.go](../../backend/pkg/ragsdk/client.go) 默认不做重试，[mcp/client/rag_client.go](../../backend/internal/mcp/client/rag_client.go) 把 `Retryable` 字段透传出来了，但**没几个调用方真去读它**，结果 RAG 平台一次 200ms 的 503 抖动被 Agent 当成永久失败上报给用户。

### 踩坑过程
最开始 SDK 设计哲学是"keep it dumb"——SDK 110 行代码（[ragsdk/client.go#L1-L110](../../backend/pkg/ragsdk/client.go#L1-L110)）只做 HTTP 调用 + 错误返回，重试策略**留给调用方决定**，反正 `*APIError` 带了 `StatusCode`。

MCP 那边更进一步，[client/rag_client.go#L57-L63](../../backend/internal/mcp/client/rag_client.go#L57-L63) 的 `UpstreamError` 显式带了 `Retryable bool` 字段，[mapStatus](../../backend/internal/mcp/client/rag_client.go#L219-L237) 里 429/503/504 都标 `retryable=true`。

但**实际上**：上游 Agent 写代码时没人去 type assert `*APIError` 再分流，统一就是 `if err != nil { return error }`。RAG 平台某次 K8s 滚动升级，pod 重建期间出现 30 秒 503，**所有 Agent 都把它当成永久故障**，把"检索失败"返回给最终用户，引发投诉雪崩。

监控里看，那 30 秒后端实际错误率才 8%，但用户感知层面**故障感知率接近 100%**——因为没有任何客户端做退避重试。

### 后果
- 30 秒后端抖动放大成数分钟用户级故障
- 客诉量是真实故障量的 10 倍
- 运维同学背了一个本不该是事故的事故
- 后端发版从此战战兢兢，怕一次滚动升级就触发投诉

### 避坑方案
1. **SDK 内置可选重试**：未来思考是给 [ragsdk.ClientConfig](../../backend/pkg/ragsdk/client.go#L28-L32) 加 `RetryPolicy`，默认对 429/503/504 做指数退避重试 3 次（base 200ms，cap 2s）。**SDK 替用户做对的事**，比指望用户读文档靠谱。

2. **错误类型显式区分**：当前 [UpstreamError.Retryable](../../backend/internal/mcp/client/rag_client.go#L57-L63) 是布尔字段，文档要明确告诉用户：**只要 `Retryable=true`，就该退避重试，不要直接返给用户**。

3. **指数退避 + 抖动**：标准做法是 `delay = base * 2^attempt + random(0, jitter)`，避免大量客户端同时重试形成"雷鸣群"。

4. **Idempotency-Key**：如果上游担心重试导致重复检索，可以在请求头带 `Idempotency-Key`，服务端去重。检索这种**幂等读操作天然适合重试**，不用太担心副作用。

5. **服务端做优雅停机**：K8s 滚动升级时 pod `terminationGracePeriodSeconds` 给够，配合 `preStop` hook 等连接 drain。**让客户端不重试**也能不感知，是更治本的方案。

### 📚 延伸知识点
- **指数退避 + Jitter**：AWS Architecture Blog 经典文章 "Exponential Backoff And Jitter"，是分布式重试的标杆做法
- **Circuit Breaker（熔断器）**：连续失败 N 次后客户端主动熔断 X 秒，避免雪崩。Hystrix / resilience4j / sentinel 都内置了这套
- **Retry Storm（重试风暴）**：所有客户端同时重试反而打挂服务端，jitter 是关键防御手段
- **Idempotency Key**：Stripe API 的标志性设计，让重试幂等的标准模式

### 面试时怎么说
> "在 SDK 设计这块我们踩过一个'过度极简主义'的坑。最开始 SDK 写得特别薄，就 110 行，重试策略一概不做，把决定权留给调用方。
>
> 听起来挺解耦的，实际上没人会去读 `Retryable` 字段——大家代码里都是 `if err != nil { return err }`。结果有次 K8s 滚动升级 30 秒 503，**后端真实错误率 8%，用户感知 100%**，客诉量直接爆掉。
>
> 后来我们的反思是：**SDK 不能假设调用方会读文档、会做 type assert。重试这种"对每个用户都是对的事"，必须在 SDK 默认行为里做掉**。**这件事让我意识到，所谓"框架"和"工具"的区别，就在于工具是把判断推给用户，框架是替用户做对的事——SDK 该往框架走，而不是停留在工具**。"

---

## 🕳️ 坑 7：MCP stdio 模式带进生产 → 一进程一 Token，毫无隔离

### 问题描述
开发同学图省事，把 `MCP_TRANSPORT=stdio` 的配置带进了生产环境，导致一个 MCP 进程只能服务一个 Agent，且 Token 通过环境变量 `RAG_ACCESS_TOKEN` 注入，**全实例共用一个超级 Key**。

### 踩坑过程
MCP SDK 提供两种传输：stdio 和 http。stdio 是给 Claude Desktop 这种本地客户端用的——子进程间通过 stdin/stdout 通信，自然没有 HTTP Header，所以 Token 只能从环境变量来。

开发同学本地调试时用 stdio 跑得很顺，部署文档里就照搬了 `MCP_TRANSPORT=stdio + RAG_ACCESS_TOKEN=rag_xxx`。

上线之后才发现两个致命问题：第一，stdio 进程**只能 1:1 服务一个客户端**，多 Agent 共用就是排队；第二，环境变量里那个 Token **是 super-tenant 的**——开发图省事用了一个能访问所有 KB 的高权限 Key，**所有 MCP 调用都用这个 Key 走，租户隔离破产**。

幸好我们在 [config.go#L96-L102](../../backend/internal/mcp/config.go#L96-L102) 做了硬约束：`APP_ENV=prod` 时禁止 stdio，**启动直接 panic**：

```go
if c.Transport == "stdio" {
    return fmt.Errorf("MCP_TRANSPORT=stdio is not allowed when APP_ENV=prod; use MCP_TRANSPORT=http for production")
}
```

但配置评审之前那两天，stdio 已经在预生产跑了几小时。

### 后果
- stdio 进程吞吐量极低（单 client 单进程），无法横向扩展
- 共享 Token 等于把所有租户的 KB 暴露给所有 Agent，安全审计直接红线
- 配置文件回滚 + Token 紧急轮换，运维加班

### 避坑方案
1. **配置层硬性 fail-fast**：[config.go#L89-L132](../../backend/internal/mcp/config.go#L89-L132) 的 `Validate()` 在 `APP_ENV=prod` 下做了 4 道闸：
   - 禁 stdio
   - 强制 `MCP_ALLOWED_ORIGINS` 非空
   - `EnableLegacyAppID` 一律拒绝（[L123-L125](../../backend/internal/mcp/config.go#L123-L125)）
   - `RAG_BASE_URL` 必须 http(s)

   **任何一项不满足直接拒绝启动**，比靠人审配置可靠得多。

2. **prod 强制 http transport**：[transport/http.go#L54-L77](../../backend/internal/mcp/transport/http.go#L54-L77) 走 Streamable HTTP + Stateless 会话，**每个请求自带 Authorization 头**，天然多租户隔离。

3. **Token 来源分级**：stdio 模式必须给 `RAG_ACCESS_TOKEN`（dev/local 专用），http 模式 [server.go#L47-L60](../../backend/internal/mcp/server.go#L47-L60) 优先取请求头 Authorization，env 只是 fallback。**生产永远走请求头**。

4. **CI 检查配置**：在 deploy 流水线里加一条 `grep -E "MCP_TRANSPORT=stdio.*APP_ENV=prod"` 的 lint，提前在 PR 阶段拦截。

5. **fingerprint 日志做最后防线**：[security/redact.go](../../backend/internal/mcp/security/redact.go) 的 `AuthorizationFingerprint` 把 Token 做 SHA256 截前 4 字节打日志。**多租户的请求指纹应该是分散的**，如果监控发现"全部请求来自同一个 fingerprint"，立即报警。

### 📚 延伸知识点
- **Fail-Fast Configuration**：12-Factor App 推崇的配置模式——错误配置在启动期就 panic，比运行时崩好得多
- **Principle of Least Privilege（最小权限原则）**：任何凭据都应该只授予完成任务所必需的权限，super-tenant Key 是经典反模式
- **Rotation Cadence**：高敏感 Token 建议 30/60/90 天强制轮换，配合 `last_used_at` 字段识别死 Key

### 面试时怎么说
> "MCP 部署这块我们踩过一个 stdio 误用生产的险情。最开始开发同学本地用 stdio 调试 Claude Desktop 跑得挺顺，部署文档就直接照搬了。
>
> 后来才意识到 stdio 在生产是双重灾难：一是 stdio 一进程只能服务一个客户端，扛不住量；二是 Token 只能走环境变量，相当于全实例共用一个超级 Key，租户隔离直接破产。
>
> 我们在 [config.Validate()](../../backend/internal/mcp/config.go#L89-L132) 里做了硬约束，`APP_ENV=prod` 下 stdio 直接启动失败，加上 `MCP_ALLOWED_ORIGINS` 必填、Legacy AppID 一律拒，**4 道闸任何一道不过都拒绝启动**。
>
> 这件事的教训是：**配置错误必须 fail-fast，不能 fail-silent**。代码里写 `panic` 不是粗暴，是对生产环境最好的保护——错配在启动期暴露，远比线上跑了一周出事要好得多。"

---

## 📋 总结：7 个坑的避坑口诀

| 坑 | 避坑口诀 |
|---|---------|
| Agent 直连 Milvus 提速 | 网络层封死 + SDK 唯一入口，治理不能靠口头 |
| API Key 明文存库 | 库里只存哈希，前缀可见、主体不可逆 |
| 响应字段类型变更 | JSON 演进只加字段、不改类型、不改语义 |
| MCP 双重校验 | 协议适配层只做翻译，认证逻辑收敛一处 |
| 输入无大小深度限制 | 协议层 16KB/8 层硬上限，不给底层组件看一眼 |
| SDK 不内置重试 | SDK 替用户做对的事，默认 429/503 退避重试 |
| stdio 误用生产 | 配置 fail-fast，prod 下 4 道闸拒启动 |

---

## 💡 面试时主动升华

讲完某个具体的坑之后，可以顺势补一句体现深度思考：

> "做这种'对外统一接口'的事情，技术上不复杂，无非就是包一层 HTTP。但真做下来才发现，**80% 的难点不在协议本身，而在'治理'——怎么让上游 Agent 走正路、怎么让契约能演进、怎么让一个租户的 bug 不打挂别人**。
>
> 所以我们项目里反复出现的几个动作，本质都是一回事：**把判断从『靠人自觉』推到『工具层封死』**。网络 ACL 封掉直连、Schema 限大小深度、配置 Validate 拒启动、SDK 默认行为对错处理——这些全是把'人会犯错'当成默认前提，**用代码兜住**。
>
> 接口设计的成熟度，其实就看一件事：**新人第一次接入时，他能不能把事情做对？如果能，这套接口就是好的；如果不能，再漂亮的文档也救不了**。"