# 海量文档异步处理与 Redis Stream 分布式调度 - 8 个最容易踩的坑与避坑指南

---

## 前言

把文档解析、向量化挪到 Redis Stream 异步消费听起来就一句话的事——「上传只入队、消费做重活」。但只要你真做过，从消息体设计、状态机、ACK 时机、重试退避，到多副本扫描器和 PDF 解析卡死，**每一项都能让你掉头发**。

下面这 8 个坑按踩中概率从高到低排序，配合主文档 [09-海量文档异步处理与RedisStream分布式调度-面试题和参考答案.md](./09-海量文档异步处理与RedisStream分布式调度-面试题和参考答案.md) 食用更佳。

---

## 🕳️ 坑 1：先入队后落库 → Redis 抖一下，整批文件就进了"薛定谔状态"

### 问题描述
HTTP handler 拿到文件后**先 `XAdd` 入队、再写 `kb_ingest_job`**，结果 Redis 短暂抖动那一瞬间，消息进了 Stream 但 DB 写失败，用户那边返回 500 重试，消费者却查不到这个 job。

### 踩坑过程
最开始的普遍心态是「**消息进队列了就万事大吉**」——队列异步嘛，写 DB 这种慢操作放后面。手起代码就是 `Publish → Create(job) → Create(document)`。

测试环境跑得很顺，没什么问题。**预发环境压测时**第一次出事：测试同学批量上传 200 份文件，正赶上 Redis 主从切换，0.3 秒抖动期间有 17 条消息成功 `XAdd`、但 DB 事务因为连接断开回滚了。消费者 17 次拿到消息，17 次 `KBIngestJobDao.GetByID` 返回 `record not found`，handler 直接 `return nil`，消息就这么默默丢了。

用户那边看到 500，以为没成功，重传了一次，结果**这次成功的反而和那 17 条孤儿消息撞了车**——同一个 document_id 被消费两遍，Milvus 里写了两份重复 chunk。

排查的时候就是奇怪：HTTP 报错的文件最后居然有 chunk 入库，重传的反而少了几条。直到翻 Redis 慢日志才看出 `XAdd` 时间和 DB 报错时间差了几十毫秒。

### 后果
- **数据一致性破坏**：~8% 的失败上传后续重传时出现重复 chunk，召回时同一段文字命中两遍
- 用户信任崩塌：客户问「我重传是不是会重复？」我们答不上来
- 排障花了**整整一天**，因为现象诡异：HTTP 失败的反而成功
- 后续做去重相当于又欠了一笔技术债

### 避坑方案

