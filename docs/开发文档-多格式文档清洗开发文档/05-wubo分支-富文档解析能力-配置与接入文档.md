# wubo 分支 · 富文档解析能力 · 配置与接入文档

> **配套文档**：
> - [00-wubo分支-富文档解析与切分链路-反推需求文档.md](file:///h:/GYT-CODE/rag-retrievalOps/gaoerDocs/00-wubo分支-富文档解析与切分链路-反推需求文档.md)
> - [03-wubo分支-富文档解析与切分链路-测试文档.md](file:///h:/GYT-CODE/rag-retrievalOps/gaoerDocs/03-wubo分支-富文档解析与切分链路-测试文档.md)
>
> **面向读者**：运维同学、业务测试同学、客户技术对接人
>
> **核心结论先看这里**：**无需单独解析文档，直接用前端的"文档上传"功能即可，系统自动按文件类型分流解析**。

---

## 一、整体架构与接入方式

### 1.1 架构图（简化版）

```
用户 / Admin 前端
    ↓
【上传文档】→ 保存源文件 → 创建入库任务 → 异步队列
                                                    ↓
                                    ┌─────────────────────────────────┐
                                    │  消费者自动判断文件类型分流        │
                                    │  - txt/md → 本地解析             │
                                    │  - pdf/docx/html → 外部解析服务  │
                                    └─────────────────────────────────┘
                                                    ↓
                                        规范化 → 智能切分 → 向量化 → 入库
```

### 1.2 关键结论（业务测试同学必看）

| 问题 | 答案 |
|---|---|
| **需要单独启动解析服务吗？** | **是的，pdf/docx/html 需要启动 docling 解析服务栈；txt/md 不需要** |
| **需要先单独解析文档再上传吗？** | **不需要**，直接用前端 Admin 的"文档上传"功能即可 |
| **系统会自动识别文件类型吗？** | **是的**，上传后消费者自动根据文件后缀分流到本地/外部解析 |
| **解析失败会怎么样？** | 生成错误 sidecar 文件，任务标记为 `failed`/`retrying`，可重试 |
| **不启动解析服务能上传吗？** | 能上传，但 pdf/docx/html 会解析失败，任务状态变成 `failed` |

---

## 二、环境配置说明

### 2.1 环境变量清单

在 `.env` 或 `docker-compose.yml` 中配置以下环境变量：

| 环境变量 | 说明 | 默认值 | 必填 |
|---|---|---|---|
| `DOCUMENT_PARSER_PROVIDER` | 解析提供者类型：`local` 或 `http` | `http` | 是 |
| `DOCUMENT_PARSER_ENGINE` | 解析引擎：`docling`（目前只支持这个） | `docling` | 是 |
| `DOCUMENT_PARSER_ENDPOINT` | 外部解析服务地址 | `http://parser-provider:9000/parse` | 是 |
| `DOCUMENT_PARSER_TIMEOUT_MS` | 解析超时时间（毫秒） | `60000`（1分钟） | 是 |
| `DOCUMENT_PARSER_STRICT_MODE` | 严格模式：遇到警告是否直接失败 | `true` | 是 |
| `DOCUMENT_PARSER_SAVE_SIDECAR` | 是否保存 sidecar 审计文件 | `true` | 是 |
| `OCR_PROVIDER` | OCR 服务提供者：`http` 或空（为空则不启用 OCR） | 空 | 否 |
| `OCR_ENDPOINT` | OCR 服务地址 | 空 | 否 |
| `OCR_TIMEOUT_MS` | OCR 超时时间（毫秒） | `30000` | 否 |
| `DOCLING_SERVE_IMAGE` | Docling Serve 镜像 | `quay.io/docling-project/docling-serve-cpu:latest` | 否 |
| `DOCLING_SERVE_PORT` | Docling Serve 端口 | `5001` | 否 |
| `PARSER_PROVIDER_PORT` | Parser Provider 适配器端口 | `9000` | 否 |

### 2.2 最小可用配置（复制即用）

```bash
# .env 中加入这几行即可
DOCUMENT_PARSER_PROVIDER=http
DOCUMENT_PARSER_ENGINE=docling
DOCUMENT_PARSER_ENDPOINT=http://parser-provider:9000/parse
DOCUMENT_PARSER_TIMEOUT_MS=60000
DOCUMENT_PARSER_STRICT_MODE=true
DOCUMENT_PARSER_SAVE_SIDECAR=true
```

---

## 三、启动步骤（运维同学必看）

### 3.1 方式一：Docker Compose 一键启动（推荐用于生产/测试环境）

这是最简单的方式，依赖 `docker-compose.yml` 中已定义的 `--profile parser`：

```bash
# 进入项目根目录
cd h:\GYT-CODE\rag-retrievalOps

# 启动解析服务栈（两个容器：docling-serve + parser-provider）
docker compose --profile parser up -d

# 验证是否启动成功
docker compose ps
# 预期输出：docling-serve Up、parser-provider Up

# 验证健康检查（可选）
curl http://localhost:9000/healthz
# 预期输出：ok 或 {"status":"ok"}
```

**启动后日志验证（可选）**：
```bash
# 查看 parser-provider 日志
docker compose logs -f parser-provider

# 查看 docling-serve 日志
docker compose logs -f docling-serve
```

### 3.2 方式二：脚本一键启动（推荐用于本地开发 Linux/Mac）

```bash
cd h:\GYT-CODE\rag-retrievalOps
bash backend/scripts/start-docling-parser-stack.sh
```

脚本会自动：
1. 拉取 docling-serve 镜像（如果不存在）
2. 启动 docling-serve 容器
3. 构建并启动 parser-provider 容器
4. 输出访问地址

### 3.3 方式三：手动启动（不推荐，仅用于调试）

```bash
# 第一步：启动 docling-serve
docker run -d \
  --name docling-serve \
  -p 5001:5001 \
  -e DOCLING_SERVE_ENABLE_UI=1 \
  quay.io/docling-project/docling-serve-cpu:latest

# 第二步：编译并启动 parser-provider（Go 本地环境）
cd backend
go build -o parser-provider ./cmd/parser-provider/main.go
./parser-provider
```

---

## 四、业务测试步骤（业务同学必看）

### 4.0 前置检查：确认解析服务已启动

在开始测试之前，先执行这一步：

```bash
# Windows PowerShell
curl http://localhost:9000/healthz

# 或浏览器访问
http://localhost:9000/healthz
```

✅ **预期结果**：返回 `ok` 或 `{"status":"ok"}`

❌ **如果失败**：回到第三章重新启动解析服务。

---

### 4.1 测试一：Markdown / TXT 文件（本地解析）

**目的**：验证本地解析路径可用，不依赖外部服务

| 步骤 | 操作 | 预期结果 |
|---|---|---|
| 1 | 打开 Admin 前端 `http://localhost:3003`，进入任意知识库 | 正常显示知识库页面 |
| 2 | 点击"上传文档"，选择一个 `.md` 或 `.txt` 文件（≤20MB） | 上传成功，返回 `document_id` 和 `job_id` |
| 3 | 刷新知识库文档列表，等待 5~10 秒 | 文档状态变成 `completed`，`chunk_count` 有值 |
| 4 | 在检索实验室输入文档中的某个问题 | 能检索到相关内容 |

**不需要启动 docling 解析服务也能测这一项！**

---

### 4.2 测试二：PDF 文件（含表格，推荐重点测）

**目的**：验证完整的外部解析链路（docling → parser-provider → canonical → chunking）

| 步骤 | 操作 | 预期结果 |
|---|---|---|
| 1 | 确认解析服务已启动（见 4.0 前置检查） | ✅ |
| 2 | 点击"上传文档"，选择一个带表格的 PDF（如产品报价单、参数表） | 上传成功 |
| 3 | 刷新等待 30~60 秒（视文件大小） | 文档状态变成 `completed` |
| 4 | 去 `backend/` 目录查找 sidecar 文件 | 存在 `{document_id}.normalized.json` |
| 5 | 打开 sidecar 文件查看 | `tables` 数组非空，表格已被识别 |
| 6 | 在检索实验室查询表格中的具体数据（如"XX 产品价格是多少"） | 能检索到准确答案 |

---

### 4.3 测试三：DOCX / HTML 文件

**目的**：验证其他富格式也能正常解析

| 步骤 | 操作 | 预期结果 |
|---|---|---|
| 1 | 上传一个 `.docx` Word 文档（含标题层级） | 上传成功，状态最终 completed |
| 2 | 上传一个 `.html` 或 `.htm` 网页文件 | 上传成功，状态最终 completed；HTML lead-in（标题前的正文）完整保留 |

---

### 4.4 测试四：重复文件不上传（哈希查重）

**目的**：验证同一文件重复上传不会重复解析

| 步骤 | 操作 | 预期结果 |
|---|---|---|
| 1 | 上传 4.2 测试用过的同一份 PDF | 接口立即返回（不用等 30 秒） |
| 2 | 查看返回 JSON | `reused=true` 字段为 `true`，`document_id` 与第一次相同 |

---

### 4.5 测试五：失败降级（可选）

**目的**：验证解析失败时系统行为可控

| 步骤 | 操作 | 预期结果 |
|---|---|---|
| 1 | 故意停止 docling-serve：<br>`docker compose stop docling-serve` | 容器变为 `Exited` |
| 2 | 上传一个新的 PDF | 上传成功，任务状态最终变成 `failed` |
| 3 | 去 `backend/` 目录查找 | 存在 `{document_id}.normalized.error.json`，含错误码和错误信息 |
| 4 | 恢复 docling-serve：<br>`docker compose start docling-serve` | 服务恢复，`retrying` 状态的任务会被补偿扫描器重试 |

---

### 4.6 测试六：多租户隔离（安全红线，必测）

**目的**：确认租户 A 的文档租户 B 绝对看不到

| 步骤 | 操作 | 预期结果 |
|---|---|---|
| 1 | 用租户 A 的账号上传一份 PDF，等待入库完成 | 成功 |
| 2 | 用租户 B 的账号，在检索实验室查询该 PDF 中的内容 | 检索结果为空（绝对查不到） |

---

## 五、支持的文件格式与大小限制

| 格式 | 扩展名 | 解析方式 | 单文件大小限制 |
|---|---|---|---|
| 纯文本 | `.txt` | 本地解析 | 20MB |
| Markdown | `.md` / `.markdown` | 本地解析 | 20MB |
| PDF | `.pdf` | 外部 Docling 解析 | 20MB |
| Word | `.docx` | 外部 Docling 解析 | 20MB |
| HTML 网页 | `.html` / `.htm` | 外部 Docling 解析 | 20MB |

> ⚠️ 注意：不支持 `.doc` 旧格式 Word 文件，请另存为 `.docx` 后上传。

---

## 六、排障指南

### 6.1 文档一直卡在 `pending` 状态

**可能原因**：
1. 异步消费者进程没启动
2. Redis 连接有问题

**排查步骤**：
```bash
# 查看 rag-server 日志
docker compose logs -f rag-server | Select-String "ingest"
```

### 6.2 文档状态变成 `failed`

**排查步骤**：
1. 去 `backend/` 目录找 `*.normalized.error.json`，看具体错误信息
2. 检查解析服务是否正常：`curl http://localhost:9000/healthz`
3. 查看 parser-provider 日志：`docker compose logs -f parser-provider`

**常见错误及解决**：
| 错误信息 | 原因 | 解决 |
|---|---|---|
| `connection refused` | 解析服务没启动或地址不对 | 按第三章重新启动解析服务 |
| `timeout` | 文件太大或解析服务太忙 | 增大 `DOCUMENT_PARSER_TIMEOUT_MS` |
| `unsupported file type` | 文件格式不支持 | 转成支持的格式再上传 |

### 6.3 检索不到表格内容

**排查步骤**：
1. 打开 sidecar 文件 `*.normalized.json`，确认 `tables` 数组非空
2. 检查 chunk metadata 中 `table_ids` 是否有值
3. 确认切分策略路由到了 `table-aware`

### 6.4 sidecar 文件在哪里

默认保存在 `backend/` 工作目录下，文件名格式：
- 成功：`{原文件名}.normalized.json`
- 失败：`{原文件名}.normalized.error.json`

---

## 七、常见问题 FAQ

### Q1：我只想用本地解析（txt/md），不想启动 docling 可以吗？
**A**：可以。把 `DOCUMENT_PARSER_PROVIDER` 改成 `local` 即可，此时 pdf/docx/html 会上传失败并提示不支持。

### Q2：sidecar 文件会越来越大，会自动清理吗？
**A**：目前不会自动清理，建议：
- 测试环境：定期手工删除或挂盘
- 生产环境：考虑挂载独立磁盘，设置定期清理策略（如保留 30 天）

### Q3：OCR 怎么启用？
**A**：配置 `OCR_PROVIDER=http` 和 `OCR_ENDPOINT` 即可启用 OCR 扫描件识别。目前只支持 HTTP 协议的 OCR 服务。

### Q4：能不能只给某些租户启用新解析器？
**A**：目前 V1.0 是全局开关，不支持按租户配置。后续迭代会支持按租户灰度发布。

### Q5：旧文档（已经入库的 txt/md）需要重新解析吗？
**A**：不需要。旧文档已经可用，新解析器只影响新上传的文档。

---

## 八、验收 Checklist（上线前确认）

| 序号 | 检查项 | 状态 | 备注 |
|---|---|---|---|
| 1 | 解析服务栈能正常启动 | ☐ | docker compose ps 两个容器 Up |
| 2 | `.md` / `.txt` 上传并检索成功 | ☐ | 不依赖 docling |
| 3 | 带表格的 PDF 上传并检索成功 | ☐ | 端到端核心链路 |
| 4 | `.docx` 上传成功 | ☐ | |
| 5 | 多租户隔离验证通过 | ☐ | 安全红线 |
| 6 | sidecar 文件生成正确 | ☐ | 审计要求 |
| 7 | 失败降级验证通过 | ☐ | 停掉 docling 后行为可控 |

---

## 文档版本

| 版本 | 日期 | 作者 | 变更 |
|---|---|---|---|
| v1.0 | 2026-06-17 | gaoerJJ（AI 协助） | 初版，含配置说明、启动步骤、业务测试步骤、排障指南 |
