# 中间件专项：Kafka 深入与实践

本专题深入探讨 Kafka 架构、存储、复制、高可用与性能优化实践。

## 目录
- 架构与组件
- 存储机制
- 副本与高可用
- 生产与消费模式
- 性能与调优

## 架构与组件
理解 Broker、Topic、Partition、Replica、ISR 等核心概念及其关系。

### 关键概念
- Broker：存储与服务节点
- Topic/Partition：主题与分区（顺序与并行度的基本单位）
- Replica/ISR：副本与同步副本集合，保证高可用
- Controller：分区领导者选举与元数据管理
- ZooKeeper/KRaft：元数据存储（新版本 KRaft 自管理）

## 存储机制
- 日志段：按大小/时间切分，段+索引文件，顺序写友好
- 索引：偏移索引与时间索引，常数时间定位
- 清理策略：删除（基于保留策略）与压缩（Log Compaction）
- 零拷贝：`sendfile` 提升吞吐与降低 CPU

## 副本与高可用
- Leader-Follower：客户端只与 Leader 交互
- ISR：仅同步副本集合内计入 quorum
- 不一致副本恢复：追赶或剔除；`unclean.leader.election` 决定容错与一致性取舍
- 跨机房/跨地域复制：MirrorMaker2 或集群链路

## 生产与消费模式
### 生产者
- acks：`0/1/all`；可靠性与延迟权衡
- 幂等：`enable.idempotence=true`；避免重复提交
- 事务：跨分区/会话的 EOS（Exactly Once Semantics）
- 批量与压缩：`batch.size`、`linger.ms`、`compression.type`

### 消费者
- 消费组：组内分区独占，重平衡策略（`range`/`roundrobin`/`sticky`）
- 位移管理：`__consumer_offsets`；自动/手动提交
- 再均衡与暂停/恢复：避免抖动与反压
- 反序列化与延迟处理（DLQ/重试主题）

## 性能与调优
- Broker：`num.network.threads`、`num.io.threads`、页缓存、磁盘类型（SSD优先）
- Topic：`replication.factor`、`min.insync.replicas`、`segment.bytes`、`retention.ms`
- Producer：批次/压缩/并发 in-flight 请求；超时与重试
- Consumer：`fetch.min.bytes`、`max.partition.fetch.bytes`、`max.poll.interval.ms`
- 操作：分区规划（避免过多小分区）、容量预估、冷热隔离

## 观测与排障
- 指标：生产/消费吞吐、请求延迟、拒绝率、ISR 变动、堆栈/GC
- 工具：Kafka Exporter、Cruise Control、Perf 工具（`kafka-producer-perf-test.sh`）
- 常见问题：小消息过多、分区倾斜、Linger 太小、GC 抖动、网络抖动


