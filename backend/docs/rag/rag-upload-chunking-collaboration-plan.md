# RAG 文档上传与切割功能建设方案（含协作开发与快速验证）

## 1. 文档目标

本文档用于指导本项目新增「RAG 完整文档上传 + 切割 + 入库 + 检索」能力，覆盖：

1. 功能建设大纲（按 P0/P1/P2 优先级）
2. 技术实现思路（结合当前代码现状）
3. 快速验证方案（本地与联调）
4. 多人协作分工与并行开发建议
5. 当前未完成功能的后续编写规范与接力方式

---

## 2. 当前项目现状（扫描结论）

### 2.1 已有能力

1. 已有 Milvus 能力模块（初始化、向量化、切割、索引、检索）：
   - `backend/internal/milvus/init.go`
   - `backend/internal/milvus/splitter/*`
   - `backend/internal/milvus/storage/*`
   - `backend/internal/milvus/retrieval/*`
2. 已有 Markdown 导入器，可做切割后入库：
   - `backend/internal/milvus/importer.go`
3. 已有异步消息消费框架（Redis Stream + Consumer）：
   - `backend/internal/mq/*`
4. 前端已有文件上传交互经验（简历上传页）：
   - `frontend/src/app/user/center/page.tsx`

### 2.2 关键缺口

1. 服务启动时 Milvus 初始化被注释，线上链路未真正启用：
   - `backend/cmd/server/main.go`
2. 现有上传流程是「简历解析」，不是「通用知识库文档入库」：
   - `backend/api/handler/resume/resume_service.go`
3. 缺少知识库实体（知识库、文档、任务）与对应 API。
4. 缺少文档入库状态可视化、失败重试、删除回收等管理能力。

---

## 3. 建设范围与非范围

### 3.1 本期范围

1. 支持上传文档并异步完成 RAG 入库（解析、切割、向量化、存储）。
2. 支持文档状态追踪（pending/processing/completed/failed）。
3. 支持按用户与知识库隔离检索。
4. 支持前端知识库管理页（上传、列表、状态、删除、重试）。

### 3.2 非范围（本期不做）

1. 多租户团队权限体系（P2）。
2. 外部知识源全量连接器（飞书/Notion/Git）自动同步（P2）。
3. 高级 OCR 表格结构化抽取（P2）。

---

## 4. 功能优先级大纲（P0 -> P2）

## 4.1 P0（必须先完成，保证端到端可用）

1. **Milvus 在线启用与健康检查**
   - 在 `main.go` 启用 `InitMilvusManager`。
   - 增加配置开关 `rag.enabled`，支持故障降级。
2. **新增知识库核心数据模型**
   - `kb_knowledge_base`（知识库）
   - `kb_document`（文档元信息）
   - `kb_ingest_job`（入库任务）
3. **新增 API（最小闭环）**
   - 创建知识库、上传文档、查询任务状态、列文档、删除文档、手工重试任务。
4. **异步入库流程**
   - 上传后落盘/OSS -> 任务入队 -> Consumer 消费 -> 解析/切割/向量化/入库 -> 状态回写。
5. **格式支持**
   - 最小支持：`pdf`、`md`、`txt`。
6. **检索打通**
   - 增加按 `user_id` + `kb_id` 的过滤检索接口。
7. **前端知识库管理页**
   - 上传、进度、失败提示、重试、删除。

**P0 验收标准**

1. 文档上传后 3 分钟内可查询到完成状态（普通文本文档）。
2. 检索接口可命中刚上传文档内容。
3. 删除文档后，检索结果中不再出现该文档 chunk。

## 4.2 P1（质量与稳定性）

1. 按文档类型切割策略配置（`chunk_size`, `overlap_size`, separators）。
2. 文档去重与幂等（基于 `file_hash`）。
3. 失败重试策略（指数退避 + 最大重试次数 + 死信标记）。
4. 可观测性指标：
   - 入库成功率、失败率、平均耗时、平均 chunk 数、检索命中率。
5. 语义检索质量优化：
   - Hybrid 检索 + rerank（复用现有 `hybrid_search` 结构）。

## 4.3 P2（扩展能力）

1. 多知识库权限（个人库/团队库）。
2. 外部连接器（飞书/Notion/Git）增量同步。
3. OCR 强化与多模态文档处理。
4. 自动评测闭环（接入 `backend/scripts/evaluation`）。

---

## 5. 技术实现思路（落地版）

## 5.1 总体架构

