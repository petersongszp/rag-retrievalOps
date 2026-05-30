# RAG Validation Checklist

这份清单用于逐轮验证 RAG 功能开关，目标是帮助你判断每个能力是否真正生效，以及它对召回、排序、拒答、引用一致性的影响。

适用配置文件：
- [backend/config.yaml](/d:/Bear/mianshiba-eino-overseas/backend/config.yaml:83)

建议原则：
- 每一轮只增加少量能力，避免一次全开后难以定位问题。
- 每一轮尽量使用同一批测试问题，方便横向对比。
- 保持 `enable_retrieve_audit=true`，便于观察检索行为。

## 使用方法

1. 修改 [backend/config.yaml](/d:/Bear/mianshiba-eino-overseas/backend/config.yaml:83) 中的 `rag.feature_flags`。
2. 重启后端服务。
3. 用本文提供的测试问题模板逐轮提问。
4. 观察回答内容、引用情况、是否拒答、延迟体感，以及检索审计日志。
5. 把结果记录到本文末尾的验证记录表。

## 逐轮开关对照表

| 轮次 | 目标 | 建议开启 | 建议关闭 | 重点观察 |
| --- | --- | --- | --- | --- |
| 0 | 默认基线 | `enable_retrieve_audit=true` | 其余先关 | 当前系统原始效果、日志是否正常 |
| 1 | Phase2 基线增强 | `enable_hybrid_retrieval=true` `enable_query_rewrite=true` `enable_dynamic_topk=true` `enable_advanced_rerank=true` `enable_retrieve_audit=true` | phase3 全关 | 召回是否更准、排序是否更合理、topk 是否动态变化 |
| 2 | 上下文补全 | 轮 1 基础上加 `enable_parent_child_retrieval=true` | `enable_strategic_topk=false` `enable_evidence_refusal=false` `enable_citation_consistency=false` | 命中内容是否更完整，回答是否少断片 |
| 3 | 更聪明的 TopK | 轮 2 基础上加 `enable_strategic_topk=true` | `enable_evidence_refusal=false` `enable_citation_consistency=false` | 简单问题是否更精简，复杂问题是否保留更多关键证据 |
| 4 | 证据不足拒答 | 轮 2 或 3 基础上加 `enable_evidence_refusal=true` | `enable_citation_consistency=false` | 无答案问题是否开始拒答，幻觉是否下降 |
| 5 | 引用一致性 | 轮 4 基础上加 `enable_citation_consistency=true` | 无 | 回答和引用是否更一致，是否更保守 |
| 6 | 高级改写增强 | 在轮 1 或轮 2 基础上开 `enable_domain_terms=true` `enable_route_specific_rewrite=true` `enable_model_assisted_rewrite=true` | refusal 和 citation 先关，单独看改写效果 | 缩写、术语、混合表达的召回是否明显提升 |

## 每轮推荐配置

以下示例只改 `feature_flags`。`phase2`、`phase3` 的参数先沿用当前默认值。

### 轮 0：默认基线

```yaml
feature_flags:
  enable_prod_guard: false
  enable_ingest_retry: false
  enable_retrieve_audit: true
  enable_hybrid_retrieval: false
  enable_query_rewrite: false
  enable_dynamic_topk: false
  enable_advanced_rerank: false
  enable_parent_child_retrieval: false
  enable_strategic_topk: false
  enable_evidence_refusal: false
  enable_citation_consistency: false
  enable_domain_terms: false
  enable_route_specific_rewrite: false
  enable_model_assisted_rewrite: false
```

### 轮 1：Phase2 基线增强

```yaml
feature_flags:
  enable_prod_guard: false
  enable_ingest_retry: false
  enable_retrieve_audit: true
  enable_hybrid_retrieval: true
  enable_query_rewrite: true
  enable_dynamic_topk: true
  enable_advanced_rerank: true
  enable_parent_child_retrieval: false
  enable_strategic_topk: false
  enable_evidence_refusal: false
  enable_citation_consistency: false
  enable_domain_terms: false
  enable_route_specific_rewrite: false
  enable_model_assisted_rewrite: false
```

### 轮 2：加父子块

