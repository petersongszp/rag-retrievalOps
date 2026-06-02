# Phase 0 详细功能实现路线（RAG 基线打通）

## 1. 文档定位

本文档是 Phase 0 的执行手册，目标是把“RAG 基线打通”拆成可直接实施的细颗粒任务路线。
它有两个用途：

1. 作为团队推进 Phase 0 的统一执行文档。
2. 作为后续功能实现的参考基线，指导持续迭代。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `统一检索结果契约` 固定指：`content/score/citation/source`。
2. `source` 在 Phase 0 最小包含：`route/collection`。
3. `Collection 一致性校验` 固定指：导入 Collection、查询 Collection、当前 active Collection 三者一致。
4. `结构化检索日志` 固定至少包含：`query/interview_user_id/kb_scope/expr/topk/rewrite/routes/final_count/duration_ms`。

---

## 2. Phase 0 范围边界

## 2.1 本阶段必须完成

1. 知识库创建、文档上传、异步入库、检索命中、删除回收。
2. 入库任务状态可追踪：`pending/processing/completed/failed`。
3. 检索结果返回最小引用元数据。
4. 检索路径统一返回稳定的 `score/citation/source` 字段。
5. 关键配置错误可在启动阶段被发现，而不是运行中隐性失败。

## 2.2 本阶段明确不做

1. 混合检索（向量 + BM25）。
2. 动态 TopK 策略。
3. 父子块检索。
4. 高级引用一致性校验与拒答策略。
5. 深度索引调优（参数扫描/自动调参）。

---

## 3. 目标与通过标准（Gate）

Phase 0 通过标准（全满足）：

1. 上传 `pdf/md/txt` 后可自动入库并成功检索。
2. 面试用户检索时统一命中 `global` 作用域下的已启用知识数据。
3. 删除文档后，不再检索到该文档 chunk。
4. 失败任务有明确错误信息，可定位问题。
5. 冒烟测试清单全通过，且不影响现有简历链路。

---

## 4. 实现路线总览（L0 -> L6）

Phase 0 按 7 条路线推进，按门禁顺序合流：

1. L0：基础开关与运行环境
2. L1：数据模型与持久化
3. L2：API 与路由（知识库域）
4. L3：异步任务编排（MQ + Consumer）
5. L4：Milvus 入库与检索范围控制
6. L5：前端最小管理页
7. L6：测试、验收、回滚预案

建议顺序：`L0 -> L1 -> L2 + L3 -> L4 -> L5 -> L6`

---

## 5. 详细路线拆解

## 5.1 L0 基础开关与运行环境

### 目标
让后端具备稳定运行 RAG 基线能力，明确失败行为。

### 功能任务

1. 在 `backend/cmd/server/main.go` 启用 MilvusManager 初始化。
2. 启动时执行健康检查并输出日志。
3. 初始化失败策略（Phase 0 建议 fail-fast，直接失败）。
4. 补充配置检查（Milvus/Embedding 必填项）。
5. 增加 Collection 一致性校验：
   - 当前配置的 CollectionName 非空
   - 查询使用 Collection 与导入使用 Collection 一致
   - Collection 不存在时启动直接报错

### 验收

1. 服务启动日志明确显示 Milvus 初始化成功。
2. Milvus 不可用时服务按预期失败，不出现半可用状态。
3. Collection 配错时可在启动日志中直接定位。

---

## 5.2 L1 数据模型与持久化

### 目标
定义知识库域最小数据结构，支撑上传、任务、状态追踪。

### 功能任务

新增模型文件（`backend/internal/model`）：

1. `kb_knowledge_base.go`
2. `kb_document.go`
3. `kb_ingest_job.go`

在 `backend/internal/repository/database.go` 增加 AutoMigrate 注册。

### 表结构（Phase 0 最小）

`kb_knowledge_base`
1. `id` PK
2. `owner_admin_id` index
3. `name`
4. `description`
5. `scope`（Phase 0 固定 `global`）
6. `status`（`active`/`disabled`）
7. `created_at/updated_at`

`kb_document`
1. `id` PK
2. `kb_id` index
3. `operator_admin_id` index
4. `file_name/file_type/file_size`
5. `file_hash` index
6. `storage_path`
7. `status`（`pending/processing/completed/failed`）
8. `chunk_count`
9. `error_msg`
10. `deleted` index
11. `created_at/updated_at`