```text
Frontend Upload
   -> API: /api/kb/documents/upload
      -> 保存原文件 + 创建文档记录 + 创建 ingest_job(pending)
         -> MQ 发布 knowledge_ingest 消息
            -> Consumer 处理:
               1) 读取文件内容
               2) 统一转文本（pdf/md/txt）
               3) 切块（splitter）
               4) Embedding + Milvus Index
               5) 写回状态 completed/failed
   -> API: /api/kb/jobs/:id /api/kb/documents
      -> 前端轮询展示状态
```

## 5.2 后端模块拆分建议

1. `internal/service/kb`：知识库领域服务（新增）
2. `internal/service/kb/ingest`：入库任务编排（新增）
3. `internal/mq`：新增消息类型 `knowledge_ingest`
4. `internal/milvus`：复用 splitter/indexer/retriever，扩展过滤字段
5. `api/handler/kb` + `api/router/kb`：新增 API 路由层

## 5.3 数据模型建议

### `kb_knowledge_base`

1. `id`, `user_id`, `name`, `description`, `status`, `created_at`, `updated_at`
2. 索引：`(user_id, name)` 唯一

### `kb_document`

1. `id`, `kb_id`, `user_id`, `file_name`, `file_type`, `file_size`, `file_hash`, `storage_path`
2. `status`, `chunk_count`, `error_msg`, `created_at`, `updated_at`
3. 索引：`(kb_id, status)`, `(user_id, file_hash)`

### `kb_ingest_job`

1. `id`, `kb_id`, `document_id`, `user_id`, `status`, `retry_count`, `started_at`, `finished_at`
2. `error_msg`, `created_at`, `updated_at`
3. 索引：`(status, created_at)`, `(document_id)`

## 5.4 API 草案（P0）

1. `POST /api/kb/bases`
2. `GET /api/kb/bases`
3. `POST /api/kb/documents/upload`（multipart）
4. `GET /api/kb/documents?kb_id=`
5. `GET /api/kb/jobs/:job_id`
6. `POST /api/kb/jobs/:job_id/retry`
7. `DELETE /api/kb/documents/:document_id`
8. `POST /api/kb/retrieve`（query + kb_id + top_k）

## 5.5 切割策略建议

1. 默认配置复用 `DocumentSplitter`。
2. markdown 使用 `SplitMarkdown`。
3. pdf/txt 先转纯文本后 `SplitText`。
4. chunk 元数据必须包含：
   - `user_id`, `kb_id`, `document_id`, `chunk_index`, `total_chunks`, `file_name`, `file_hash`, `created_at`

## 5.6 检索隔离策略

1. 检索表达式必须包含 `user_id` 与 `kb_id`。
2. 所有 RAG 查询禁止跨用户默认集合。
3. 保留 `top_k` 上限（例如最大 20）防止成本飙升。

---

## 6. 快速验证方案（开发期）

## 6.1 环境准备

1. 启动依赖：
   - `docker-compose up -d mysql redis milvus etcd minio`
2. 启动后端：
   - `cd backend`
   - `go run cmd/server/main.go`
3. 确认健康：
   - `GET /health` 返回 200

## 6.2 冒烟验证（P0 必测）

1. 创建知识库：
   - `POST /api/kb/bases`
2. 上传文档：
   - `POST /api/kb/documents/upload`
3. 轮询任务状态：
   - `GET /api/kb/jobs/:job_id`
4. 查询文档列表：
   - `GET /api/kb/documents?kb_id=...`
5. 检索验证：
   - `POST /api/kb/retrieve`，输入文档中存在的关键句
6. 删除验证：
   - `DELETE /api/kb/documents/:id`
   - 再次检索应无法命中该文档 chunk

## 6.3 自动化验证建议

1. 单测：
   - parser、splitter、metadata 构造、表达式过滤构建
2. 集成测试：
   - 上传 -> 入队 -> 消费 -> 入库 -> 检索
3. 回归测试：
   - 简历上传链路不受影响
4. 压测（轻量）：
   - 并发 10 文档上传，观察失败率与耗时

## 6.4 验收清单（发布前）

1. 完整流程成功率 >= 95%（小文件）
2. 失败任务可重试并成功恢复
3. 日志可定位到每个 `job_id` 的全链路
4. 无跨用户数据泄漏

---

## 7. 多人协作开发方案（建议 6 人）

## 7.1 角色与职责

### A 负责人（架构与集成）

1. 负责整体方案、代码规范、接口契约冻结、里程碑推进
2. 负责 `main.go`、配置开关、风险控制、最终联调
3. 负责 PR 合并策略与发布检查

### B 后端同学（数据模型与 DAO）

