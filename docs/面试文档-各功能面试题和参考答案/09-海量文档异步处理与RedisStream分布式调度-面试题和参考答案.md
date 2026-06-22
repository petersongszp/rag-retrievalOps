# 海量文档异步处理与 Redis Stream 分布式任务调度 - 面试题和参考答案

> 项目背景：基于 Redis Stream 搭建分布式任务调度框架，把文档解析、向量化任务异步化消费，避免大批量上传同步阻塞 HTTP 接口，提升知识库入库吞吐。
>
> 涉及代码：[ragqueue/redis_stream.go](../../backend/internal/ragqueue/redis_stream.go)、[ragqueue/queue.go](../../backend/internal/ragqueue/queue.go)、[ragqueue/consumer.go](../../backend/internal/ragqueue/consumer.go)、[ragplatform/mq/publisher.go](../../backend/internal/ragplatform/mq/publisher.go)、[ragplatform/mq/consumer.go](../../backend/internal/ragplatform/mq/consumer.go)、[api/handler/kb/handler.go](../../backend/api/handler/kb/handler.go)、[model/kb_ingest_job.go](../../backend/internal/model/kb_ingest_job.go)。

---

## 面试官提问路径

```
"你们海量文档入库怎么做的？同步还是异步？"
   ↓
"为什么选 Redis Stream，不用 Kafka / RabbitMQ？"
   ↓
"消息体长什么样？job 状态机怎么设计的？"
   ↓
"消费者怎么并发？XReadGroup 的 Block / Count 为什么是这个值？"
   ↓
"任务失败了怎么重试？退避策略是什么？什么情况下不重试？"
   ↓
"线上突然 backlog 涨到几万你怎么排？"
   ↓
"如果让你重做一遍，哪些地方你会动手术？"
```

---

## Q1：你们这个海量文档异步处理模块整体是怎么设计的？

**我的回答：**

我们这块是一条很标准的「**生产者—Redis Stream—消费者池**」流水线。核心结论是：**HTTP 上传只做"落库 + 入队"两件事，1 秒内必返回；解析、切片、Embedding、写 Milvus 全部在消费者侧异步完成。**

