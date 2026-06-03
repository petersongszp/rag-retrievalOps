# Phase 2 L0/L1 验证教程

## 1. 背景

你现在已经完成了两块能力：

1. `L0 策略开关、配置冻结与基线快照`
2. `L1 混合检索召回链路（Dense + Sparse）`

这时候最重要的事情，不是继续往后写功能，而是先确认两件事：

1. 这些代码是否真的能跑起来。
2. 打开或关闭开关时，系统行为是不是符合预期。

这篇教程就是专门为这个目标写的。你可以把它当成一份“验收操作手册”。

---

## 2. 这篇教程会做什么

我们把验证分成 4 层，从便宜到昂贵、从局部到整体：

1. `配置层验证`：确认 L0 的开关、快照、启动日志、fail-fast 校验正常。
2. `单元测试验证`：确认配置校验、Sparse 检索、检索结果契约这些核心逻辑正常。
3. `接口联调验证`：启动服务，直接调用 `/api/kb/retrieve`，观察 dense 和 hybrid 的真实行为。
4. `离线回归验证`：用 `retrieval-eval` 跑 baseline 和 candidate，对比 Recall、MRR、nDCG、延迟。

如果你时间很紧，至少先做前 3 层。

---

## 3. 先理解几个关键点

### 3.1 L0 在验证什么

L0 不是“检索效果变好了”，而是“检索优化具备安全上线基础”。

它重点验证的是：

1. 开关是否存在，而且能独立控制策略。
2. 配置非法时，系统能不能启动即报错，而不是运行到一半才炸。
3. 启动时是否能留下足够的策略快照，方便排查“当时到底开了什么”。

### 3.2 L1 在验证什么

L1 不是“所有检索都变强了”，而是“系统已经具备两条召回路由”。

它重点验证的是：

1. Dense 路由还能正常工作。
2. 打开 hybrid 开关后，系统能走 `dense + sparse`。
3. 返回结果和审计日志里，能看见路由信息。

### 3.3 一个很重要的现实约束

离线评测文件 `backend/scripts/evaluation/dataset.json` 里现在很多 `relevant_ids` 还是占位值，比如：

`replace-with-real-chunk-id-go-goroutine-channel`

这意味着：

1. 命令可以跑。
2. 但结果现在不具备真实参考意义。

所以做 L7 风格的离线评测前，你要先把这些占位值替换成你知识库里真实的 `chunk_id`。

---

## 4. 整体验证流程

建议按这个顺序做：

1. 跑单元测试，确认基础逻辑没坏。
2. 启动服务，确认 L0 日志和 baseline 快照生成。
3. 关闭 hybrid 跑一次接口，确认还是 dense。
4. 打开 hybrid 再跑一次接口，确认进入 `dense+sparse`。
5. 最后再跑离线回归，做基线对比。

这个顺序的好处很简单：先排除“代码本身就坏了”，再排除“配置问题”，最后才看“效果问题”。

---

## 5. 第一步：跑单元测试

### 5.1 目标

先确认 L0/L1 相关核心逻辑没有明显回归。

### 5.2 命令

在 `backend` 目录下执行：

```powershell
go test ./internal/config ./internal/milvus/retrieval ./api/handler/kb
```

### 5.3 我这边已经替你跑过一次

上面这条命令当前在仓库里是通过的。

通过结果对应三类验证：

1. `internal/config`
   - 验证 RAG 配置校验
   - 验证 hybrid 权重非法时会报错
   - 验证 dynamic topk 范围非法时会报错
   - 验证加载配置时会创建 baseline 快照
2. `internal/milvus/retrieval`
   - 验证 sparse 倒排索引和 BM25 排序
   - 验证 fusion / dedupe
   - 验证检索结果指标结构
3. `api/handler/kb`
   - 验证检索响应结构、citation/source 字段、审计辅助逻辑

### 5.4 通过标准

看到 `ok` 即可，例如：

```text
ok  	interview-agents/internal/config
ok  	interview-agents/internal/milvus/retrieval
ok  	interview-agents/api/handler/kb
```

### 5.5 如果这里没过，先不要继续

因为后面的接口联调和离线评测都会建立在这些基础逻辑之上。这里不过，后面得到的现象通常都是噪音。

---

## 6. 第二步：验证 L0 配置快照和启动日志

