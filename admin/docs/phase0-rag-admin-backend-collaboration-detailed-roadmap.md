# Phase 0 详细功能实现路线（RAG 管理后台后端配合）

## 1. 文档定位

本文档是 `admin/docs/rag-admin-frontend-roadmap.md` 中 Phase 0（P0）“管理台基础重构与知识库闭环”对应的后端详细执行手册，目标是把“后端需要配合”从简版协作说明，展开为可直接排期、拆工、联调、验收的任务路线。

它有三个用途：

1. 作为后端推进管理台 P0 配合工作的统一执行文档。
2. 作为前后端冻结 API 契约、字段口径、状态流转和错误分类的协作基线。
3. 作为 Phase 1 接入监控总览、结构化日志、trace 详情前的稳定后端底座。

本文档风格与以下文档保持一致：

1. `backend/docs/phase0-rag-baseline-detailed-roadmap.md`
2. `admin/docs/phase0-rag-admin-frontend-detailed-roadmap.md`

统一口径说明：

1. `知识库闭环` 固定指：知识库创建、文档上传、文档列表、文档删除、异步入库、任务状态追踪、失败任务重试、任务取消、检索测试。
2. `统一检索结果契约` 固定指：`request_id/items/content/score/citation/source`。
3. `citation` 在 P0 最小包含：`kb_id/document_id/file_name/chunk_id/chunk_index`。
4. `source` 在 P0 最小包含：`route/collection`；如果当前实现已经有 `retriever_version`，可以提前带上，但不作为 P0 强依赖。
5. `任务状态机` 在 P0 固定指：`pending -> processing -> completed/failed/cancelled`。
6. `Collection 一致性校验` 固定指：导入 Collection、查询 Collection、当前 active Collection 三者一致。
7. `最小联调字段` 固定指：文档列表中的 `ingest_duration_ms/last_ingest_job_id/chunk_count/file_hash`，以及任务列表中的 `stage/progress/retry_count/error_code/error_msg/started_at/finished_at`。

---

## 2. Phase 0 范围边界

## 2.1 本阶段必须完成

1. 提供管理台 P0 所需的知识库、文档、任务、检索最小可用 API。
2. 保证上传后走异步入库链路，任务状态可追踪，失败原因可读。
3. 保证检索接口稳定返回 `request_id/items/content/score/citation/source`。
4. 保证删除文档后，检索可见性同步收敛，不再命中已删除文档 chunk。
5. 补齐管理台 P0 展示所需的文档和任务附加字段，避免前端靠推断拼状态。
6. 在服务启动阶段完成关键配置校验，避免进入联调后才暴露基础配置错误。
7. 不影响现有非管理台链路，尤其是不影响当前简历/面试主流程。

## 2.2 本阶段明确不做

1. 不做混合检索、query rewrite、dynamic topK、rerank、父子块、证据拒答。
2. 不做完整结构化日志查询页和监控聚合 API，这些进入 Phase 1。
3. 不做离线评测平台、A/B 实验平台、策略中心，这些进入 Phase 2/3。
4. 不做企业级告警、审计、成本平台，这些进入 Phase 4。
5. 不在 P0 引入过重的“为后续预埋一切能力”的设计，先把管理台闭环底座做稳。

---

## 3. 目标与通过标准（Gate）

Phase 0 后端配合通过标准（全满足）：

1. 管理台可以创建知识库、上传 `pdf/md/txt`，并能在任务列表看到状态流转到最终态。
2. 检索测试结果稳定返回 `request_id/items/content/score/citation/source`，不存在同一次请求字段不完整的问题。
3. 删除文档后重新检索，不再命中该文档对应 chunk。
4. 失败任务带明确错误信息，至少能区分解析失败、embedding 失败、Milvus 写入失败、配置错误、人工取消。
5. 文档列表和任务列表能稳定返回前端 P0 所需字段，不需要前端通过多接口拼接状态。
6. 服务在 Milvus、Embedding、Collection 配置错误时会 fail-fast，而不是以半可用状态对外服务。
7. P0 联调完成后，前端可以在不等待 P1 后端能力的情况下继续推进管理台页面重构。

---

## 4. 实现路线总览（L0 -> L7）

Phase 0 后端配合按 8 条路线推进，按门禁顺序合流：

1. L0：现状盘点与契约冻结
2. L1：运行环境、启动校验与 Collection 一致性
3. L2：知识库域数据模型与持久化
4. L3：管理台最小 API 与路由落地
5. L4：异步入库任务编排、重试入口与取消入口
6. L5：检索结果统一契约与删除可见性收敛
7. L6：联调字段补齐、错误分类与最小观测
8. L7：回归验收、回滚预案与 Phase 1 交接

