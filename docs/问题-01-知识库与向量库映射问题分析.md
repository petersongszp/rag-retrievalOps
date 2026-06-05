# 知识库与向量库映射问题分析

## 1. 结论先说

你现在系统里的“知识库”和“向量库 collection”不是一一对应关系。

当前实现实际上是：

1. 后台“新建知识库”只是在业务数据库里创建一条 `kb_knowledge_base` 记录。
2. 文档入库时没有从知识库上读取“应该写入哪个 Milvus collection”。
3. 上传、删除、检索默认都走全局配置里的同一个 collection，通常就是你现在看到的 `documents`。
4. 不同知识库之间的隔离，主要依赖文档 metadata 里的 `kb_id` 过滤，不是依赖不同 collection。

所以你现在看到的现象是正常的：

1. 后台能“新建知识库”，但这个动作不会新建向量 collection。
2. 后面不同知识库上传的文档，仍然可能一起进入同一个 Milvus collection。
3. 你感受到“后台数据库”和“向量库对不上”，本质上是因为当前系统把它们设计成了两层不同概念，但中间没有绑定字段。

## 2. 问题出在哪里

### 2.1 知识库表没有保存向量 collection

知识库模型 `backend/internal/model/kb_knowledge_base.go` 目前只有这些核心字段：

1. `id`
2. `name`
3. `description`
4. `status`

没有类似下面这样的字段：

1. `vector_collection`
2. `vector_database`
3. `index_version`

这意味着知识库记录本身不知道自己的向量数据应该落到哪里。

### 2.2 后台“新建知识库”页面也没有 collection 配置

前端新建知识库弹窗 `admin/src/components/admin/create-knowledge-base-modal.tsx` 只提交：

1. `name`
2. `description`

对应的后端创建接口在 `backend/api/handler/kb/handler.go:59` 和 `backend/api/handler/kb/handler.go:297`。

也就是说，前后端都没有“这个知识库绑定哪个 collection”的输入和持久化。

### 2.3 文档入库走的是全局 collection，不是知识库自己的 collection

上传入口在 `backend/api/handler/kb/handler.go:362`，真正异步入库在 `backend/internal/mq/consumer.go:268`。

这里的核心逻辑是：

1. 先根据 `kb_id` 创建文档记录和 ingest job。
2. 切分文档。
3. 调 `manager.GetIndexerService().Store(ctx, chunks)` 入 Milvus。

关键问题在 `backend/internal/mq/consumer.go:311`：

1. `Store` 没有传“当前知识库的 collection”。
2. `IndexerService` 在 Milvus 初始化时就绑定了一个默认 collection。

这个默认 collection 的来源在：

1. `backend/internal/milvus/init.go:65`
2. `backend/internal/config/config.go:1579`
3. `backend/internal/config/config.go:1595`

所以现在入库天然是“全局单 collection 模式”。

### 2.4 删除文档时也按全局 collection 删

删除逻辑在 `backend/api/handler/kb/handler.go:815`。

代码会直接取：

1. `config.Global.Milvus.GetCollection("knowledge")`
2. 如果没有，再退回 `config.Global.Milvus.CollectionName`

这同样不是按某个知识库自己的配置来的。

### 2.5 检索时也是先拿全局 collection

检索入口在 `backend/api/handler/kb/handler.go:831`。

这里在 `backend/api/handler/kb/handler.go:912` 先拿了全局 collection，然后再把 `kb_id` 当作 metadata 条件去过滤。

也就是说当前检索模型是：

1. 先进入同一个 collection。
2. 再按 `metadata["kb_id"]` 过滤。

不是：

1. 先确定知识库自己的 collection。
2. 再只在那个 collection 里检索。

## 3. 你现在后台里“新建数据库”为什么不是这个向量库

这里要把几个概念拆开，不然很容易混：

### 3.1 你后台现在新建的其实是“知识库记录”

不是 Milvus collection，也不是 Milvus database。

它本质是业务库 MySQL 里的一个业务实体，用来表达：

1. 这组文档叫什么名字
2. 描述是什么
3. 状态是什么

### 3.2 Milvus 里还有两个层次

Milvus 一般会涉及：

1. `DatabaseName`
2. `CollectionName`

你现在最接近“documents 库”的，实际更像是 Milvus 的 collection，而不是后台那个“知识库”。

### 3.3 当前代码里 Milvus database 还是全局配置，不是按知识库动态切

Milvus client 初始化时就绑定了 `DBName`，位置在 `backend/internal/milvus/init.go:65`。

所以当前系统即使从 Milvus 角度看，也不是“每个知识库一个 Milvus database”的路由模式，而是：

1. 一个全局 Milvus database
2. 一个默认 collection 或少量全局 collection
3. 业务侧再用 `kb_id` 区分文档归属

## 4. 当前实现的隐藏问题

除了“没有一一绑定”之外，现在还有一个更深一点的问题：

### 4.1 多知识库检索在逻辑上并没有真正完整支持

你后端里其实有多 KB 的输入设计：

1. `kb_id`
2. `kb_ids`

相关代码在：

1. `backend/api/handler/kb/handler.go:1780`
2. `backend/api/handler/kb/handler.go:1824`

表面上看，`buildKBFilterExpr` 能生成多 KB 条件。

但实际检索链路里，默认还是把“生效 KB”塞进 `ActiveGlobalKBID`：

1. `backend/internal/milvus/retrieval/retriever.go:182`
2. `backend/internal/milvus/retrieval/hybrid_search.go:165`
3. `backend/internal/milvus/retrieval/filter.go:24`
4. `backend/internal/milvus/retrieval/filter.go:25`

这说明现在“多 KB 检索”更像是半成品：

1. 接口层有多 KB 参数。
2. 但核心检索器仍偏向单个 active KB 过滤。