### 6.1 目标

确认这些事情已经生效：

1. Phase 2 feature flag 存在。
2. 启动时会打印策略摘要。
3. baseline 快照会落盘。

### 6.2 先检查配置项

文件：`backend/config.yaml`

重点看这几段：

```yaml
rag:
  enabled: true
  feature_flags:
    enable_hybrid_retrieval: false
    enable_query_rewrite: false
    enable_dynamic_topk: false
    enable_advanced_rerank: false
  thresholds:
    retrieve_timeout_ms: 3000
    user_qps_limit: 20
  phase2:
    hybrid_dense_weight: 0.7
    hybrid_sparse_weight: 0.3
    candidate_topk: 10
    min_topk: 3
    max_topk: 8
    rewrite_timeout_ms: 120
    rewrite_max_expansions: 3
    rerank_timeout_ms: 250
    rerank_model: "jaccard-v1"
```

### 6.3 启动服务

在 `backend` 目录执行：

```powershell
go run ./cmd/server
```

### 6.4 你应该重点看哪些日志

启动后重点搜索这些前缀：

```text
[RAG:L0] flags
[RAG:L0] phase2_flags
[RAG:L0] thresholds
[RAG:L0] phase2_params
```

如果日志正常，你应该能看到类似信息：

1. `phase2_flags hybrid=false rewrite=false dynamic_topk=false advanced_rerank=false`
2. `phase2_params dense_weight=0.700 sparse_weight=0.300 ...`

这一步验证的是“运行时到底用了什么策略”，而不是“你以为配置文件里写了什么”。

### 6.5 检查 baseline 快照文件

文件路径：

`backend/docs/baseline/phase1/baseline_snapshot.json`

第一次加载配置时，这个文件应该会被自动创建。

你至少确认三件事：

1. 文件存在。
2. 里面有 `feature_flags`、`thresholds`、`phase2`。
3. 里面有 `strategy_digest` 和 `config_version`。

### 6.6 通过标准

满足下面 3 条，就说明 L0 基本正常：

1. 服务能启动。
2. 日志里能看到完整 Phase 2 策略摘要。
3. baseline 快照文件已生成。

---

## 7. 第三步：验证 L0 的 fail-fast

### 7.1 目标

确认配置不合法时，系统会立刻拒绝启动。

这是 L0 很关键的一部分，因为它决定了错误是“上线前发现”，还是“线上请求时才发现”。

### 7.2 验证方法 A：故意把 hybrid 权重写错

把 `backend/config.yaml` 临时改成：

```yaml
rag:
  feature_flags:
    enable_hybrid_retrieval: true
  phase2:
    hybrid_dense_weight: 0.9
    hybrid_sparse_weight: 0.3
    candidate_topk: 10
```

这里的问题是：

`0.9 + 0.3 != 1`

然后重新启动：

```powershell
go run ./cmd/server
```

### 7.3 预期结果

服务应该启动失败，并提示：

1. RAG 配置非法
2. hybrid 权重不合法

### 7.4 验证方法 B：故意把 dynamic topk 范围写错

临时改成：

```yaml
rag:
  feature_flags:
    enable_dynamic_topk: true
  phase2:
    candidate_topk: 6
    min_topk: 8
    max_topk: 7
```

这里的问题是：

`min_topk > max_topk`

重新启动后，也应该直接失败。

### 7.5 做完记得恢复配置

这一步是破坏性验证，做完一定要把配置改回正常值，不然后面的联调会一直失败。

---

## 8. 第四步：准备接口联调环境

### 8.1 目标

在真实服务里验证：

1. dense-only 模式可用
2. hybrid 模式可用
3. 审计日志里能看到路由差异

### 8.2 前置条件

你至少需要具备这些条件：

1. 数据库、Redis、Milvus 可连通
2. 服务能正常启动
3. 已有一个可用知识库
4. 知识库里至少有一批已入库完成的文档
5. 你手里有一个有效的登录 token

如果第 3 和第 4 条还没准备好，先走一遍知识库上传和入库流程。

### 8.3 接口入口

路由在：
路由在：
路由在：

`backend/api/router/custom_kb.go`

关键接口：

1. `POST /api/kb/retrieve`
2. `GET /api/kb/retrieve/audit/:request_id`

---

## 9. 第五步：先验证 dense-only 路径