补充要求：

1. `file_hash` 用于重复上传识别与幂等保护。
2. 同一 `kb_id + file_hash` 的重复上传，至少要支持“拒绝重复”或“复用已有结果”二选一。

`kb_ingest_job`
1. `id` PK
2. `kb_id/document_id/operator_admin_id` index
3. `status`
4. `retry_count`
5. `error_msg`
6. `started_at/finished_at`
7. `created_at/updated_at`

### 验收

1. 启动后 3 张表自动创建。
2. 能通过 DAO 写入/查询/更新状态。

---

## 5.3 L2 API 与路由（知识库域）

### 目标
提供前后端联调的最小闭环 API。

### 路由策略

不改 IDL 生成文件，走自定义路由（参考现有 `RegisterCustomRoutes` 机制）。

建议新增：

1. `backend/api/router/custom_kb.go`
2. `backend/api/handler/kb/*.go`

### API 列表（Phase 0）

1. `POST /api/admin/kb/bases`：创建知识库（管理后台）
2. `GET /api/admin/kb/bases`：知识库列表（管理后台）
3. `POST /api/admin/kb/documents/upload`：上传文档（管理后台）
4. `GET /api/admin/kb/documents?kb_id=`：文档列表（管理后台）
5. `GET /api/admin/kb/jobs/:job_id`：任务状态（管理后台）
6. `DELETE /api/admin/kb/documents/:document_id`：删除文档（管理后台）
7. `POST /api/kb/retrieve`：知识库检索（面试链路内部调用）

### 接口契约（关键字段）

上传返回：
1. `document_id`
2. `job_id`
3. `status`（初始 pending）

检索返回每条结果至少包含：
1. `content`
2. `score`
3. `citation`：
   - `kb_id`
   - `document_id`
   - `chunk_id`
   - `file_name`
   - `chunk_index`
4. `source`：
   - `route`（Phase 0 固定为 `dense`）
   - `collection`

### 验收

1. 全部 API 可通过 Postman/curl 调通。
2. 统一响应格式与现有 `response.Success/Error` 一致。
3. 检索接口所有结果都稳定带 `score/citation/source`。

---

## 5.4 L3 异步任务编排（MQ + Consumer）

### 目标
上传后不阻塞请求，后台异步完成入库。

### 功能任务

在 `backend/internal/mq/mq.go`：
1. 新增 `MessageTypeKnowledgeIngest`
2. 新增 payload 结构：
   - `operator_admin_id`
   - `kb_id`
   - `document_id`
   - `job_id`
   - `file_path`
   - `file_type`

在 `backend/internal/mq/consumer.go`：
1. 新增 `handleKnowledgeIngest`
2. 状态更新流程：
   - job: pending -> processing -> completed/failed
   - document: pending -> processing -> completed/failed

### 消费逻辑最小流程

1. 读取文件
2. 文本提取（`md/txt` 直接读，`pdf` 走现有提取逻辑）
3. 切块
4. 注入 metadata
5. 向量入库
6. 回写 chunk_count 和状态

补充要求：

1. 消费日志采用结构化字段，至少包含 `job_id/document_id/kb_id/operator_admin_id/status/error_msg/duration_ms`。
2. 对解析失败、embedding 失败、Milvus 写入失败做错误分类，便于 Phase 1 接重试与补偿。

### 验收

1. 上传后能看到 job 状态变化。
2. 消费失败时 status=failed 且 error_msg 可读。

---

## 5.5 L4 Milvus 入库与检索范围控制

### 目标
确保 RAG 检索可用，且面试链路稳定命中全局共享知识。

### 功能任务

1. 统一 chunk metadata 字段（Phase 0 固定）：
   - `operator_admin_id`
   - `kb_scope`（固定 `global`）
   - `kb_id`
   - `document_id`
   - `chunk_index`
   - `total_chunks`
   - `file_name`
   - `created_at`
2. 检索接口强制附加过滤表达式：
   - `kb_scope == "global"`
   - `kb_id == 系统配置的active_global_kb_id`（或 active 集合）
3. `top_k` 保护：
   - 默认 5
   - 最大 20
4. 检索执行路径统一：
   - 所有检索请求走同一检索方法
   - 保证 `score` 注入逻辑不因分支不同而丢失
5. 检索返回结果标准化：
   - `content/score/citation/source`
   - 后续 Phase 2 可无缝扩展 `route/rerank_score`