1. 负责 DB 表设计与迁移脚本
2. 负责 `kb_knowledge_base/kb_document/kb_ingest_job` DAO
3. 负责状态机字段与重试计数逻辑

### C 后端同学（上传与任务编排）

1. 负责 `api/handler/kb` 上传接口与参数校验
2. 负责上传落存储、任务创建、MQ 发布
3. 负责 job 查询与重试 API

### D 后端同学（消费与 RAG 入库）

1. 负责 MQ consumer 扩展 `knowledge_ingest`
2. 负责解析、切割、metadata 注入、向量入库
3. 负责错误处理、幂等和补偿

### E 后端同学（检索与 Agent 接入）

1. 负责检索接口与表达式隔离（user_id/kb_id）
2. 负责在工具层接入新的检索能力
3. 负责检索质量优化（top_k, rerank）

### F 前端同学（知识库管理页）

1. 负责知识库页面与上传组件
2. 负责任务轮询、失败提示、重试操作、删除交互
3. 负责调用新 API 与状态映射

## 7.2 并行开发顺序（减少互相阻塞）

1. 第 1 阶段（D1-D2）
   - A/B 冻结数据结构与 API 契约
2. 第 2 阶段（D3-D5）
   - B/C/D/E/F 并行开发（按责任边界）
3. 第 3 阶段（D6-D7）
   - 联调与缺陷修复
4. 第 4 阶段（D8）
   - 灰度发布与监控验证

## 7.3 分支与 PR 规范

1. 分支命名：
   - `feature/rag-kb-model`
   - `feature/rag-kb-upload`
   - `feature/rag-kb-consumer`
   - `feature/rag-kb-retrieval`
   - `feature/rag-kb-frontend`
2. 每个 PR 必须包含：
   - 变更说明
   - 自测步骤
   - 风险点
   - 回滚方案
3. 禁止跨职责大 PR，单 PR 控制在一个模块内。

---

## 8. 未完成功能的后续编写与接力规则

## 8.1 当前未完成项清单

1. `main.go` 中 Milvus 初始化为注释状态。
2. 缺少 `kb` 业务模型与 API。
3. MQ 仅有 `resume_parse`，无 `knowledge_ingest`。
4. 缺少上传文档到 RAG 的独立状态机。
5. 前端没有知识库管理页面与任务监控交互。

## 8.2 后续编写规则（强制）

1. 新增能力不能破坏已有简历链路，保持接口兼容。
2. 所有任务状态流转必须可追踪：`pending -> processing -> completed/failed`。
3. 所有失败必须写入 `error_msg` 且可重试。
4. 新增检索必须带用户隔离过滤，不允许默认全局检索。
5. 所有可配置参数进入 `config.yaml`，禁止硬编码。
6. 每个功能提交必须附最小测试（单测或接口冒烟）。

## 8.3 接力开发模板

每个未完成功能都必须补全以下信息并写入 PR 描述：

1. 当前进度（完成到哪一步）
2. 剩余任务（可执行的 3-5 条）
3. 风险与依赖（阻塞人/阻塞模块）
4. 下一位接手同学的入口文件与建议顺序

---

## 9. 里程碑计划（建议）

1. **M1（P0 基础可用）**
   - API + 入库任务 + 前端上传页可用
2. **M2（P0 闭环达标）**
   - 检索可命中，删除可回收，状态完整
3. **M3（P1 稳定性）**
   - 幂等、重试、指标、质量优化
4. **M4（P2 扩展）**
   - 连接器、多知识库权限、评测闭环

---

## 10. 风险与应对

1. **风险：Milvus/Embedding 服务不稳定**
   - 应对：开关降级 + 重试 + 超时 + 告警
2. **风险：文档格式复杂导致解析失败**
   - 应对：先限制支持格式，失败可重试并透出错误
3. **风险：多人开发冲突**
   - 应对：先冻结契约，严格按模块 owner 提交 PR
4. **风险：检索越权**
   - 应对：后端强制拼接隔离过滤表达式

---

## 11. 推荐首批落地任务（可以直接开工）

1. A：启用 Milvus 初始化开关，补健康检查日志。
2. B：建三张 `kb_*` 表与 DAO。
3. C：实现 `POST /api/kb/documents/upload` 与 job 创建。
4. D：新增 `knowledge_ingest` consumer 并打通 splitter/indexer。
5. E：实现 `POST /api/kb/retrieve` 与隔离过滤。
6. F：新增前端知识库页面与状态轮询。

完成以上 6 项，即可进入 P0 联调。