```yaml
feature_flags:
  enable_prod_guard: false
  enable_ingest_retry: false
  enable_retrieve_audit: true
  enable_hybrid_retrieval: true
  enable_query_rewrite: true
  enable_dynamic_topk: true
  enable_advanced_rerank: true
  enable_parent_child_retrieval: true
  enable_strategic_topk: false
  enable_evidence_refusal: false
  enable_citation_consistency: false
  enable_domain_terms: false
  enable_route_specific_rewrite: false
  enable_model_assisted_rewrite: false
```

### 轮 3：加策略型 TopK

```yaml
feature_flags:
  enable_prod_guard: false
  enable_ingest_retry: false
  enable_retrieve_audit: true
  enable_hybrid_retrieval: true
  enable_query_rewrite: true
  enable_dynamic_topk: true
  enable_advanced_rerank: true
  enable_parent_child_retrieval: true
  enable_strategic_topk: true
  enable_evidence_refusal: false
  enable_citation_consistency: false
  enable_domain_terms: false
  enable_route_specific_rewrite: false
  enable_model_assisted_rewrite: false
```

### 轮 4：加拒答

```yaml
feature_flags:
  enable_prod_guard: false
  enable_ingest_retry: false
  enable_retrieve_audit: true
  enable_hybrid_retrieval: true
  enable_query_rewrite: true
  enable_dynamic_topk: true
  enable_advanced_rerank: true
  enable_parent_child_retrieval: true
  enable_strategic_topk: true
  enable_evidence_refusal: true
  enable_citation_consistency: false
  enable_domain_terms: false
  enable_route_specific_rewrite: false
  enable_model_assisted_rewrite: false
```

### 轮 5：加引用一致性

```yaml
feature_flags:
  enable_prod_guard: false
  enable_ingest_retry: false
  enable_retrieve_audit: true
  enable_hybrid_retrieval: true
  enable_query_rewrite: true
  enable_dynamic_topk: true
  enable_advanced_rerank: true
  enable_parent_child_retrieval: true
  enable_strategic_topk: true
  enable_evidence_refusal: true
  enable_citation_consistency: true
  enable_domain_terms: false
  enable_route_specific_rewrite: false
  enable_model_assisted_rewrite: false
```

### 轮 6：高级改写单测版

建议单独测，不要和 `enable_evidence_refusal`、`enable_citation_consistency` 混在一起。

```yaml
feature_flags:
  enable_prod_guard: false
  enable_ingest_retry: false
  enable_retrieve_audit: true
  enable_hybrid_retrieval: true
  enable_query_rewrite: true
  enable_dynamic_topk: true
  enable_advanced_rerank: true
  enable_parent_child_retrieval: true
  enable_strategic_topk: false
  enable_evidence_refusal: false
  enable_citation_consistency: false
  enable_domain_terms: true
  enable_route_specific_rewrite: true
  enable_model_assisted_rewrite: true
```

## 测试问题模板

建议每类准备 3 到 5 题，并在每一轮复用同一批问题。

### A 类：明确命中题

目的：
- 看基础召回和排序

模板：
- 请根据知识库介绍一下 `X` 的核心概念
- `X` 和 `Y` 有什么区别
- 总结一下文档里关于 `X` 的最佳实践
- 知识库里提到的 `X` 主要步骤是什么

示例：
- 什么是 JVM 调优？
- Kafka 和 RabbitMQ 有什么区别？
- Go 并发编程有哪些常见模式？

预期：
- 轮 1 后相关性应明显提升
- 轮 2 后答案上下文更完整

### B 类：缩写/术语题

目的：
- 看 query rewrite、domain terms、model-assisted rewrite

模板：
- `缩写` 的作用是什么
- `术语别名` 在项目里怎么用
- `中英混合词` 有哪些注意点

示例：
- JVM GC 常见问题有哪些？
- RPC 超时一般怎么排查？
- ES 写入变慢可能是什么原因？
- golang 的 GC STW 是什么？
- MQ 积压怎么处理？

预期：
- 轮 1 后缩写效果应比轮 0 略好
- 轮 6 后术语和缩写召回提升最明显

### C 类：长文上下文题

目的：
- 看 parent-child retrieval

模板：
- 结合上下文解释 `X` 为什么这么设计
- `X` 这一节前后文主要讲了什么
- 给我完整梳理 `X` 的实现思路

示例：
- 结合上下文解释 Kafka 深入文章里 consumer group 的设计
- 完整梳理 JVM 调优那篇文档的排查思路
- 介绍 Go 高级并发文档里提到的几个模式和适用场景