1. **以 DB 为唯一真相源**：`Create(document)` 和 `Create(job)` 必须**先在事务里跑完、提交后**才调 `PublishKnowledgeIngest`，参考 [handler.go#L521-L545](../../backend/api/handler/kb/handler.go#L521-L545) 的写法。

2. **入队失败必须显式回写状态**：见 [handler.go#L546-L562](../../backend/api/handler/kb/handler.go#L546-L562)，`Publish` 报错就立刻把 job 改成 `failed` + `last_error_code=enqueue_error`，不能留 pending 等扫描器（扫描器只看 retrying）。

3. **消费侧再加一道幂等闸门**：[consumer.go#L60-L72](../../backend/internal/ragqueue/consumer.go#L60-L72) 的 `shouldHandleKnowledgeJob` + `ClaimForProcessing` 会检查 job 是否存在 / 状态是否还可处理，**消息和 DB 状态对不上就直接 ACK 丢弃**。

4. **不要试图用 `MULTI/EXEC` 把 Redis 和 MySQL "一起"事务化**：分布式事务的代价远远大于"先 DB 后队列"的简单语义。

> 详见主文档 [Q1：整体设计](./09-海量文档异步处理与RedisStream分布式调度-面试题和参考答案.md)

### 📚 延伸知识点
- **Outbox Pattern**：业界经典的"DB → 队列"投递模式，DB 事务里写一张 outbox 表，独立进程扫表转投，杜绝双写不一致
- **At-Least-Once vs Exactly-Once**：Redis Stream 是 at-least-once，必须配合消费侧幂等才能逼近 exactly-once
- **CDC（Change Data Capture）**：Debezium 这类工具可以直接订阅 MySQL binlog 转投到队列，把双写问题彻底消除

### 面试时怎么说
> "在异步任务的入队顺序上我们踩过一个挺典型的坑。最开始想当然觉得'消息进队列就稳了'，所以是先 XAdd、再写 DB。
>
> 测试都没问题，预发压测的时候 Redis 抖了 0.3 秒，那一瞬间有 17 条消息进了 Stream 但 DB 回滚了。消费者拿到消息查不到 job 就默默 return 了，用户重传又撞车，同一份文件 chunk 被写了两遍。
>
> 排查那天我整个人是懵的——HTTP 报错的反而入库了，重传的反而少。最后是翻 Redis 慢日志看时间戳才对上的。
>
> 后来我们就把顺序倒过来：**DB 事务先提交，提交成功才入队，入队失败就把 job 直接改成 failed**。这件事让我意识到：**所谓异步化，本质是把'数据落地'和'任务派发'解耦——但顺序错了就什么都解耦不了，反而多一层不一致**。"

---

## 🕳️ 坑 2：消息体塞 100MB PDF 内容 → Redis 内存爆炸 + XAdd RTT 飙到秒级

### 问题描述
为了「消费者不用再去查 DB / OSS」，把整个文档文本塞进消息 payload，结果一份 50MB PDF 让 Redis 单 stream 占用 5GB+ 内存，`XAdd` RTT 从毫秒级涨到 2 秒。

### 踩坑过程
最开始设计 payload 字段时，开发同学很自然地想：「消费者拿到消息就能直接处理多好啊，省得再查一遍 DB」。所以第一版 payload 里塞了 `raw_text`、`chunks_config`、`embedding_config` 等等，恨不得把整个上下文都打包进去。

业内**这个坑特别普遍**——尤其是从 Kafka 转过来的同学，习惯把消息当"自包含的事件"来设计。但 Redis Stream 不是 Kafka：**单条消息越大，XADD/XREAD 的 RTT 越高，整个 Redis 实例的内存占用增长是线性的**，而且 Redis 是单线程，大消息直接拖慢所有其他业务（缓存、限流也用着这个 Redis）。

我们项目从一开始就走了"指针消息"的设计——`KnowledgeIngestPayload` 只有 7 个字段（[queue.go#L23-L32](../../backend/internal/ragqueue/queue.go#L23-L32)）：`user_id / kb_id / document_id / job_id / file_path / file_type / collection`。文件在 OSS、配置在 DB，**消息只承担"指针"作用**。

但有个团队踩过这个：他们把 chunk 切完之后逐个塞进消息发到队列做 Embedding，结果 Redis 内存几小时就涨到 80%，运维同学半夜起来扩容。

### 后果
- Redis 内存占用线性飙升，**100MB 文档 = 100MB+ 消息 + 复制副本 = 200MB+ 实际占用**
- `XADD` RTT 从 1-2ms 涨到 1-2s，HTTP 上传接口直接超时
- Redis 单线程被拖慢，连带**缓存、限流、队列**全部受影响
- 网络带宽被消息流量打满

### 避坑方案

1. **消息只放指针，不放内容**：参考 [queue.go#L23-L32](../../backend/internal/ragqueue/queue.go#L23-L32)，`document_id + file_path` 就够了，文件让消费者自己去 OSS 读。

2. **强制单条消息上限**：建议 < 1KB，超过就拒绝入队。Redis Stream 没有原生限制，必须在 Publish 层 `len(payloadJSON) > 1024 → return ErrPayloadTooLarge`。

3. **`MaxLen` 加 Approx=true 控总量**：[redis_stream.go#L67-L72](../../backend/internal/ragqueue/redis_stream.go#L67-L72) 设 `MaxLen=10000 Approx=true`，最坏情况内存占用上限可控。

4. **业务数据放 OSS / DB，不靠队列搬运**：队列是"通知"不是"运输"。

### 📚 延伸知识点
- **Claim Check Pattern**：EIP（企业集成模式）经典模式，大消息存对象存储，队列里只传引用 ID，正是我们采用的设计
- **Redis Stream 内部 Listpack/Radix Tree 存储**：消息越大越倾向退化为 Radix Tree，进一步增加内存开销
- **Kafka Compact Topic**：如果非要塞大消息，Kafka 至少有专门的存储分层，Redis Stream 没有

### 面试时怎么说
> "在消息体设计上我见过一个非常普遍的坑。最开始很多人会觉得'消息得自包含才好用'，把文件内容都塞进去，消费者直接拿来就能干活。
>
> 但 Redis Stream 不是 Kafka，单条消息一大，整个 Redis 内存就线性涨，Redis 又是单线程，大消息会把缓存、限流这些共用 Redis 的业务全部拖慢。
>
> 我们项目一开始就只把 `document_id`、`file_path` 这种 7 个字段塞进 payload，文件在 OSS，配置在 DB，消息只是个指针。**这个模式叫 Claim Check Pattern，企业集成里早就有定论。**
>
> 这件事让我意识到：**队列是用来'通知'的，不是用来'运输'的。** 把队列当 RPC 用，迟早翻车。"

---

## 🕳️ 坑 3：错误一律重 3 次 → 一份坏 PDF 把 retry 预算耗光

### 问题描述
重试策略写成"全部错误重 3 次"，结果某用户上传一份加密 PDF，每次重试都在解析阶段失败，耗了 3 个 worker × 3 次解析时间，**一直没释放 backlog**。

### 踩坑过程
最开始做重试的普遍想法是「重试嘛，错了就再来一次，反正最多 3 次」。代码也写得很朴素：`if err != nil { retry(payload) }`。

某天客户上传了一份**密码保护的 PDF**，`pdf.Open` 失败 → 重试 → 又失败 → 再重试 → 还失败 → 标记 dead。表面看流程没问题，但实际损失很大：

- 这份 PDF 一共占了 worker 3 次（每次几秒），消耗了 3 个重试预算
- 退避时间 500ms / 1s / 2s 累积下来是 3.5 秒，期间 backlog 没下降
- 监控里 `error_code=parse_error` 没有专门标签，混在 `unknown` 里，**告警敏感度被冲淡**

更糟的是有用户**批量上传了 30 份加密 PDF**，30 × 3 = 90 次无效解析，整个池子被这种"必死的任务"占满几分钟，正常文件全在排队。

后来我们就把错误分类做细了。看 [consumer.go#L454-L478](../../backend/internal/ragqueue/consumer.go#L454-L478)：分 `payload / parse / embedding / milvus / unknown` 5 类，[isKnowledgeIngestRetryable](../../backend/internal/ragqueue/consumer.go#L480-L491) 里 **`payload` 和 `parse` 永不重试**，因为这两类问题重 100 次还是错。

### 后果
- **重试预算被永久错误耗光**，真正的瞬时错误（网络抖动）反而没机会重
- worker 池被无效任务占据，正常吞吐下降
- `dead` 状态的 job 看起来都是 3 次重试失败，**根因看不出来**——是网络抖了还是文件本身坏了？
- Embedding 第三方 API 被无效请求打到（如果错误是后置的）

### 避坑方案

1. **错误分类决定可重试性**：参考 [consumer.go#L454-L478](../../backend/internal/ragqueue/consumer.go#L454-L478) 的 5 类分法 + [hasTransientFailureSignal](../../backend/internal/ragqueue/consumer.go#L493-L518) 关键词匹配（`timeout / connection refused / EOF / network`）。

2. **永久错误立刻 `failed`，不进 retrying**：见 [consumer.go#L226-L247](../../backend/internal/ragqueue/consumer.go#L226-L247)，payload/parse 错误直接 `UpdateFailureState(failed)`，连一次重试都不浪费。

3. **指数退避 + 抖动**：[calculateKnowledgeRetryBackoff](../../backend/internal/ragqueue/consumer.go#L547-L565) 用 `base * 2^(retry-1)` + `0.8~1.2` 抖动避免雷击，最大 5 分钟封顶。

4. **`error_code` 必须打到 metric 标签**：[rag_metrics.go#L174-L180](../../backend/internal/observability/metrics/rag_metrics.go#L174-L180) 的 `rag_ingest_jobs_total` 带 `error_code` 标签，**不分类的话告警就只能笼统报"失败率高"，没法定位**。

### 📚 延伸知识点
- **Poison Message / Dead Letter Queue**：永久不可处理的消息要专门走死信队列归档供人工分析，不能反复重投打死队列
- **Exponential Backoff with Jitter**：AWS 早期博客 [Exponential Backoff And Jitter](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/) 是经典论述，jitter 是关键
- **Circuit Breaker**：当某类错误连续触发时直接熔断，停止消费一段时间，避免重试雪崩

### 面试时怎么说
> "重试这块我们踩过一个坑。最开始写得很朴素——错了就重 3 次，没分类。
>
> 结果有用户传了批加密 PDF，每次都在 `pdf.Open` 那里挂，3 次重试 + 退避叠起来好几秒，**worker 全卡在这种必死的任务上**，正常文件排队。后来批量上传 30 份加密 PDF，整个池子直接被打满。
>
> 我们就把错误分类细化了：`payload` 和 `parse` 这种**永久错误一次都不重**，直接 failed；`embedding` 和 `milvus` 这种**只有匹配 timeout、connection refused 这种瞬时关键词才重**。retry 预算是稀缺资源，得花在刀刃上。
>
> 这件事让我明白：**重试不是'再来一次的乐观主义'，是'判断失败是不是值得再来一次的诊断学'。**"

---

## 🕳️ 坑 4：handler 失败不 ACK 等自动接管 → 消息卡 pending list 成孤儿

### 问题描述
按 Redis Stream 教科书写法「失败不 ACK，让 Redis 自动重投」，结果项目根本没跑 `XPENDING+XCLAIM` 接管协程，**消息永远卡在 pending list 里没人处理**。

### 踩坑过程
最开始读 Redis Stream 文档时，普遍印象就是「Stream 自带 consumer group + pending list，失败的消息会自动给别人处理」。看 [redis_stream.go#L182-L192](../../backend/internal/ragqueue/redis_stream.go#L182-L192) 我们也是这么写的——handler 失败就直接 return，不调 `XAck`。

跑了一段时间，看监控发现 `rag_consumer_pending` 这个指标稳定在几百，**而且只增不减**。一开始以为是消费速度跟不上，扩了个 pod 没用；调大 ants 池也没用。

挖代码挖了半天才发现：**[StartMetricsReporter](../../backend/internal/ragqueue/redis_stream.go#L199) 这个方法只上报指标，根本没人去 XCLAIM**。再翻一遍代码，整个 ragqueue 包**没有任何地方调用 XPENDING / XCLAIM**。失败的消息确实在 Redis pending list 里待着，但一直没消费者去"认领"。

那"失败重试到底是怎么工作的"？答案是**绕了远路**——靠 DB 里 `kb_ingest_job` 的 `next_retry_at` + 一个独立扫描器（[startKnowledgeRetryCompensator](../../backend/internal/ragqueue/consumer.go#L324-L411)）到点重新调 `PublishKnowledgeIngest` 投回 Stream。Stream 里的 pending 实际是**孤儿**，只能等 `MaxLen` 把它们裁掉。

更糟的是 [`StartRetryCompensator`](../../backend/internal/ragqueue/consumer.go#L109-L111) 这个入口函数虽然定义了，**main.go 里居然没启动它**（参见 [main.go#L83-L102](../../backend/cmd/rag-server/main.go#L83-L102)）。也就是说**线上目前真的没人在自动重试 retrying 的 job**，全靠管理后台手动点。

### 后果
- Redis pending list **持续增长**，监控指标长期偏高，告警噪音
- Worker 进程崩溃留下的消息**永远不会自动恢复**，必须等 `MaxLen` 裁剪丢弃
- DB 里 `processing` 状态的 job 进程崩了之后**卡死**，无法判断是真在跑还是已经死
- 重试链路实际依赖 DB 扫描器——而扫描器还没接进 main.go

### 避坑方案

1. **要么 ACK 后用 DB 重试，要么不 ACK 用 XCLAIM 接管，二选一别混用**：我们项目实际选择了前者——**handler 内部 catch 错误后写 DB 状态，然后让 reader 调 ACK**（消息生命周期在 ACK 那一刻终结），而不是依赖 Stream 的 pending 机制。

2. **必须接 `StartRetryCompensator` 进 main**：在 [rag-server/main.go](../../backend/cmd/rag-server/main.go#L83-L102) 启动消费者那段加 `go ragqueue.StartRetryCompensator(consumerCtx)`，这是**目前已知必须补的洞**。

3. **加 stale-claim reaper（如果保留 Stream 重试）**：另起协程每分钟扫 `XPENDING idle > 5min` 的消息，`XCLAIM` 给当前 consumer 重处理。

4. **`processing` 状态加超时探测**：DB 里 `started_at + 10min < now()` 的 processing job 视为僵尸，扫描器自动改 retrying。

### 📚 延伸知识点
- **Visibility Timeout（SQS 概念）**：消息被 consumer 拿走后有个"可见性超时"，超时未 ACK 自动回到队列，这才是 Stream pending 该有的样子
- **Heartbeat + Lease**：worker 拿任务前先抢 lease，定期 heartbeat 续约，宕机自动释放，比单纯 visibility timeout 更精确
- **僵尸进程检测**：systemd / k8s liveness probe 在外层兜底，进程级别的死锁外部能感知到

### 面试时怎么说
> "Redis Stream 的 pending list 我们其实掉过坑。最开始按教科书写法'失败不 ACK，让 Stream 自动重投'，跑了一阵发现 `consumer_pending` 这个指标只涨不降。
>
> 挖代码才发现：**我们项目根本没起 XCLAIM 接管协程**，pending 里的消息变成了孤儿，只能等 MaxLen 裁掉。重试实际是靠 DB 里 `next_retry_at` + 扫描器绕过去的——但扫描器代码定义了又**没接进 main.go**，目前线上靠后台手动点重试兜着。
>
> 这件事让我意识到：**很多框架特性看着开箱即用，其实需要你额外配套补齐**——pending list 不是免费的，它是张支票，得自己写代码兑现。所以现在我做异步任务的第一件事，是把'失败到底走哪条路'画清楚，要么走 ACK 后 DB 重试，要么走 XCLAIM 接管，**两条路绝不混用**。"

---

## 🕳️ 坑 5：单文件 100MB PDF 解析 20 秒 → 1000 worker 全卡死、新消息进不来

### 问题描述
某次用户批量上传 20+ 份 100MB+ PDF，单文件解析就要 20+ 秒，1000 个 worker 全卡在 PDF Reader 上，**新消息根本拉不进来**。

### 踩坑过程
最开始的普遍想法是「ants 池 1000 容量够大了，不可能用满」。看 [redis_stream.go#L40](../../backend/internal/ragqueue/redis_stream.go#L40) 的 `ants.NewPool(1000)`，单实例 1000 并发，听起来富余度爆表。

某天大客户突然发起一次"知识库迁移"，**一次性传了 50 份 100MB+ PDF**。我们的 [extractTextFromPDF](../../backend/internal/ragqueue/consumer.go#L633-L664) 用的是 `github.com/ledongthuc/pdf`，纯 CPU 解析、没有超时控制、单文件能跑 20-30 秒。

50 份 × 20 秒就是 1000 worker × 1 秒处理时间被这一批占满。**真正打挂的是后续来的小文件**——它们还在 Stream 里堆着，反而是明明几百 KB 的 markdown 在排队等 100MB PDF 解析完。

监控上：`rag_consumer_backlog` 飙到 5 万，`rag_ingest_duration_seconds_bucket{le="30"}` 大量样本，p99 飙到 60s+。值班同学一脸懵——QPS 没涨啊，怎么 backlog 就爆了？

应急是用 [PauseKnowledgeIngest](../../backend/internal/ragqueue/queue.go#L133) 这个反向开关临时暂停消费（[consumer.go#L46-L49](../../backend/internal/ragqueue/consumer.go#L46-L49) 入口检查 paused 直接 return nil），让卡住的 worker 跑完之后再放开。

### 后果
- 1000 worker 池**有效利用率掉到 5%**（剩下 95% 在等 IO）
- 新消息 backlog **5 分钟涨 5 万**，p99 入库延迟从 3 秒飙到 60+ 秒
- 普通用户的小文件**完全被大文件淹没**，体验灾难
- 应急 pause 操作只在**单 pod 生效**——`ingestPaused` 是进程内变量（[queue.go#L113-L115](../../backend/internal/ragqueue/queue.go#L113-L115)），多副本必须逐 pod 调

### 避坑方案

1. **HTTP 上传层加文件大小硬上限**：在 handler 接收文件时拒掉 > 50MB 的，**别让大文件进队列**——前置拦截比后置补救便宜 100 倍。

2. **PDF 解析必须有 ctx 超时**：`extractTextFromPDF` 当前**完全没用 ctx**（[consumer.go#L567-L568](../../backend/internal/ragqueue/consumer.go#L567-L568) 直接 `_ = ctx`），需要改造成 `context.WithTimeout(ctx, 30*time.Second)` 配合 select 超时退出。

3. **大文件走专用 stream**：可以分 `knowledge_ingest_small` / `knowledge_ingest_large` 两个 stream，大文件用更小的池子串行处理，**避免大文件吃光小文件的资源**。

4. **Pause 改成分布式开关**：用 Redis Pub/Sub 广播或 Hash 定时拉取，多副本同步。当前是进程内 bool（[queue.go#L113-L115](../../backend/internal/ragqueue/queue.go#L113-L115)），是**已知缺陷**。

5. **告警标签要分大小桶**：`rag_ingest_duration_seconds` 的 buckets 从 0.1 到 300（[rag_metrics.go#L185](../../backend/internal/observability/metrics/rag_metrics.go#L185)），但**没有按 file_size 维度切**，看不出来到底是流量涨了还是单任务变重了。

### 📚 延伸知识点
- **Bulkhead Pattern**：把不同特征的任务放进隔离的池子，避免互相挤占资源——这是微服务韧性设计的经典模式
- **流式解析 vs 全量加载**：100MB PDF 完全可以分页流式处理，不必一次性塞进内存
- **Workload Isolation**：k8s 里就有 PriorityClass、ResourceQuota 这种工具，应用层也该有"任务优先级"概念

### 面试时怎么说
> "我们线上真出过一次大文件打挂队列的事故。**最开始觉得 1000 worker 容量绝对够**，单实例并发上限定得很高。
>
> 结果客户一次性传了 50 份 100MB+ PDF，单文件解析 20-30 秒，1000 个 worker 集体卡在 PDF Reader 上，**普通小文件全在后面排队**，backlog 五分钟涨 5 万。
>
> 当时应急是用了一个 `PauseKnowledgeIngest` 的反向开关，让卡住的 worker 跑完之后再放开。后来我们就做了几件事：HTTP 层加文件大小限制、PDF 解析加 ctx 超时、大小文件分两个 stream 隔离。
>
> 这件事让我意识到：**'池子大' ≠ '能扛'。如果池子里所有任务的'重量'差异巨大——比如一份是 1KB markdown、一份是 100MB PDF——那有效容量是被最重的任务决定的，不是池子大小决定的**。"

---

## 🕳️ 坑 6：`ClaimForProcessing` 之后崩进程 → job 永远卡在 processing

### 问题描述
worker 拿到消息、刚把 job 改成 `processing`、还没处理完进程就 OOM 了，**这条 job 在 DB 里永远是 processing 状态**——后台看着像在跑，实际死透了。

### 踩坑过程
最开始设计 `ClaimForProcessing` 这个动作的时候，**想得很简单**：消息进来 → 占位 → 处理 → 完成 / 失败。状态机里 [pending → processing](../../backend/internal/model/kb_ingest_job.go#L39-L43) 是合法转移，看 [consumer.go#L64-L72](../../backend/internal/ragqueue/consumer.go#L64-L72) 也是这么干的。

但**没想清楚的是"中间崩了怎么办"**。我们项目目前没有 stale-claim 检测，进程 OOM / 主机重启 / panic 没 recover，job 就永远卡在 processing。监控 `kb_ingest_job_status_processing_count > 0` 这个指标长期偏高，但你**根本分不清是真的有正在跑的、还是僵尸**。

[consumer.go#L226](../../backend/internal/ragqueue/consumer.go#L226) 的 `handleKnowledgeIngestFailure` 只在 handler 自己 return error 时被调用——进程被 OOM Killer 干掉是不会走这条路径的。**Kubernetes 重启 pod 之后，新 worker 拉到的是别的消息**（因为 Stream 已经把这条派给了"之前那个 consumer name"），死掉的 consumer 名下的 pending 消息，没有 XCLAIM 也没有 stale 扫描，就这么悬着。

我们后来加过一个临时脚本：扫 `started_at < now() - 30min` 且 `status = processing` 的 job 强制改回 `retrying`，但这是**人肉运维**，不是自动化。

### 后果
- DB 里 `processing` 状态长期堆积**僵尸 job**，监控失真
- 用户在前端看「该文档处理中」永远转圈圈，**没有任何超时反馈**
- 文档状态和 job 状态不一致：`kb_document.status=processing` 但实际没人在处理
- 偶发的 OOM 事故变成永久数据状态错误

### 避坑方案

1. **DB 层加超时探测扫描器**：每分钟扫 `status='processing' AND started_at < now() - 10min`，强制改 `retrying`，重新入队。

2. **handler 入口 defer recover**：[ragqueue/consumer.go#L45](../../backend/internal/ragqueue/consumer.go#L45) 应该加 `defer func() { if r := recover(); r != nil { handleKnowledgeIngestFailure(...) } }()`，**panic 也走失败链路**而不是飞掉。

3. **k8s preStop hook**：pod 终止前优雅停消费 + 把 inflight 的 job 状态回滚为 retrying，下次 pod 起来直接接管。

4. **配合 XPENDING 双重保险**：DB 超时扫描 + Stream pending 接管两条路同时跑，互为兜底。

### 📚 延伸知识点
- **Lease-based locking**：每个 worker 拿 job 时同时 set 一个 lease（带 TTL），失效后自动释放，比纯靠状态字段更准
- **Crash-only Software**：进程崩溃应该是常态而不是异常，所有状态都要可恢复，这是 Pat Helland 的经典论文
- **Saga Pattern**：长事务用补偿动作回滚而非锁定状态，每一步都要"可回滚"

### 面试时怎么说
> "进程崩在 processing 状态这事，我们踩得挺深。最开始设计状态机时，**想着 pending → processing → completed/failed 流程很清晰**——但完全没考虑'processing 中间进程死了'这种场景。
>
> 线上有一次 pod OOM，几十个 job 卡在 processing，前端用户看到永远在'处理中'。我们当时是写了个临时脚本扫 started_at 超过 30 分钟的 processing 强制改回 retrying，**纯人肉运维**。
>
> 后来我们补了三件事：handler 入口加 defer recover、DB 加超时扫描器、规划用 XPENDING 做双保险。
>
> 这件事让我明白：**状态机不是流程图，是'每个状态都得能从崩溃中恢复'的契约**。每个'中间态'必须配套一个'兜底机制'，不然就是定时炸弹。"

---

## 🕳️ 坑 7：`StartRetryCompensator` 写完忘记接 main → 重试链路名存实亡

### 问题描述
重试代码写得很完整，[startKnowledgeRetryCompensator](../../backend/internal/ragqueue/consumer.go#L324-L411) 逻辑也对，但 [main.go](../../backend/cmd/rag-server/main.go#L83-L102) 里**根本没调用 `StartRetryCompensator`**——线上的 retrying job 全靠管理后台手动点。

### 踩坑过程
这是个非常隐蔽的"代码空跑"坑，**业内通病**。开发同学写完了 [consumer.go#L109-L111](../../backend/internal/ragqueue/consumer.go#L109-L111) 这个公开导出函数：

```go
func StartRetryCompensator(ctx context.Context) {
    startKnowledgeRetryCompensator(ctx)
}
```

写完单测、跑通，**心里默认 main.go 会接进去**——结果 PR 合并的时候，main.go 那边只接了 `RAGConsumer.Start`，没人想起来要再 `go ragqueue.StartRetryCompensator(ctx)`。

代码评审也没拦住——因为审查的同学盯着新增的代码，**对 main.go 没调用这个新函数没有警觉**。Compile 当然通过，单测当然全绿，因为单测里直接调的是 `startKnowledgeRetryCompensator`，根本不经 main。

直到上线半个月后某次故障复盘，运维问"为什么 retrying 状态的 job 一直在那里没动"，我们才回头查代码——`grep StartRetryCompensator` 发现**整个 main 链路里没有调用**。

### 后果
- 失败的 job 写到 DB 标 retrying + next_retry_at 完整无误，但**没人扫**
- 用户重传成功但偶尔要等到管理后台手动点重试
- 监控上 `rag_ingest_jobs_total{status="retrying"}` 持续累积，**像漏水**
- 整个团队对"重试到底有没有用"产生信任危机

### 避坑方案

1. **公开导出的"启动型"函数必须有集成测试**：跑一次 `cmd/rag-server` 的 e2e，看 retrying job 30 秒后会不会自动消失，**集成测试是 main 接线的最后一道闸**。

2. **关键守护协程登记到启动清单**：写一个 `[]Daemon` 列表（health checker、retry compensator、metrics reporter），main 里 `for _, d := range daemons { go d.Start(ctx) }`，新增守护协程必须加到清单——**改一个地方就行，不会漏**。

3. **Prometheus 指标兜底告警**：加一条规则 `rate(rag_ingest_jobs_total{status="retrying"}[5m]) - rate(...completed) > 0 持续 30min` 触发告警——**重试堆积一定异常**。

4. **代码评审 checklist 显式列出**：「新增的导出 daemon 函数是否在 main 里启动？」加进 PR 模板。

### 📚 延伸知识点
- **Dead Code Detection**：staticcheck / go vet 的 unused export 检查，对**未被任何二进制调用的导出函数**应该报警
- **Chaos Engineering**：故意杀掉某个组件看系统是否还自愈，能暴露这种"协程根本没启动"的隐藏 bug
- **Wired Dependency Graph**：用 wire / fx 这类 DI 框架，组件依赖关系显式声明，main 漏接会编译失败

### 面试时怎么说
> "这是我亲身经历的一个挺尴尬的坑——重试代码写得很全，但 main.go 里**忘了调用启动函数**，整个重试链路名存实亡半个月没人发现。
>
> 当时我自己写完 `StartRetryCompensator` 这个导出函数，单测跑得很爽，PR 评审一路通过。但 main.go 那边只接了消费者主循环，没人想起来要再起一个守护协程。
>
> 直到运维同学问'为什么 retrying 状态的 job 一直不动'，我才回头 grep——好家伙，整个项目里根本没人调用这个函数。
>
> 后来我们做了几件事：守护协程统一登记到一个 daemons 列表，main 里循环启动；加了重试堆积的兜底告警；PR 模板里专门写一条「新增 daemon 是否接到 main」。
>
> 这件事让我意识到：**单测绿不代表线上跑——main.go 是所有代码的'最后一公里'，但偏偏是评审最少看的地方**。从此我对启动链路特别敏感。"

---

## 🕳️ 坑 8：MaxLen=10000 + 消费慢 → 消息被静默裁剪、入库永久丢失

### 问题描述
[redis_stream.go#L67-L72](../../backend/internal/ragqueue/redis_stream.go#L67-L72) 设了 `MaxLen=10000 Approx=true`，正常情况下绰绰有余，但**某次消费阻塞两小时**，新消息把老消息挤掉，**那些老消息再也回不来**。

### 踩坑过程
最开始设计 stream 容量时的考虑是「**MaxLen=10000 够 1-2 小时缓冲了**」——按入库 QPS 几十计算，1 万条消息相当于几小时积压。`Approx=true` 是 Redis 推荐写法（按 Macro Node 整批裁，性能更好）。

业内**这类坑特别经典**：MaxLen 设的容量在常态下永远用不完，所以**没人盯着 stream length**，监控也没专门告警。

某次第三方 Embedding API 大面积超时，我们的消费几乎完全停滞——但**生产端不知情**，HTTP 上传仍然 100% 成功入队（因为 Redis 没问题、Publish 永远成功）。两小时后，stream 已经塞了 1.5 万条消息，**最早进来的 5000 条被 `MaxLen` 静默裁掉**。

裁掉的消息对应的 job 在 DB 里还是 pending 状态——**Stream 不会通知 DB「这条被我删了」**。这些 pending job 既不会被消费（消息没了）、也不会被扫描器拉起来（扫描器只看 retrying）。**5000 个 job 永远卡在 pending**。

发现的时候是用户投诉「我半小时前传的文档还没入库」，我们查 stream 里没有、查 DB 是 pending、查日志没有任何处理痕迹——**完美的"静默丢失"**。

### 后果
- ~5000 条 job 静默丢失，**用户上传成功但永久不会处理**
- DB 里 pending 状态堆积，看着像"还在排队"实际是孤儿
- 监控完全没告警，因为 `MaxLen` 触发裁剪是 Redis 内部行为，不抛异常
- 客户投诉后排查至少 2 小时，最后只能写脚本把 pending 老 job 重投

### 避坑方案

1. **MaxLen 不是终极兜底，DB pending 才是**：所有 pending > N 分钟的 job 必须被扫描器拉起来重投——**当前扫描器只看 retrying，需要扩展看 pending**。

2. **stream length 监控告警**：`XLEN stream > MaxLen * 0.8` 或 `consumer_backlog > 5000` 持续 5 分钟立刻 P1，**不能等用户投诉**。

3. **MaxLen 配合 `XADD NOMKSTREAM` + 上游限流**：当 backlog 高的时候直接拒新写入（HTTP 返回 429），让上游慢下来，**别让队列被打爆**——这叫 backpressure。

4. **MaxLen 不要设小**：10000 在 QPS 几十的场景看着够，但**异常发生时几小时就满**。建议 100k 起步，配合监控告警先于裁剪触发。

5. **关键流量考虑 Stream 持久化策略**：Redis AOF 同步 + 每秒 fsync，避免 Redis 重启丢消息。

### 📚 延伸知识点
- **Backpressure（反压）**：响应式编程核心概念，下游消费不动就让上游慢下来，**比单纯扩容更治本**
- **Redis Stream 不是 Kafka**：没有按 offset 回放、没有保留期内不丢、**MaxLen 是覆盖式裁剪**，丢了就是丢了
- **Outbox + CDC**：DB 是真相源，队列是派发通道，DB 里的 outbox 表永远在，靠 CDC 重放，**根治静默丢失**

### 面试时怎么说
> "MaxLen 这事我们栽过一个挺隐蔽的跟头。最开始设了 10000，**算出来够 1-2 小时缓冲，根本用不完**，所以也没专门监控。
>
> 某次 Embedding API 大面积超时，消费几乎停了两小时，stream 塞到 1.5 万。**最早的 5000 条被 MaxLen 静默裁掉**——Redis 不会告诉你它删了什么，DB 里那些 job 还是 pending。等用户投诉「文档还没入库」我们查日志，发现根本没有处理痕迹。
>
> 后来我们改了几件事：扫描器从只看 retrying 扩展到也看老 pending；stream length 加了告警；考虑给上游加 backpressure，backlog 高就直接 429 拒掉新上传。
>
> 这件事让我意识到：**Redis Stream 不是 Kafka**——MaxLen 是覆盖式裁剪，没有"保留期"概念。**异步系统的可靠性，最终一定要回到 DB 上**。把队列当真相源，迟早出大事。"

---

## 总结：八条避坑口诀

| # | 坑名 | 避坑口诀 |
|---|---|---|
| 1 | 先入队后落库 | **DB 是真相源，事务先提交，入队失败就直接 failed** |
| 2 | 大消息塞 payload | **消息只放指针，业务数据走 OSS / DB（Claim Check）** |
| 3 | 重试不分类 | **payload / parse 永不重试，瞬时错误才有重试预算** |
| 4 | 失败不 ACK 等 XCLAIM | **要么 ACK 走 DB 重试，要么 XCLAIM 接管，绝不混用** |
| 5 | 大文件吃光 worker | **HTTP 层卡大小，PDF 加 ctx 超时，大小文件分流** |
| 6 | processing 卡僵尸 | **DB 加超时扫描，handler 加 recover，状态机带兜底** |
| 7 | daemon 没接 main | **守护协程登记 daemons 清单，加堆积告警，PR 模板检查** |
| 8 | MaxLen 静默裁剪 | **stream length 告警先于裁剪，pending 也要扫描兜底** |

---

## 💡 面试时主动升华

如果时间允许，可以在结尾抛这一句：

> "做完这套异步任务调度，我最大的体会是：**异步化不是把活儿挪到后台就行了，它本质是把'同步语义'拆成'状态 + 时间 + 兜底'的三元组**。
>
> 同步代码里的一行 `process(file)` 在异步世界里要展开成：消息存哪、状态机怎么转、失败怎么重、超时谁兜底、监控怎么对账——**任何一个环节偷懒都会被放大成数据问题**。
>
> 所以我现在判断一个异步系统是不是'生产可用'，不看 QPS、不看延迟，就看一件事：**最坏情况下数据会不会丢、丢了能不能找回来。** 能正面回答这一句的，才算真正驾驭了异步。"