建议顺序：`L0 -> L1 -> L2 -> L3 + L4 -> L5 -> L6 -> L7`

---

## 5. 详细路线拆解

## 5.1 L0 现状盘点与契约冻结

### 目标

在开发前先冻结当前已有能力、P0 最小 API、字段口径和不可回退行为，避免联调期间来回改口径。

### 功能任务

1. 盘点当前已存在的知识库域能力：
   - 知识库列表与创建
   - 文档上传与列表
   - 文档删除
   - 入库任务列表
   - 失败任务重试
   - 任务取消
   - 检索测试
2. 盘点当前已有后端 API、消息队列、Milvus 接入和模型结构。
3. 冻结 P0 前端依赖的 API 清单：
   - `POST /api/admin/kb/bases`
   - `GET /api/admin/kb/bases`
   - `POST /api/admin/kb/documents/upload`
   - `GET /api/admin/kb/documents`
   - `GET /api/admin/kb/jobs/:job_id`
   - `POST /api/admin/kb/jobs/:job_id/retry`
   - `POST /api/admin/kb/jobs/:job_id/cancel`
   - `DELETE /api/admin/kb/documents/:document_id`
   - `POST /api/kb/retrieve`
4. 冻结 P0 不可回退能力：
   - 上传后任务能推进
   - 删除后结果不可见
   - 失败任务可读
   - 失败任务可重试
   - 任务可取消
   - 检索结果能展示引用
5. 输出一版 P0 API 示例，供前端先对齐类型。

### 验收

1. API 清单、字段清单、状态机清单明确。
2. 前后端对 P0 必做与不做边界达成一致。
3. 后续任何 P0 字段变更都有明确依据，而不是临时口头同步。

---

## 5.2 L1 运行环境、启动校验与 Collection 一致性

### 目标

先把管理台 P0 依赖的底层运行环境做稳，避免前端拆页完成后才发现后端基础配置不可用。

### 功能任务

1. 在服务启动阶段启用 MilvusManager 初始化。
2. 启动时执行健康检查并打印关键日志。
3. 对以下配置做必填校验：
   - Milvus 地址与认证
   - Embedding 配置
   - CollectionName
4. 增加 Collection 一致性校验：
   - 当前导入使用的 CollectionName 非空
   - 当前检索使用的 Collection 与导入使用的一致
   - 当前 active Collection 与配置一致
   - Collection 不存在时启动直接失败
5. 启动失败策略固定为 fail-fast，不允许以“部分功能不可用”的状态继续启动。

### 验收

1. 启动日志能明确显示 Milvus 初始化和 Collection 校验结果。
2. 配置错误时服务拒绝启动，报错可直接定位。
3. 联调同学可以通过启动日志快速判断 RAG 基线是否可用。

---

## 5.3 L2 知识库域数据模型与持久化

### 目标

定义 P0 后端稳定状态源，让知识库、文档、任务都能被管理台可靠消费。

### 功能任务

1. 落地 3 张核心表及对应模型：
   - `kb_knowledge_base`
   - `kb_document`
   - `kb_ingest_job`
2. `kb_knowledge_base` 最小字段建议：
   - `id`
   - `owner_admin_id`
   - `name`
   - `description`
   - `scope`
   - `status`
   - `created_at`
   - `updated_at`
3. `kb_document` 最小字段建议：
   - `id`
   - `kb_id`
   - `operator_admin_id`
   - `file_name`
   - `file_type`
   - `file_size`
   - `file_hash`
   - `storage_path`
   - `status`
   - `chunk_count`
   - `error_msg`
   - `deleted`
   - `created_at`
   - `updated_at`
4. `kb_ingest_job` 最小字段建议：
   - `id`
   - `kb_id`
   - `document_id`
   - `operator_admin_id`
   - `status`
   - `stage`
   - `progress`
   - `retry_count`
   - `error_code`
   - `error_msg`
   - `started_at`
   - `finished_at`
   - `created_at`
   - `updated_at`
5. 同一 `kb_id + file_hash` 增加重复上传保护，至少支持“拒绝重复”或“复用已有结果”二选一。
6. 在数据库初始化中注册 AutoMigrate。

### 验收

1. 三张表可自动迁移。
2. DAO 能稳定完成写入、查询、更新状态。
3. 管理台依赖字段均有明确来源，而不是运行时临时拼接。