预期：
- 轮 2 后命中内容更完整
- 回答不再只截取一个孤立段落

### D 类：复杂多条件题

目的：
- 看 dynamic topk 和 strategic topk

模板：
- 如果 `条件1 + 条件2 + 条件3`，应该怎么处理
- 在 `场景A` 下，如何权衡 `指标B` 和 `指标C`
- 请从原理、风险、实践三个角度说明 `X`

示例：
- 如果 Kafka 出现消息堆积、消费延迟升高、实例 CPU 也很高，应该怎么排查？
- 在高并发场景下，Go 锁竞争严重该怎么分析和优化？
- JVM Full GC 频繁、响应时间抖动、堆内存持续增长时应该怎么定位？

预期：
- 轮 3 后复杂题更稳定
- 简单题更精简，复杂题保留更多关键证据

### E 类：无答案题

目的：
- 看 evidence refusal

模板：
- 知识库里有没有提到 `一个明显不存在的东西`
- 请根据知识库说明 `无关领域主题`
- 项目文档里有没有 `虚构名词` 的定义

示例：
- 知识库里有没有介绍 Rust 异步运行时 Tokio 的设计？
- 请总结一下 TensorFlow 量化训练最佳实践
- 项目文档里有没有定义 BearLang 这门语言？

预期：
- 轮 4 后应更倾向拒答或明确说证据不足
- 如果仍然一本正经胡说，说明 refusal 没起作用或阈值偏松

### F 类：引用一致性题

目的：
- 看 citation consistency

模板：
- 请回答并给出依据
- 必须引用知识库里的原文依据来说明
- 总结时请保持和引用内容一致，不要扩展猜测

示例：
- 请根据知识库说明 JVM 调优的重点，并给出依据
- 请引用知识库内容解释 Kafka consumer rebalance 的影响
- 根据文档总结 Go 并发最佳实践，并给出依据

预期：
- 轮 5 后回答会更克制
- 引用和结论更贴合
- 可能会略微增加延迟，或让答案变短

## 每轮该看什么日志

保持 `enable_retrieve_audit=true`，重点观察检索审计日志。

建议搜索关键词：
- `[KB Retrieve]`

每轮通用关注字段：
- 原始 query
- rewrite 后 query
- candidate topk
- final topk
- 是否启用 rerank
- 是否走 hybrid
- 返回 chunk 数量
- 最终是否拒答
- 引用检查是否通过
- latency

分轮重点：

### 轮 1

- rewrite 前后是否变化
- topk 是否动态变化
- rerank 是否生效

### 轮 2

- 是否出现 parent/child 补全痕迹
- chunk 上下文是否变长

### 轮 3

- strategic topk 是否调整最终 k
- 简单题和复杂题的 k 是否不同

### 轮 4

- refusal gate 是否触发
- 哪个阈值导致拒答

### 轮 5

- citation check 是否执行
- consistency pass/fail

### 轮 6

- domain terms 是否命中
- model rewrite 是否 applied
- dense 和 sparse route 是否采用不同改写

## 跑法建议

推荐顺序：
1. 跑轮 0，保留基线结果
2. 跑轮 1，看主链路增强
3. 跑轮 2，看上下文补全
4. 跑轮 3，看复杂问题收益
5. 跑轮 4，看拒答
6. 跑轮 5，看引用一致性
7. 单独跑轮 6，看高级改写收益

如果轮 6 改写效果不稳定，优先排查：
- `enable_model_assisted_rewrite`
- `enable_route_specific_rewrite`
- `enable_domain_terms`

建议按上面顺序逐个回退，而不是一次全关。

## 验证记录表

你可以把每轮结果直接填在这里。

| 题目 | 轮次 | 是否答对 | 是否更完整 | 是否有胡编 | 是否拒答合理 | 延迟体感 | 备注 |
| --- | --- | --- | --- | --- | --- | --- | --- |
|  |  |  |  |  |  |  |  |
|  |  |  |  |  |  |  |  |
|  |  |  |  |  |  |  |  |
|  |  |  |  |  |  |  |  |
|  |  |  |  |  |  |  |  |

## 快速结论模板

每轮跑完后，可以用下面格式快速记录结论：

```text
轮次：
开启能力：
最明显改善：
最明显问题：
是否建议保留：
下一轮是否继续：
```
