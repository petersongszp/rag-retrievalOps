# RAG RetrievalOps 平台

> 企业级 RAG 知识库管理与检索治理平台

---

## 项目简介

**rag-retrievalOps** 是一个面向企业的 RAG（Retrieval-Augmented Generation）管控平台，提供从知识入库、向量检索到策略治理、质量评估的全链路能力。主要包括RAG和监控平台两部分。

- `backend/`：RAG 后端服务，入口为 `backend/cmd/rag-server`
- `admin/`：RAG 管理后台，基于 Next.js

与简单的 RAG Demo 不同，本平台专注于解决 RAG 系统上线后的**运维治理问题**：检索效果怎么量化？策略怎么安全发布？出了问题怎么快速定位？成本花在了哪里？

平台面向多业务场景（客服、面试、销售、内部知识助手等），提供统一的检索 API 和管理后台，让业务团队无需自建 RAG 基础设施，即可获得可调优、可监控、可回滚的企业级检索能力。

---

## 技术栈

| 层级 | 技术选型 |
|------|----------|
| 后端框架 | Go 1.25.1 + [Hertz](https://github.com/cloudwego/hertz) |
| 数据库 | MySQL 8.0（GORM） |
| 缓存/消息队列 | Redis 7.0（Redis Stream） |
| 向量数据库 | [Milvus 2.3+](https://milvus.io/) |
| 对象存储 | MinIO |
| 前端框架 | Next.js 14+ / React / TypeScript |
| UI 组件库 | Ant Design |
| 监控 | Prometheus + Grafana |

---

## 快速启动

项目根目录下的.env.example，复制粘贴，命名为新的文件.env
(.env文件不要同步到git仓库，避免团队协作环境冲突)

### 方式一：Docker Compose（推荐）

一键启动所有服务，包括后端、前端、MySQL、Redis、Milvus、Attu。

```bash
# 1. 配置环境变量
cp .env.example .env
# 编辑 .env 设置数据库密码等

# 2. 启动所有服务
docker-compose up -d

# 3. 查看服务状态
docker-compose ps

# 4. 查看日志
docker-compose logs -f rag-server
```

**服务端口**

| 服务 | 端口 | 说明 |
|------|------|------|
| rag-server | 8899 | 后端 API |
| rag-admin | 3003 | 管理后台前端 |
| MySQL | 3308 | 数据库 |
| Redis | 6380 | 缓存/消息队列 |
| Milvus | 19531 | 向量数据库 |
| Attu | 8001 | Milvus 管理界面 |

**服务依赖关系**

```
rag-admin → rag-server → MySQL
                       → Redis
                       → Milvus
```

所有服务都配置了健康检查，会自动等待依赖服务就绪后再启动。

默认服务：

- RAG API: `http://localhost:8899`
- 管理后台: `http://localhost:3003`
- Attu: `http://localhost:8001`
- API 健康检查: `http://localhost:8899/healthz`

### 方式二：本地开发

**环境要求**

- Go 1.25.1+
- Node.js 18+
- MySQL 8.0+
- Redis 7.0+
- Milvus 2.3+

**后端启动**

```bash
cd backend
go run ./cmd/rag-server
```

**前端启动**

```bash
cd admin
npm install
npm run dev
```

前端默认运行在 `http://localhost:3001`。

---

## 目录结构

```
rag-retrievalOps/
├── backend/                     # RAG 后端服务
│   ├── cmd/rag-server/          # RAG 服务入口
│   ├── api/
│   │   ├── handler/kb/          # 知识库管理 API
│   │   └── handler/rag/         # 公共检索 API
│   ├── internal/
│   │   ├── milvus/              # Milvus 检索服务
│   │   ├── ragqueue/            # RAG 专用消息队列与入库消费
│   │   ├── model/               # RAG 数据模型
│   │   └── service/kb/          # RAG 评测与知识库服务
│   └── docs/                    
├── admin/                       # 管理后台前端
│   └── src/
│       ├── app/(admin)/         # 管理后台页面
│       └── components/admin/    # RAG 管理功能组件
└── docs/                        
```
---

## 配置说明

当前主要配置文件只有三类：

- 根目录 `.env`
- `backend/config.yaml`
- `admin/.env.local`

详细说明见 [配置-01-环境配置说明.md](docs/配置-01-环境配置说明.md)。

## 功能说明

### 核心亮点

#### 1. RAG 配置热加载

传统 RAG 系统的检索策略（TopK、混合权重、改写模型、重排序等）通常是写死在配置文件里的，改一次就要重启服务。

本平台通过 **Feature Flags 机制** 实现策略热加载：

- **运行时生效**：修改策略配置后立即生效，无需重启服务
- **灰度发布**：支持 `internal → 5% → 20% → 50% → 100%` 的渐进式发布
- **A/B 实验**：多个策略版本并行，按流量分组对比效果
- **一键回滚**：发现问题立即回滚到上一稳定版本
- **风险管控**：高风险策略必须经过 shadow 或 canary 阶段，不能直接全量开启

#### 2. 评测驱动的策略上线

策略上线前，必须通过评测集验证效果：

- **评测数据集**：管理 Query + 标准答案的评测用例集
- **Baseline vs Candidate**：对比新旧策略的检索效果
- **量化指标**：Recall@K、MRR、nDCG、Citation Accuracy、P95 Latency 等
- **质量门禁（Gate）**：自动判断新策略是否满足上线条件，不达标则禁止发布
- **Debug Trace**：每条评测样本都可追溯完整检索链路，定位失败原因

**典型工作流**：

```
调参 → 离线评测 → Gate 通过 → Shadow 灰度 → Canary 灰度 → 全量发布
                ↓
            Gate 不通过 → 继续调参
```

### 平台解决的核心问题

| 问题 | 解决方案 |
|------|----------|
| 文档怎么进来 | 知识入库：文档上传、解析、Chunk 切分、Embedding 生成 |
| 向量怎么写入 | Milvus 向量数据库存储与索引 |
| 检索怎么跑 | 混合检索（向量+关键词）、查询改写、重排序 |
| 效果怎么调 | 策略热加载 + 检索实验室实时调试 |
| 怎么验证效果 | 评测集 + Baseline vs Candidate + 质量门禁 |
| 策略怎么发布 | 灰度发布 + A/B 实验 + 一键回滚 |
| 问题怎么排查 | 检索日志 + Debug Trace + 审计追踪 |
| 成本怎么算 | 多维度成本归因（按 KB/App/策略/模型） |

---

## 模块一：RAG 检索核心

这是平台的核心能力，负责把非结构化文档变成可检索的知识。

### 1.1 知识入库

**文档从哪里进来，怎么变成向量？**

```
文档上传 → 文件解析 → Chunk 切分 → Embedding 生成 → Milvus 写入
```

| 能力 | 说明 |
|------|------|
| 文档上传 | 支持 PDF、Word、Markdown 等格式 |
| 入库任务 | 状态机管理，支持暂停/恢复/重试 |
| Chunk 切分 | 按段落/句子/固定长度切分 |
| 向量写入 | Embedding 后写入 Milvus，支持增量更新 |

**前端入口**：知识库管理 → 文档管理 → 入库任务

### 1.2 混合检索

**一次检索请求怎么跑？**

```
Query → 查询改写 → Dense 向量检索 + Sparse 关键词检索 → 融合 → 重排序 → 结果返回
```

| 能力 | 说明 |
|------|------|
| 向量检索（Dense） | 基于 Embedding 的语义相似度检索 |
| 关键词检索（Sparse） | 基于 BM25 的精确匹配检索 |
| 融合（Fusion） | 多路召回结果加权融合 |
| 查询改写（Rewrite） | 对用户 Query 进行扩展、纠错、意图识别 |
| 重排序（Rerank） | 使用 Cross-Encoder 对融合结果精排 |
| 动态 TopK | 根据查询意图动态调整召回数量 |

**前端入口**：检索实验室（Retrieval Lab）

### 1.3 父子检索

**为什么需要父子检索？**

单个 Chunk 可能丢失上下文。父子检索通过保留 Chunk 的父级文档，补全上下文信息。

| 能力 | 说明 |
|------|------|
| Parent-Child 块 | 子块用于精确匹配，父块用于补全上下文 |
| 上下文补全 | 返回结果时自动拼接父块内容 |

### 1.4 拒答与引用

**检索结果可信吗？**

| 能力 | 说明 |
|------|------|
| 证据不足拒答 | 当检索结果与 Query 相关度低时，返回拒答回复 |
| 引用一致性检查 | 验证生成答案与引用来源是否一致 |
| 引用溯源 | 每条结果标注来源文档、页码、Chunk 位置 |

**前端入口**：检索日志 → 引用检查

---

## 模块二：运维监控体系

RAG 跑起来之后，怎么知道它跑得好不好？这是监控系统解决的问题。

### 2.1 检索日志

**每一次检索请求都记录了什么？**

| 字段 | 说明 |
|------|------|
| request_id | 请求唯一标识，用于链路追踪 |
| query | 用户查询原文 |
| kb_ids | 命中的知识库 |
| strategy_version | 使用的策略版本 |
| results | 召回结果列表（content、score、source） |
| latency | 各阶段耗时（改写、检索、融合、重排） |
| empty_reason | 空召回原因（如有） |

**前端入口**：检索日志（Retrieval Logs）

### 2.2 评测体系

**怎么量化检索效果好不好？**

| 能力 | 说明 |
|------|------|
| 评测数据集 | 管理 Query + 标准答案的评测集 |
| 评测运行 | 对比 baseline 和 candidate 的检索结果 |
| 评测指标 | Recall@K、MRR、nDCG、Citation Support Score |
| 评测报告 | 可视化对比，支持导出 |

**前端入口**：评测中心（Evaluation Center）

### 2.3 策略管理

**检索策略怎么调、怎么发、怎么回滚？**

| 能力 | 说明 |
|------|------|
| 策略配置 | TopK、候选数、混合权重、改写策略、重排模型 |
| 策略版本 | baseline / candidate / active / rollback |
| 灰度发布 | internal → 5% → 20% → 50% → 100% |
| A/B 实验 | 多版本并行，按流量分组 |
| 一键回滚 | 发现问题立即回滚到上一稳定版本 |

**前端入口**：策略中心（Strategy Center）

### 2.4 成本追踪

**检索花了多少钱？钱花在哪了？**

| 维度 | 说明 |
|------|------|
| 按知识库 | 每个 KB 的检索成本 |
| 按应用 | 每个 App 的调用成本 |
| 按策略 | 不同策略的成本差异 |
| 按模型 | Embedding / Rerank 模型的 Token 消耗 |
| 高成本 Query | 识别异常高消耗的查询 |

**前端入口**：成本运营（Cost Ops）

### 2.5 审计与告警

**谁在什么时候做了什么操作？系统异常怎么通知？**

| 能力 | 说明 |
|------|------|
| 审计事件 | 记录知识库创建、文档删除、策略变更等操作 |
| 检索审计 | 记录每次检索的完整链路 |
| 异常告警 | 入库失败、检索超时、空召回率异常等 |
| 告警通道 | 飞书、邮件等 |

**前端入口**：审计中心（Audit Center）、告警中心（Alert Center）

### 2.6 质量监控

**当前检索质量是升了还是降了？**

把最近一次评测结果浓缩成摘要页，快速了解当前质量趋势。

**前端入口**：质量监控（Quality Monitor）

---

## API 接口

### 知识库管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/admin/kb` | 获取知识库列表 |
| POST | `/api/admin/kb` | 创建知识库 |
| POST | `/api/admin/kb/docs/upload` | 上传文档 |
| GET | `/api/admin/kb/jobs` | 获取入库任务列表 |

### 检索接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/retrieve` | 执行检索 |

### 评测接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/admin/kb/eval/runs` | 获取评测运行列表 |
| GET | `/api/admin/kb/eval/runs/:run_id/report` | 获取评测报告 |

---

## 适用场景

| 场景 | 典型用例 |
|------|----------|
| 面试 Agent | 技术题库、简历评估、追问建议 |
| 客服 Agent | 产品 FAQ、售后流程、政策条款 |
| 销售支持 | 产品资料、案例库、竞品话术 |
| 内部知识助手 | 制度文档、流程文档、研发规范 |
| 培训/考试 | 课程资料、题库、讲义 |

---

## 文档目录

| 文档 | 说明 |
|------|------|
| [RAG 平台战略规划与未来路线图](/docs/规划-01-RAG平台战略规划与未来路线图.md) | 平台发展方向与阶段规划 |
| [RAG 平台竞品分析与市场定位](/docs/规划-02-RAG平台竞品分析与市场定位.md) | 市场竞争格局与差异化定位 |
| [RAG 与后台管理平台学习指南](/docs/后台-06-RAG与后台管理平台学习指南.md) | 新人上手指南 |
| [后台页面功能详解](/docs/admin页面功能详解/) | 各页面详细使用说明 |

---

## License

Private Project