如果后面再引入“一个 KB 一个 collection”，这个问题会更明显，因为多 KB 可能跨多个 collection。

## 5. 解决方案有哪些

## 方案 A：继续单 collection，只靠 `kb_id` 区分

### 做法

1. 保持所有文档都进 `documents`
2. 检索时靠 `metadata.kb_id` 过滤
3. 后台不再试图把知识库和 collection 做一一对应

### 优点

1. 改动最小
2. 不用迁移历史数据
3. 运维简单

### 缺点

1. 不满足你“不同文档想用不同向量库”的诉求
2. 知识库和向量库无法一一映射
3. 后期做隔离、重建、迁移、容量管理会比较别扭

### 适用场景

1. 文档量不大
2. 只想先把功能跑通
3. 暂时不关心每个知识库独立治理

## 方案 B：一个知识库绑定一个 Milvus collection

这是最推荐的方案。

### 做法

在 `kb_knowledge_base` 表新增字段，例如：

1. `vector_collection`

然后实现以下规则：

1. 新建知识库时生成或填写 `vector_collection`
2. 上传文档时，按当前知识库的 `vector_collection` 入库
3. 删除文档时，按当前知识库的 `vector_collection` 删除向量
4. 检索时，优先使用当前知识库绑定的 `vector_collection`
5. 后台详情页展示“当前知识库绑定的 collection”

### 优点

1. 业务语义最清晰
2. 真正实现知识库和向量 collection 一一对应
3. 后面重建索引、独立迁移、按库治理都更方便

### 缺点

1. 需要补数据库字段和前后端联调
2. 多知识库联合检索需要做“跨 collection 聚合”
3. 历史已经进 `documents` 的数据需要迁移或兼容

### 适用场景

1. 你希望每个知识库有独立的向量空间
2. 后面会有多套文档域
3. 希望后台管理和向量层能明确对齐

## 方案 C：一个知识库绑定一个 Milvus database

这个方案不建议现在上。

### 原因

当前代码里 Milvus client 初始化就绑定了全局 `DBName`，见 `backend/internal/milvus/init.go:65`。

如果改成每个知识库对应不同 Milvus database，需要同时改：

1. Milvus client 生命周期
2. manager 缓存
3. 检索路由
4. 入库路由
5. 后台管理模型

这会比“按 collection 切”重很多。

### 结论

如果你现在说的“documents 库”其实是 collection，那优先做方案 B 就够了，不建议直接上方案 C。

## 6. 推荐落地方案

推荐采用方案 B：一个知识库绑定一个 collection。

建议的设计如下。

### 6.1 数据模型

在 `kb_knowledge_base` 新增：

1. `vector_collection`

建议规则：

1. 新建时可手填
2. 如果不手填，系统自动生成，比如 `kb_<id>` 或 `kb_<sanitized_name>`
3. 建议全局唯一

### 6.2 后台页面

新建知识库弹窗增加一个可选字段：

1. `vector_collection`

知识库详情页展示：

1. 知识库名称
2. 描述
3. 状态
4. 绑定的 Milvus collection

这样后台看到的“知识库”就能和向量层真正对应上。

### 6.3 入库逻辑

上传时不要再直接走全局默认 collection，而是：

1. 根据 `kb_id` 读出知识库
2. 取该知识库的 `vector_collection`
3. 将切分后的 chunks 存入该 collection

### 6.4 删除逻辑

删除文档时：

1. 根据文档找到 `kb_id`
2. 再找到知识库对应的 `vector_collection`
3. 只删这个 collection 里的向量

### 6.5 检索逻辑

单知识库检索时：

1. 直接使用该知识库自己的 collection

多知识库检索时：

1. 先按 collection 分组
2. 对每个 collection 分别检索
3. 合并结果再排序裁剪 topK

这一步是整个方案里技术上最关键的点。

## 7. 历史数据怎么处理

如果你现在已有数据全在 `documents`，建议分两阶段处理。

### 第一阶段：先兼容老数据

规则可以是：

1. 老知识库 `vector_collection` 为空时，默认仍指向全局 `documents`
2. 新建知识库开始强制绑定自己的 collection

这样不会影响现网已存在数据。

### 第二阶段：再做迁移

后续如果要彻底清理，可以做迁移任务：

1. 扫描旧知识库文档
2. 按 `kb_id` 重写入各自 collection
3. 校验通过后，再从 `documents` 删除旧向量

## 8. 推荐实施顺序

如果后面要真正开始改，我建议按这个顺序做：

1. 给 `kb_knowledge_base` 增加 `vector_collection`
2. 后台创建知识库弹窗增加 collection 字段
3. 知识库详情页展示当前绑定 collection
4. 上传入库改为按 KB collection 写入
5. 删除改为按 KB collection 删除
6. 单知识库检索改为按 KB collection 检索
7. 多知识库检索改为“跨 collection 聚合”
8. 最后再考虑历史数据迁移

## 9. 最终建议

如果你的目标是：

1. 不同知识库真正对应不同向量空间
2. 后台管理和向量库能一眼对应
3. 后面方便做独立重建和治理

那就不要继续维持“所有文档都进 `documents`，只靠 `kb_id` 过滤”的模式了。

最合适的路线是：

1. 把“知识库”从纯业务概念升级为“业务记录 + collection 绑定”
2. 以后新知识库默认一个独立 collection
3. 历史 `documents` 数据走兼容后迁移

## 10. 一句话总结

你的问题不是 Milvus 本身有问题，而是当前系统只实现了“知识库记录”和“向量数据”两层概念，却没有把两者通过 `vector_collection` 这种字段绑定起来，所以看起来后台能建库，但向量层始终还是进同一个 `documents` collection。