### 9.1 目标

确认在 hybrid 开关关闭时，系统仍然走原本的 dense 路径。

### 9.2 配置

确认：

```yaml
rag:
  feature_flags:
    enable_hybrid_retrieval: false
```

然后重启服务。

### 9.3 调用检索接口

下面是一个最小请求示例：

```powershell
curl.exe -X POST "http://127.0.0.1:8899/api/kb/retrieve" ^
  -H "Content-Type: application/json" ^
  -H "Authorization: Bearer <你的Token>" ^
  -d "{\"kb_id\":1,\"query\":\"goroutine 和 channel 的关系\",\"top_k\":5}"
```

如果你想一次查多个知识库，可以把 `kb_id` 改成 `kb_ids`。

### 9.4 你要看什么

先看响应体：

1. 是否返回 `request_id`
2. `items` 是否非空
3. 每个 item 是否包含：
   - `content`
   - `score`
   - `citation`
   - `source`

然后重点看 `source.route`。

在 dense-only 模式下，预期应以 `dense` 为主。

### 9.5 再看审计日志

把返回的 `request_id` 带到下面这个接口：

```powershell
curl.exe "http://127.0.0.1:8899/api/kb/retrieve/audit/<request_id>" ^
  -H "Authorization: Bearer <你的Token>"
```

你重点看这些字段：

1. `query`
2. `final_query`
3. `topk`
4. `candidate_topk`
5. `final_topk`
6. `routes`
7. `final_count`
8. `duration_ms`
9. `result_status`

在 dense-only 模式下，`routes` 预期是：

```text
dense
```

---

## 10. 第六步：打开 hybrid，验证 L1 主链路

### 10.1 目标

确认开关打开后，服务已经具备 `dense + sparse` 两路召回能力。

### 10.2 配置

把 `backend/config.yaml` 调整为：

```yaml
rag:
  feature_flags:
    enable_hybrid_retrieval: true
    enable_query_rewrite: false
    enable_dynamic_topk: false
    enable_advanced_rerank: false
  phase2:
    hybrid_dense_weight: 0.7
    hybrid_sparse_weight: 0.3
    candidate_topk: 10
```

然后重启服务。

### 10.3 再次调用同一类检索请求

```powershell
curl.exe -X POST "http://127.0.0.1:8899/api/kb/retrieve" ^
  -H "Content-Type: application/json" ^
  -H "Authorization: Bearer <你的Token>" ^
  -d "{\"kb_id\":1,\"query\":\"goroutine 和 channel 的关系\",\"top_k\":5}"
```

### 10.4 你要重点确认什么

先看响应体中的 `items[*].source.route`。

你可能会看到：

1. `dense`
2. `sparse`
3. 部分情况下表现为融合后的主路由

然后再看审计日志中的 `routes` 字段。

在 hybrid 模式下，预期应该是：

```text
dense+sparse
```

这一步非常关键。因为它验证的不是“某个文档恰好命中了”，而是“系统编排层已经切到了混合召回链路”。

### 10.5 建议选什么 query 来验证

L1 最适合用这些 query 验证：

1. 缩写词：`MVCC`
2. 实体词：`goroutine channel`
3. 明显关键词型 query：`Redis 持久化`

原因很简单：

这类 query 更容易体现 sparse 路由的价值。

---

## 11. 第七步：检查接口返回契约是否稳定

### 11.1 目标

确认混合检索接入后，没有把原有返回格式搞坏。

### 11.2 当前返回结构

`/api/kb/retrieve` 当前返回的最小契约是：

1. `request_id`
2. `items[].content`
3. `items[].score`
4. `items[].citation`
5. `items[].source`

其中 `source` 最小包含：

1. `route`
2. `collection`
3. `retriever_version`

### 11.3 怎么检查

直接看接口响应 JSON，确认这些字段始终存在。

如果你是前后端联调，建议把这份最小契约固定下来，不要只盯着“能不能返回内容”。

因为很多回归问题不是“查不到”，而是“查到了，但结构变了，前端或下游解析挂了”。

---

## 12. 第八步：跑离线回归评测

### 12.1 目标

这一步验证的是“效果对比”，不是“链路可用”。

也就是说：

前面的步骤是在回答“它跑不跑得起来”，这一节是在回答“它是不是比 baseline 更好”。