---

## 5.4 L3 管理台最小 API 与路由落地

### 目标

给前端 P0 页面拆分提供稳定 API，而不是让前端持续追着后端问字段。

### 功能任务

1. 落地或冻结以下接口：
   - `POST /api/admin/kb/bases`
   - `GET /api/admin/kb/bases`
   - `POST /api/admin/kb/documents/upload`
   - `GET /api/admin/kb/documents?kb_id=`
   - `GET /api/admin/kb/jobs/:job_id`
   - `POST /api/admin/kb/jobs/:job_id/retry`
   - `POST /api/admin/kb/jobs/:job_id/cancel`
   - `DELETE /api/admin/kb/documents/:document_id`
   - `POST /api/kb/retrieve`
2. 所有接口统一沿用现有响应包装，不单独引入新格式。
3. 上传接口返回最小字段：
   - `document_id`
   - `job_id`
   - `status`
4. 文档列表返回补齐：
   - `ingest_duration_ms`
   - `last_ingest_job_id`
   - `chunk_count`
   - `file_hash`
5. 任务列表或任务详情返回补齐：
   - `stage`
   - `progress`
   - `retry_count`
   - `error_code`
   - `error_msg`
   - `started_at`
   - `finished_at`

### 验收

1. 所有 P0 API 可以通过 curl 或 Postman 独立打通。
2. 前端可以基于固定类型完成 P0 页面开发。
3. 同一字段不会在不同接口返回不同命名或不同语义。

---

## 5.5 L4 异步入库任务编排、重试入口与取消入口

### 目标

让上传请求快速返回，把重活放到任务链路，并让管理台对任务具备最小可操作性。

### 功能任务

1. MQ 新增知识库入库消息类型与最小 payload：
   - `operator_admin_id`
   - `kb_id`
   - `document_id`
   - `job_id`
   - `file_path`
   - `file_type`
2. 消费器新增知识库入库处理器。
3. 状态流转固定为：
   - `pending -> processing -> completed`
   - `pending/processing -> failed`
   - `pending/processing -> cancelled`
4. 最小消费流程：
   - 读取文件
   - 文本提取
   - 切块
   - 注入 metadata
   - embedding
   - 写入 Milvus
   - 回写 `chunk_count/status`
5. 提供失败任务重试入口：
   - 仅允许最终失败态重试
   - 重试后增加 `retry_count`
   - 保留上一次错误信息
6. 提供任务取消入口：
   - 仅允许 `pending/processing` 取消
   - 取消后任务进入 `cancelled`
   - 前端刷新任务列表可见取消结果

### 验收

1. 上传后能在管理台看到任务状态推进。
2. 失败任务可重试，取消任务可终止后续流程。
3. `chunk_count`、最终状态和错误信息能稳定回写。

---

## 5.6 L5 检索结果统一契约与删除可见性收敛

### 目标

把 P0 检索结果的展示契约一次打稳，同时保证删除后的检索结果收敛。

### 功能任务

1. 检索接口稳定返回：
   - `request_id`
   - `items`
2. 每条结果最小包含：
   - `content`
   - `score`
   - `citation`
   - `source`
3. `citation` 最小包含：
   - `kb_id`
   - `document_id`
   - `chunk_id`
   - `file_name`
   - `chunk_index`
4. `source` 最小包含：
   - `route`
   - `collection`
5. 检索范围严格绑定：
   - `kb_id`
   - 已启用知识库
   - 未删除文档
6. 删除文档时同步处理检索可见性：
   - 新检索不再召回已删除文档 chunk
   - 文档列表状态与检索可见性保持一致
7. 若当前已具备 `retriever_version`，允许提前返回，便于前端 P0 直接展示并为 P1 复用。

### 验收

1. 管理台检索测试结果能稳定展示 `score/citation/source`。
2. 删除文档后重新检索，不再召回已删除文档内容。
3. 不存在“部分结果没有 `score` 或 `citation`”的契约漂移问题。

---

## 5.7 L6 联调字段补齐、错误分类与最小观测

### 目标

让管理台 P0 不只是“能调用接口”，还知道“任务卡在哪、为什么失败、是否还能继续定位”。

### 功能任务

1. 补齐文档列表字段：
   - `ingest_duration_ms`
   - `last_ingest_job_id`
   - `chunk_count`
   - `file_hash`
2. 补齐任务状态字段：
   - `stage`
   - `progress`
   - `retry_count`
   - `error_code`
   - `error_msg`
   - `started_at`
   - `finished_at`