### 验收

1. 同文档关键词可检索命中。
2. 面试用户检索时可命中共享知识；未启用知识库不会被命中。
3. 任意检索路径下 `score` 字段不为空。

---

## 5.6 L5 前端最小管理页

### 目标
让业务同学可直接操作与验证基线功能。

### 功能任务

1. 在 `frontend/src/config/api.ts` 增加 `KB_API` 常量。
2. 新增页面 `frontend/src/app/admin/knowledge/page.tsx`（挂到管理后台）。
3. 最小交互：
   - 创建知识库
   - 上传文档
   - 轮询任务状态
   - 列表展示状态
   - 删除文档

### 验收

1. 前端页面无需脚本可完成闭环验证。
2. 失败状态有可读提示。

---

## 5.7 L6 测试、验收、回滚预案

### 目标
确保 Phase 0 可交付，并具备基础风险控制。

### 冒烟测试清单

1. 创建知识库成功。
2. 上传合法文件成功。
3. 状态轮询最终 completed。
4. 检索命中上传内容。
5. 删除文档后检索不命中。
6. 非法文件类型拒绝上传。
7. 大文件超限被拒绝。
8. 重复上传同一文件时，`file_hash` 去重策略按预期生效。
9. Collection 配错时服务启动失败且报错可读。
10. 检索结果稳定包含 `score/citation/source`。

### 回归测试清单

1. 简历上传链路不受影响。
2. 面试主流程接口不受影响。
3. 服务重启后数据与状态一致。
4. 原有 Milvus 检索链路未因统一检索入口而出现结果回退。

### 回滚预案（Phase 0）

1. 代码层面：通过 feature flag 临时关闭 `/api/admin/kb/*` 与 `/api/kb/retrieve` 路由注册。
2. 运行层面：消费器异常时暂停 knowledge ingest 消费。
3. 数据层面：仅软删除 kb_document，避免误删历史文件。

---

## 6. 推荐实施节奏（无固定时长）

## 6.1 阶段推进建议

1. 先完成 `L0 + L1`，确认基础设施和表结构稳定。
2. 再完成 `L2 + L3`，形成“上传-任务-消费”主链路。
3. 然后完成 `L4`，打通检索并落实共享范围控制。
4. 再完成 `L5`，提供可视化操作界面。
5. 最后执行 `L6`，完成验收与回滚准备。

## 6.2 并行与合流规则

1. 可并行：`L2` 与 `L3`。
2. 必须串行：`L4` 依赖 `L3` 完成，`L5` 依赖 `L2` 契约稳定。
3. 统一合流：全部功能通过 `L6` 验收后再进入 Phase 1。

---

## 7. 角色分工（建议）

1. 后端A：L0 + L1（配置、模型、迁移）
2. 后端B：L2（API、校验、响应）
3. 后端C：L3 + L4（消费、入库、检索）
4. 前端：L5（页面与轮询）
5. QA/联调：L6（测试、验收记录）

补充协作约束：

1. 后端A 与后端C 需先冻结 `kb_document` 与 chunk metadata 契约，再并行开发。
2. 后端B 与前端在联调前先冻结检索响应结构，至少确认 `score/citation/source` 字段命名。
3. QA 在 Phase 0 就要把“重复上传”“Collection 配错”“score 缺失”纳入固定回归项。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0~L6）
2. 冒烟测试结果（通过/失败 + 失败原因）
3. 回归测试结果
4. 指标快照：
   - 入库成功率
   - 入库平均耗时
   - 检索命中率（样本）
5. 遗留问题与负责人
6. 是否进入 Phase 1（是/否）

---

## 9. Phase 0 完成后下一步（明确路线衔接）

下一阶段固定进入 Phase 1（生产可用），按以下顺序：

1. 重试/补偿机制完善
2. 监控与告警补齐
3. 基础溯源增强
4. 权限与共享范围回归压测

完成 Phase 1 门禁后，再进入 Phase 2 的召回率优化（混合检索、动态 TopK、索引优化评测）。

---

## 10. 文档维护规则（作为后续实现参考基线）

1. 任何 Phase 0 范围变更，先改本文档再改代码。
2. 新增接口或字段必须同步更新“L2/L4 契约部分”。
3. 每次联调后补充“阶段验收模板”记录。
4. 后续我按本文档逐项实现时，以本版本为唯一参考。