入口在 [handler.go#L536](../../backend/api/handler/kb/handler.go#L536)，文件落 `local_oss` 之后开一个事务写两张表：`kb_document` 和 `kb_ingest_job`，事务提交后才调 `ragqueue.PublishKnowledgeIngest`。这样**即使 Redis 挂了**，job 已经在 DB 里，扫描器后面还能补救。

队列层用 Redis Stream，stream key 是 `interview:stream:knowledge_ingest`，`MaxLen=10000` 近似裁剪，consumer group 叫 `rag-consumer-group`，每个 server 实例的 consumer name 是 `rag-consumer-{host}`。消费在 [redis_stream.go#L126](../../backend/internal/ragqueue/redis_stream.go#L126)，`XReadGroup Count=10 Block=1s`。

消费者拿到消息后并不直接处理，而是丢进一个 **`ants` 协程池（容量 1000）** 异步执行（[redis_stream.go#L40](../../backend/internal/ragqueue/redis_stream.go#L40)、[L182](../../backend/internal/ragqueue/redis_stream.go#L182)），处理完才 `XAck`。失败的 job 会按指数退避更新 `next_retry_at`，由独立的扫描器到点拉起来重投。

> 相关代码：[handler.go#L536-L545](../../backend/api/handler/kb/handler.go#L536-L545)、[redis_stream.go#L29-L97](../../backend/internal/ragqueue/redis_stream.go#L29-L97)、[consumer.go#L45-L107](../../backend/internal/ragqueue/consumer.go#L45-L107)

### 🔍 深挖追问兜底

**Q：为什么是"先落库再入队"而不是"先入队再落库"？**
> 入队成功 ≠ 业务成功。Redis Stream 没有事务和我业务库的事务挂钩，先入队，后续 DB 失败的话消息已经飞了；先落 DB，至少 `kb_ingest_job` 有 pending 记录，最坏情况下扫描器或人工脚本能把这条 job 重投。**这是典型的"以 DB 为真相源"模式。**

**Q：HTTP 入队失败你怎么处理？**
> 见 [handler.go#L546-L562](../../backend/api/handler/kb/handler.go#L546-L562)，发布失败立刻把 job 改成 `failed` 状态、`last_error_code=enqueue_error`、`document` 也改 `failed`，给前端返 500，让用户重试上传。**不会留着 pending 状态等扫描器**——因为扫描器只扫 `retrying`。

**Q：consumer name 用 host 唯一吗？多副本怎么办？**
> 每个 pod 的 host 不一样，所以天然是多消费者。Redis Stream 的 consumer group 保证同一条消息只会派给组内一个 consumer，水平扩容只要再起一个 pod 就行，无需协调。

---

## Q2：消息体和 `kb_ingest_job` 状态机是怎么设计的？

**我的回答：**

消息体非常瘦——故意的。`KnowledgeIngestPayload` 只有 7 个字段：`user_id / kb_id / document_id / job_id / file_path / file_type / collection`（[queue.go#L23-L32](../../backend/internal/ragqueue/queue.go#L23-L32)）。**关键设计是消息里不带文件内容、不带切片配置**，所有重资料都在 DB 里，消息只承担"指针"作用。

为什么这么干？因为 Redis Stream 单条消息越大，XADD/XREAD 的 RTT 越高、内存占用越快。我们一份文档可能几十 MB，塞进消息毫无意义——文件已经在 OSS，job 已经在 DB，消费者拿着 ID 自己去捞就行。

状态机是这块的灵魂，定义在 [kb_ingest_job.go#L38-L64](../../backend/internal/model/kb_ingest_job.go#L38-L64)，**七个状态**：`pending / processing / completed / failed / retrying / dead / canceled`。所有变更必须走 `updateStatusWithGuard`，**底层用 SQL 的 `WHERE status IN (?)` 做乐观锁**，配合 `kbIngestJobTransitions` 这张转移表，非法跳转直接 `RowsAffected=0` 返回 `ErrInvalidKBIngestJobTransition`。

最关键的是 [ClaimForProcessing](../../backend/internal/model/kb_ingest_job.go#L274)：消费者拿到消息第一件事是 `pending/retrying → processing`，这个是**幂等的"占有"动作**——如果重复投递、`claim` 第二次会返回 `false`，消费者直接 `return nil` 然后 ACK，不会重复处理。

> 相关代码：[queue.go#L23-L32](../../backend/internal/ragqueue/queue.go#L23-L32)、[kb_ingest_job.go#L38-L64](../../backend/internal/model/kb_ingest_job.go#L38-L64)、[kb_ingest_job.go#L389-L421](../../backend/internal/model/kb_ingest_job.go#L389-L421)

### 🔍 深挖追问兜底

**Q：状态机只在应用层校验，DB 层没约束怎么办？**
> 应用层用「转移表 + WHERE IN」做乐观锁已经够用，Postgres/MySQL 不支持开箱即用的状态机约束。**真正的兜底是 `RowsAffected==0` 返回 `ErrInvalidKBIngestJobTransition`，调用方根据错误决定要不要回滚或忽略。**

**Q：为什么单独有 `dead` 状态？和 `failed` 区别？**
> `failed` 是"这次跑挂了，可能可以重试"；`dead` 是"重试到上限或不可重试错误"，**人工不介入的话永远不会再跑**。`dead → retrying` 是唯一逃出来的路径（人工点重试），见 [kb_ingest_job.go#L61-L63](../../backend/internal/model/kb_ingest_job.go#L61-L63)。

**Q：consumer 已经 `ClaimForProcessing` 占了，但进程崩了，状态卡在 `processing` 怎么办？**
> 这是目前的一个**已知缺陷**：Redis Stream 的 `pending list` 会保留这条消息（因为没 ACK），但 DB 里 job 卡死在 processing。**目前没做 `XPENDING+XCLAIM` 的自动接管**，需要靠运维脚本扫超时 processing 手工救。未来计划是加一个 stale-claim 扫描器。

---

## Q3：Redis Stream、Kafka、RabbitMQ 你们怎么选的？为什么不用 Kafka？

**我的回答：**

直接说结论：**当时（项目早期）我们已经把 Redis 部署起来了用作缓存和限流，再多上一个 Kafka 只为了入库这一个场景，运维成本和收益不匹配。** 所以选了 Redis Stream。

技术对比上：

第一，**消息量级**。我们入库 QPS 峰值也就几十、日均几千条，Redis Stream 的吞吐（单 key 几万 QPS）绰绰有余。Kafka 的优势在百万级 QPS、长期持久化、多消费组回放，这些场景我们都没有。

第二，**特性匹配**。Redis Stream 自带 consumer group（XGROUP）、待处理列表（XPENDING）、消息确认（XACK），**这就是个轻量版 Kafka**。我们要的"组内单消费、可水平扩容、可 ACK 重试"，Stream 全有。

第三，**运维成本**。Kafka 要 ZooKeeper（或 KRaft）、Broker、Topic 分区规划，三个人小团队真扛不动；RabbitMQ 的 Erlang 运维栈我们也没经验。Redis 已经在跑了，加个 Stream 是 0 成本。

**反向也要承认：** 如果后面我们要做"重放最近 7 天的入库消息做回归测试"，Redis Stream 的 `MaxLen=10000` 会被截断、Kafka 的长期保留就赢。**这是用规模换简洁性的权衡，不是 Kafka 不好。**

> 相关代码：[redis_stream.go#L67-L75](../../backend/internal/ragqueue/redis_stream.go#L67-L75)、[redis_stream.go#L99-L108](../../backend/internal/ragqueue/redis_stream.go#L99-L108)

### 🔍 深挖追问兜底

**Q：Redis 单点挂了你们入库就停了，怎么扛？**
> 一是 Redis Sentinel/Cluster 做高可用；二是 [handler.go#L546](../../backend/api/handler/kb/handler.go#L546) 的入队失败兜底——**job 在 DB 里是 pending，HTTP 直接报错让用户重试**。最坏情况"丢可用性不丢数据"。

**Q：Stream MaxLen=10000 太小了吧？**
> `Approx=true` 是近似裁剪，Redis 内部按 MACRO node 整批删，实际容量略大。10000 对当前 QPS 够 1~2 小时缓冲，**真正的兜底不是 Stream 容量、是 DB 里的 job 表**。Stream 即使被裁了，扫描器还是能从 `kb_ingest_job` 把 retrying 的拉起来。

**Q：为什么不直接用 BLPOP/RPUSH 这种 list？**
> List 没有 consumer group，多副本会"抢消息抢不公平"且没有 ACK 概念，消费侧崩了消息就没了。**Stream 是 Redis 5.0 后专门为这个场景做的**，没必要倒退。

---

## Q4：消费侧的并发模型怎么定的？为什么 ants 池容量是 1000、Count=10、Block=1s？

**我的回答：**

我们消费侧是「**单 reader goroutine + 大协程池**」的模型，不是每条消息开一个 goroutine。

reader 在 [redis_stream.go#L110](../../backend/internal/ragqueue/redis_stream.go#L110) 跑一个无限 for，每次 `XReadGroup Count=10 Block=1s` 拉最多 10 条，拉到后 for 循环里把每条提交给 [ants 池](../../backend/internal/ragqueue/redis_stream.go#L40)（`ants.NewPool(1000)`）。

**几个关键参数为什么是这个值：**

`Count=10`：单次拉太多会让一条消息的 ack 拖累后面 9 条；太少（=1）则 RTT 浪费。10 是经验值，单批处理 + 单批等待的平衡。

`Block=1s`：拉空时阻塞 1 秒，**既不饿死 CPU 也不让 ctx.Done() 等太久**——shutdown 时最多 1 秒内能退出循环。

`Pool=1000`：单个解析+Embedding+Milvus 写入大概几百毫秒到几秒，按 1000 并发上限算，单实例峰值能扛 ~1000 个并发任务。Embedding 的 RPS 上限其实在第三方 API 那边（litellm），所以 1000 已经远大于瓶颈。**真正的瓶颈是下游而不是池子本身**。

还有一个反直觉的点：**handler 处理失败时不 ACK，让 Redis 在 pending list 里保留这条**（[redis_stream.go#L182-L192](../../backend/internal/ragqueue/redis_stream.go#L182-L192)）。但我们目前**没有跑 XPENDING 自动接管**，所以这条最终是靠 DB 里的 retrying 机制重投——Stream 里的 pending 实际是孤儿。后续要补 stale-claim。

> 相关代码：[redis_stream.go#L40-L52](../../backend/internal/ragqueue/redis_stream.go#L40-L52)、[redis_stream.go#L110-L197](../../backend/internal/ragqueue/redis_stream.go#L110-L197)

### 🔍 深挖追问兜底

**Q：池子满了 `pool.Submit` 会怎样？**
> 默认 ants 是阻塞模式，`Submit` 会等到有空 worker。考虑到我们 reader 是单线程，submit 阻塞 = 反压到 XReadGroup，**这其实是个天然的限流**——Redis 端 backlog 增长，但下游不会被打挂。

**Q：handler 函数在协程池里 panic 了怎么办？**
> 目前 handler 内部用了几层 `_ = err` 和 log，但**没有专门的 recover**。ants 池本身会 recover panic 不让进程崩，但消息也就直接丢了（因为协程返回前没 ACK，但也没回写 DB）。**这是个待补的洞**，需要在 [handler.go](../../backend/internal/ragqueue/consumer.go#L45) 入口加 defer recover 转成 retryable 错误。

**Q：单 reader 是不是性能瓶颈？**
> Reader 只做 IO + 派发，CPU 占用极低；瓶颈一定在下游处理。如果真的扛不住，水平扩容多个 pod 就行——consumer group 自动均衡。**单 reader 反而避免了 Stream 客户端连接数爆炸的问题**。

---

## Q5：为什么 Embedding 错误重试、Parse 错误不重试？退避是怎么算的？

**我的回答：**

这道题的核心是**错误分类决定可重试性**，不是简单"全部重 3 次"。

错误分类在 [consumer.go#L454-L478](../../backend/internal/ragqueue/consumer.go#L454-L478)，分 5 类：`payload / parse / embedding / milvus / unknown`。判断逻辑在 [isKnowledgeIngestRetryable](../../backend/internal/ragqueue/consumer.go#L480-L491)：

- `payload` 错误（消息字段缺失）→ **永远不重**，消息本身就是脏的，重 100 遍都一样
- `parse` 错误（PDF 解析失败、文件类型不支持）→ **永远不重**，文件内容本身的问题
- `embedding / milvus` 错误 → **只有"瞬时失败信号"才重**，看 [hasTransientFailureSignal](../../backend/internal/ragqueue/consumer.go#L493-L518) 匹配 `timeout / connection refused / network / EOF` 这类关键词
- `unknown` 错误 → 同上，看是不是瞬时

**这个设计的关键是「永久错误不浪费重试预算」**：一份坏 PDF 重 3 次还是坏，只会污染监控、消耗 worker。

退避算法在 [calculateKnowledgeRetryBackoff](../../backend/internal/ragqueue/consumer.go#L547-L565)：基础值从 `config.RAG.Thresholds.RetryBackoffMS`（默认 500ms）来，**指数退避 base * 2^(retry-1)**，最大不超过 5 分钟，再叠 0.8~1.2 的随机抖动避免雷击。最大重试次数也从配置读，默认 3 次，超过就 `dead`。

重试不靠 Stream 的 redelivery，**而是 DB 里 `next_retry_at`（[kb_ingest_job.go#L350-L387](../../backend/internal/model/kb_ingest_job.go#L350-L387)）+ 独立的扫描器**（[consumer.go#L324-L411](../../backend/internal/ragqueue/consumer.go#L324-L411)）：每隔 5~30 秒扫一次到点的 retrying job，重新调 `PublishKnowledgeIngest` 投回 Stream。

> 相关代码：[consumer.go#L226-L322](../../backend/internal/ragqueue/consumer.go#L226-L322)、[consumer.go#L480-L518](../../backend/internal/ragqueue/consumer.go#L480-L518)

### 🔍 深挖追问兜底

**Q：抖动 0.8~1.2 范围怎么定的？太窄了吧？**
> 范围窄是故意的，**核心目的是错峰不是发散**。如果是 0.5~2.0，重试时间分布太散了，监控曲线变难看。0.8~1.2 大概 ±20%，足够把雷击错开但又不会拖很久。

**Q：扫描器是单点的，多副本怎么办？**
> 这是个**目前还没解决的问题**。`StartRetryCompensator` 在 [consumer.go#L109](../../backend/internal/ragqueue/consumer.go#L109) 定义了但 **main.go 里没调用**——多副本启动会出现"多个扫描器同时改 retrying"的竞争。短期靠 `MarkPendingForRetry` 里的 `RowsAffected>0` 单行乐观锁兜底，长期需要加分布式锁（Redis SETNX）。

**Q：超过 max_retry 就 dead，不能让用户手动再点重试吗？**
> 能。状态机里 `dead → retrying` 是合法转移（[kb_ingest_job.go#L61-L63](../../backend/internal/model/kb_ingest_job.go#L61-L63)），管理后台调 `MarkRetrying` 接口（[kb_ingest_job.go#L226](../../backend/internal/model/kb_ingest_job.go#L226)）会清空 `retry_count`、置 `next_retry_at=null`，重新投递。

---

## Q6：线上突然 backlog 涨到几万你怎么排？

**我的回答：**

第一反应**不是去看代码，是去看曲线**。

我们埋了三个核心 metric（[rag_metrics.go#L196-L216](../../backend/internal/observability/metrics/rag_metrics.go#L196-L216)）：`rag_consumer_backlog`、`rag_consumer_pending`、`rag_consumer_lag`，都打了 `stream/group/message_type` 标签。**先看 lag 是不是飙升、pending 是不是堆积**，这是定性的第一步。

然后看 [rag_ingest_duration_seconds](../../backend/internal/observability/metrics/rag_metrics.go#L181)（histogram）和 [rag_ingest_jobs_total](../../backend/internal/observability/metrics/rag_metrics.go#L174)（按 status/error_code 标签），定位问题：

- 如果 `error_code=embedding_error` 占比突然飙到 30%+ → litellm/Embedding 服务挂了
- 如果 `status=retrying` 大量堆积、duration 正常 → 下游 Milvus 慢，重试雪崩
- 如果整个 backlog 涨但 duration p95 没变 → 上游写入流量飙了，需要扩 pod
- 如果 duration p99 飙到 30 秒以上 → 大概率是大文件 PDF 解析卡死

**线上确实出过一次：** 用户批量上传了 20 多份 100MB+ PDF，单文件解析就要 20 多秒，1000 个 worker 全卡在 PDF Reader 上，新消息进不来。**当时的应急方案是调用 [PauseKnowledgeIngest](../../backend/internal/ragqueue/queue.go#L133) 临时暂停消费**——这是个反向开关，handler 入口检查到 paused 就直接 return nil（[consumer.go#L46-L49](../../backend/internal/ragqueue/consumer.go#L46-L49)），让卡住的 worker 跑完之后队列自然恢复。

后来加了文件大小预检（前置在上传 handler）和 PDF 解析超时（Context Deadline），杜绝单条消息打爆整个池。

> 相关代码：[rag_metrics.go#L174-L216](../../backend/internal/observability/metrics/rag_metrics.go#L174-L216)、[redis_stream.go#L199-L244](../../backend/internal/ragqueue/redis_stream.go#L199-L244)、[queue.go#L133-L155](../../backend/internal/ragqueue/queue.go#L133-L155)

### 🔍 深挖追问兜底

**Q：`StartMetricsReporter` 是 10 秒一次推 backlog，会不会太频？**
> 10 秒是和 Prometheus scrape interval（默认 15s）配套的，确保每个抓取周期都能拿到新值。频率再高没意义——`XINFO GROUPS` 本身有几十毫秒延迟。

**Q：Pause 是全局的，多副本怎么同步？**
> 目前 `ingestPaused` 是**进程内全局变量**（[queue.go#L113-L115](../../backend/internal/ragqueue/queue.go#L113-L115)），多副本不会互相通知。**这是已知缺陷**——线上要逐 pod 调 API 才能全停。未来想用 Redis Pub/Sub 广播 pause 事件让所有 pod 同步开关。

**Q：metrics 里 lag 是负数（-1）什么意思？**
> 见 [redis_stream.go#L221](../../backend/internal/ragqueue/redis_stream.go#L221)，`-1` 表示 `XINFO GROUPS` 调用失败或 group 不存在。Grafana 上看到 -1 就知道是 metric 上报本身挂了，而不是真的没积压。

---

## Q7：如果让你重做一遍，哪些地方你会优先动手术？

**我的回答：**

按重要性排，三件事我现在就想动：

第一，**`StartRetryCompensator` 真的接进 main.go**。代码定义了但在 [rag-server/main.go](../../backend/cmd/rag-server/main.go) 里没启动，目前 retrying 的 job 完全靠手动触发或运维脚本，**这是最大的运营黑洞**。同步要把扫描器的多实例竞争用 Redis SETNX 锁掉，避免多副本都在扫。

第二，**XPENDING + XCLAIM 的 stale-claim 接管**。现在 worker 崩了消息会卡在 pending list 里，靠 DB 状态机救其实绕了远路；标准做法是另起一个 reaper 协程，扫 idle 超过 5 分钟的 pending，XCLAIM 给自己重新处理，**这是 Redis Stream 官方推荐的范式**。

第三，**消息追踪 ID（trace_id）打通全链路**。现在消息体里没 trace_id，handler 内部生成 audit_trace_id 但和上游 HTTP 请求的 trace 对不上。加上之后能从用户上传的 HTTP 请求一路追到 Milvus 写入，排障效率翻倍。

次要的还有：handler 里加 defer recover、Pause 用 Pub/Sub 广播、stream MaxLen 提到 10w（成本可控）、单文件大小限制前置到 HTTP 层不要等到消费侧。

**整个模块的设计思路我是认可的**——以 DB 为真相源、以 Stream 为派发通道、以协程池做并发——但**生产化的细节（自动接管、分布式协调、recover）还差一截**，这是从 MVP 走到企业级必经的台阶。

> 相关代码：[main.go#L83-L102](../../backend/cmd/rag-server/main.go#L83-L102)、[consumer.go#L109-L111](../../backend/internal/ragqueue/consumer.go#L109-L111)、[redis_stream.go#L199-L244](../../backend/internal/ragqueue/redis_stream.go#L199-L244)

### 🔍 深挖追问兜底

**Q：你说 MVP 思路认可，那为什么不直接上 Kafka？**
> Kafka 解决百万级吞吐和长期保留，**我们当前规模没那个需求**。先把 Redis Stream 用扎实（自动接管 + 分布式锁 + trace），等真有 10x 增长压力再迁移，**别为了"高级"而高级**。

**Q：`StartRetryCompensator` 没接是 bug 吗？**
> 算半个 bug——失败 job 在 [handleKnowledgeIngestFailure](../../backend/internal/ragqueue/consumer.go#L226) 里会被正确写成 retrying + next_retry_at，但**没人来扫**。线上目前靠后台同学手动点"重试"按钮（走 [MarkRetrying](../../backend/internal/model/kb_ingest_job.go#L226) 路径）兜，能用，但不优雅。

**Q：Pub/Sub 广播 pause 不会丢消息吗？**
> 会，Pub/Sub 是 fire-and-forget。但 pause 这个场景**容忍少数 pod 漏一次广播**——下次发布时再广播一次就行，不需要强可靠性。强一致的话就走 Redis Hash + 客户端定时拉。

---

## 主动引导的钩子

把节奏抢回到自己熟悉的话题：

- 💡 "说到 Redis Stream 的 XPENDING，我们项目里**还没补这块**，但我看过一个工业实践——可以聊聊为什么 Stream 比 List 适合做异步任务" → 引导到 Redis 数据结构选型

- 💡 "聊到状态机，我们 `kb_ingest_job` 的 7 状态转移是用 SQL 乐观锁实现的，**这个套路在我们的多租户 KB 模块也用了**" → 引导到 [08-多租户架构](./08-多租户架构与三级权限模型-面试题和参考答案.md)

- 💡 "提到 backlog 监控，**我们整套 Prometheus 指标体系**埋了 retrieve / cache / ingest 三大类，可以展开聊全链路可观测" → 引导到 [10-全链路可观测体系](./10-全链路可观测体系-面试题和参考答案.md)

- 💡 "讲到入队失败兜底，这个'**以 DB 为真相源**'的设计原则我们在 API Key 状态、缓存写入也都用了，本质都是承认外部系统不可靠" → 引导到分布式系统一致性话题

- 💡 "重试退避加抖动这个细节，**我们语义缓存的失效雪崩防护也是同样思路**——0.8~1.2 倍随机化" → 引导到 [05-语义缓存降本](./05-语义缓存降本-面试题和参考答案.md)