### 12.2 先修正评测集占位值

文件：

`backend/scripts/evaluation/dataset.json`

先把里面这些占位符替换掉：

1. `relevant_ids`
2. `citation_targets[].chunk_id`

要替换成你知识库里真实存在的 chunk id。

如果不替换，命令虽然能跑，但 recall / MRR / nDCG 基本没有参考意义。

### 12.3 运行命令

在 `backend` 目录执行：

```powershell
go run ./cmd/retrieval-eval `
  -config ./config.yaml `
  -dataset ./scripts/evaluation/dataset.json `
  -profiles ./scripts/evaluation/retrieval_strategy_profiles.example.json `
  -gates ./scripts/evaluation/retrieval_gate_thresholds.example.json `
  -output ./docs/retrieval-regression-report
```

### 12.4 默认策略组合

文件：

`backend/scripts/evaluation/retrieval_strategy_profiles.example.json`

默认会比这几组：

1. `dense_only`
2. `hybrid`
3. `hybrid_rewrite`
4. `hybrid_rewrite_dynamic_topk`

对你当前 L0/L1 来说，最关键的是先看：

1. `dense_only`
2. `hybrid`

### 12.5 门禁阈值

文件：

`backend/scripts/evaluation/retrieval_gate_thresholds.example.json`

当前默认阈值是：

1. `Recall@K delta >= 0.08`
2. `MRR delta >= 0`
3. `nDCG delta >= 0`
4. `Citation Accuracy delta >= 0`
5. `P95 latency regression ratio <= 0.2`

### 12.6 产物

执行后会生成：

1. `backend/docs/retrieval-regression-report.json`
2. `backend/docs/retrieval-regression-report.md`

### 12.7 怎么判断是否通过

看退出码：

1. `0`：评测完成，而且门禁通过
2. `2`：评测完成，但门禁没通过
3. 其他非 0：命令本身执行失败

### 12.8 对你当前阶段的建议

L0/L1 刚做完时，不要急着盯总分。

先重点看两件事：

1. `hybrid` 相比 `dense_only`，在实体词、缩写词 query 上是否有提升
2. 延迟是否出现明显不可接受退化

这才是 L1 最核心的验收方向。

---

## 13. 一份建议的验收清单

你可以直接照着打勾：

1. `go test ./internal/config ./internal/milvus/retrieval ./api/handler/kb` 通过
2. 服务启动日志里出现 `[RAG:L0] phase2_flags`
3. 服务启动日志里出现 `[RAG:L0] phase2_params`
4. `backend/docs/baseline/phase1/baseline_snapshot.json` 已生成
5. hybrid 关闭时，`/api/kb/retrieve` 可正常返回
6. hybrid 关闭时，审计日志 `routes=dense`
7. hybrid 打开时，`/api/kb/retrieve` 可正常返回
8. hybrid 打开时，审计日志 `routes=dense+sparse`
9. 返回结果仍包含 `content/score/citation/source`
10. 离线评测报告可正常生成

如果这 10 条都满足，L0 和 L1 基本就算是“功能正常”了。

---

## 14. 常见问题

### 14.1 为什么 hybrid 打开后，响应里不一定每条都显示 `sparse`

因为当前接口返回的是最终排序后的结果，每条结果的 `source.route` 表示主贡献路由，不等于“整个请求只走了这一条路”。

真正判断“有没有进入混合检索链路”，优先看审计日志里的：

`routes=dense+sparse`

### 14.2 为什么离线评测跑了，但指标很怪

最常见原因就是评测集里的 `relevant_ids` 还没替换成真实 chunk id。

### 14.3 为什么我只改了配置，结果却和预期不一样

优先检查两件事：

1. 启动日志里的 Phase 2 策略摘要
2. baseline / audit 里记录下来的真实参数

不要只看 `config.yaml` 表面值，因为配置还可能被环境变量覆盖。

---

## 15. 最后给你的建议

如果你现在是要做“L0/L1 验收”，最实用的顺序就是：

1. 先跑单元测试
2. 再看启动日志和 baseline 快照
3. 再做两次接口验证：`hybrid=false` 和 `hybrid=true`
4. 最后补离线评测

这样做的好处是，你能很快把问题定位到“代码逻辑”、“配置”、“数据”、“效果”四类中的哪一类，而不是混在一起查。