3. 错误分类最少覆盖：
   - `parse_failed`
   - `embedding_failed`
   - `milvus_write_failed`
   - `config_invalid`
   - `job_cancelled`
4. 增加最小结构化日志字段，至少覆盖：
   - 入库：`job_id/document_id/kb_id/status/stage/error_code/error_msg/duration_ms`
   - 检索：`request_id/query/kb_id/topk/final_count/duration_ms/status`
5. 保证失败文案可读，避免前端只能直接展示底层异常堆栈。

### 验收

1. 前端不需要依赖字符串模糊匹配来判断失败类型。
2. 任务列表和检索测试足以支持联调排查。
3. P1 做结构化日志和 trace 页面时，不需要推翻 P0 字段设计。

---

## 5.8 L7 回归验收、回滚预案与 Phase 1 交接

### 目标

把 P0 后端配合收口成一个可以稳定交给前端继续迭代的底座版本。

### 功能任务

1. 固化 P0 冒烟回归清单：
   - 创建知识库
   - 上传文档
   - 任务进入最终态
   - 检索命中新文档内容
   - 删除后不再命中
   - 失败任务可读
   - 失败任务可重试
   - 任务可取消
2. 固化回滚预案：
   - Milvus 初始化失败如何回滚
   - Collection 配错如何回滚
   - 新接口字段回退如何兼容前端
   - 消费器异常如何暂停入库
3. 冻结一版 API 示例：
   - 上传示例
   - 文档列表示例
   - 任务列表示例
   - 检索结果示例
4. 交接 Phase 1 预留点：
   - `request_id`
   - 结构化日志最小字段
   - `retriever_version` 可选字段
   - `stage` 与 `progress` 延展位

### 验收

1. 管理台 P0 主流程可重复演示。
2. 前端 P0 页面不再依赖后端临时解释字段。
3. Phase 1 可以基于 P0 底座继续做监控与日志可视化，而不是返工 P0 API。

---

## 6. 推荐协作节奏

1. 先完成 `L0 + L1`，冻结环境校验、Collection 口径、基础状态源。
2. 再完成 `L2`，把表结构、核心字段和重复上传策略固定下来。
3. `L3 + L4` 可以并行推进：一边打 API，一边打异步任务编排。
4. `L5` 必须在 `L3 + L4` 稳定后收口，因为检索契约会直接影响前端组件实现。
5. `L6 + L7` 用于联调、回归和文档收口，避免 P0 基础字段带病进入 P1。

---

## 7. 角色分工建议

1. 后端 A：`L1 + L2`，负责配置、模型、迁移、Collection 校验。
2. 后端 B：`L3 + L4`，负责管理台 API、上传入口、MQ 编排、状态机、重试和取消。
3. 后端 C：`L5 + L6`，负责检索契约、删除可见性、错误分类、最小结构化日志。
4. 前端：基于冻结后的 API 和字段完成知识库页、详情页、检索测试面板接入。
5. QA/联调负责人：执行 `L7` 冒烟回归、失败路径验证、删除验证、字段验收。

---

## 8. 阶段验收模板

P0 后端配合收口时，建议按以下模板填写：

1. 功能完成情况（按 `L0 ~ L7`）
2. 已冻结 API 清单
3. 已冻结字段清单
4. 状态机口径是否冻结
5. 检索契约是否冻结
6. 冒烟结果：
   - 上传
   - 入库
   - 删除
   - 重试
   - 取消
   - 检索
7. 已知遗留问题与负责人
8. 是否允许前端进入 Phase 1：是/否

---

## 9. Phase 0 完成后下一步

P0 后端配合完成后，下一步进入 Phase 1 的核心不是“马上做更多策略”，而是把 P0 已经跑通的链路沉淀成可筛选、可下钻、可回放的可观测能力：

1. 增加指标聚合 API。
2. 增加入库与检索结构化日志查询 API。
3. 增加 `request_id` 下钻详情。
4. 增加更稳定的 `stage_durations`、`empty_reason`、`status` 字段。
5. 让管理台从“能做知识库闭环”升级成“能看懂链路和问题原因”。

---

## 10. 维护规则

1. 任何 P0 后端字段变更，先改本文档，再改接口实现。
2. 新增错误码、任务状态、检索结果字段时，同步更新 `L3/L4/L5/L6` 对应章节。
3. 若某项能力推迟到 Phase 1，必须明确写入“已知遗留”，不能口头默认。
4. 本文档只描述 P0 管理台后端配合范围，不把 P1/P2/P3 的策略能力提前混入。
