# wubo 分支 · 富文档解析与切分链路 · 验收报告

> **配套文档**：[00-wubo分支-富文档解析与切分链路-反推需求文档.md](file:///h:/GYT-CODE/rag-retrievalOps/gaoerDocs/00-wubo分支-富文档解析与切分链路-反推需求文档.md)
>
> **验收范围**：仅覆盖 wubo（profoundwu）本人在 wubo 分支上的提交，不含 MCP Server、面试题文档等他人合并进来的内容。
>
> **验收方式**：白盒走读 + 单元测试执行 + `go vet` 静态检查。本报告未做端到端冒烟（需要 docker-compose 起 docling-serve 实环境，留待与 wubo 一起执行）。

---

## 一、一句话结论

> **有条件通过**。  
> 核心代码、测试、静态检查均通过；架构设计、模块边界、可审计性达到企业级合格线。  
> 但存在 1 项 P0 红线问题（仓库历史包含已构建二进制需确认彻底清理）和若干 P1/P2 改进项（详见第六节）。

---

## 二、技术栈梳理（在你之前没有梳理的地方先补齐）

### 2.1 后端核心技术栈（Go 侧）

| 技术 / 组件 | 版本 | 在本次改动中的角色 |
|---|---|---|
| Go | 1.25.1 | 主语言 |
| Hertz | v0.10.3 | RAG 后端 HTTP 框架（既有），新增上传校验扩展 |
| Eino（cloudwego） | v0.7.28 + 多个 ext | 既有的 RAG 编排框架，本次切分策略产物 `*schema.Document` 来自 Eino schema |
| eino-ext / splitter / recursive | v0.0.0-…-33cdd47ff03a | markdown 兜底策略中的递归字符切分器 |
| eino-ext / embedding / ark | v0.1.1 | 火山方舟 embedding（本次新增 ark api type 兼容） |
| eino-ext / indexer / milvus、retriever / milvus | v0.0.0-…-33cdd47ff03a | Milvus 写入与检索 |
| **goldmark** | **v1.7.13** | **本次新引入：用于在 markdown 兜底策略中解析 Markdown 标题（heading），按章节切片** |
| milvus-sdk-go/v2 | v2.4.2 | 向量库客户端（既有） |
| golang.org/x/text/unicode/norm | indirect | **本次 canonical normalizer 用于 NFC Unicode 兼容归一化** |
| Prometheus client_golang | v1.23.2 | `metrics.ObserveIngest` 上报入库指标 |
| Redis go-redis | v9.16.0 | ragqueue 消息队列（既有） |

> 备注：`go.mod` 还含 `modelcontextprotocol/go-sdk`，那是 gaoerjj（你）合进来的 MCP 依赖，**不算 wubo 引入**。

### 2.2 外部解析服务栈（Python / 容器侧）

| 组件 | 镜像 | 角色 |
|---|---|---|
| **docling-serve** | `quay.io/docling-project/docling-serve-cpu:latest` | **Docling 官方提供的 HTTP 文档解析服务** |
| **parser-provider**（自研） | 由 [Dockerfile.parser-provider](file:///h:/GYT-CODE/rag-retrievalOps/backend/Dockerfile.parser-provider) 构建 | wubo 写的轻量 Go HTTP adapter，桥接 RAG 后端 ↔ docling-serve |
| Alpine 3.18 + Go 1.25.1 | parser-provider 运行基础镜像 | 多阶段构建，二进制入口 `./parser-provider` |
| Docker Compose | docker-compose.yml | 通过 `profiles: ["parser"]` 隔离按需启动 |

### 2.3 数据 / 存储栈（既有，非本次新增）
- MySQL 8.0：`KBDocument`、`KBIngestJob`、`audit_event` 等业务表
- Redis 7-alpine：消息队列 + 限流
- Milvus v2.4.23：向量库（standalone 模式，内嵌 etcd）
- Attu v2.4.12：Milvus 可视化管理 UI

---

## 三、专项介绍：`rag-platform-docling-serve-1` 是什么？

### 3.1 名字怎么来的
- `rag-platform`：来自 [docker-compose.yml](file:///h:/GYT-CODE/rag-retrievalOps/docker-compose.yml#L1-L1) 顶部的 `name: rag-platform`，作为 compose project 名。
- `docling-serve`：来自 service 定义 [docker-compose.yml](file:///h:/GYT-CODE/rag-retrievalOps/docker-compose.yml#L48-L57)。
- `-1`：Docker Compose v2 默认会给每个 service 起一个 replica，命名后缀是 `-1`、`-2` 之类。

所以这个容器实质上就是 compose 启出来的 **docling-serve service** 的实例。

### 3.2 docling-serve 是什么

**Docling**（[docling-project](https://github.com/docling-project/docling)）是 IBM Research 在 2024 年开源的文档解析项目，定位是「把 PDF / DOCX / HTML / PPTX / 图片 等各种富文档，统一解析成可结构化的中间表示（含 Markdown / JSON）」。它内部集成了 PDF 解析（pypdfium2 / DLParse）、OCR（EasyOCR / Tesseract）、表格识别（TableFormer 等）等能力。

**docling-serve** 是 Docling 提供的 **HTTP 服务包装**，把 Docling 的解析能力暴露成 REST API：
- `POST /v1/convert/file`：multipart 上传文件，返回 Markdown/JSON
- `/ui`：内置可视化调试 UI（端口 5001）
- `/docs`：OpenAPI Swagger 文档

镜像后缀 `-cpu` 表示 CPU 版本（不依赖 GPU），适合本地开发与轻量部署；如需高吞吐可换 `-gpu` 镜像。

### 3.3 在本项目的拓扑里它处于哪一层

```text
[用户/前端]
   │
   ▼
[rag-server]  ← Go 后端，本项目主体
   │ HTTPProvider.Parse() 走 multipart POST
   ▼
[parser-provider]  ← wubo 自研 Go adapter，端口 9000，路径 /parse
   │ DoclingClient.Parse() 走 multipart POST
   ▼
[docling-serve]  ← Docling 官方 Python 服务，端口 5001，路径 /v1/convert/file
   │
   ▼
（内部调用 pypdfium2 / OCR / TableFormer 等做实际解析）
```

### 3.4 为什么不让 rag-server 直连 docling-serve？

wubo 中间多放一层 `parser-provider` 是有意为之，主要好处是：

1. **协议解耦**：RAG 后端只需要懂自己的 `NormalizedDocument` JSON Schema；Docling 的请求字段（`to_formats=md` / `to_formats=json` / `image_export_mode=placeholder` / `pdf_backend=pypdfium2` 等）和它返回的复杂结构，全部被 adapter 吸收。
2. **替换灵活**：未来如果换成 MinerU、Unstructured.io、自研解析器，只需要替换 parser-provider 的实现，RAG 后端零感知。
3. **质量补丁就近做**：HTML lead-in 修复、Docling 表格 span 延展、表格规范化、Extracted Tables 追加等，都是「靠近解析器侧」的修复，放在 adapter 里最合理。
4. **运维边界清晰**：parser-provider 与 docling-serve 一起 `--profile parser` 按需启停，不影响主链路。

### 3.5 启动方式

| 场景 | 命令 |
|---|---|
| 全栈一键 | `docker compose --profile parser up -d docling-serve parser-provider` |
| 一键脚本（Linux/Mac） | `bash backend/scripts/start-docling-parser-stack.sh` |
| 仅起 docling 测试 UI | `bash backend/scripts/start-docling-serve.sh` 然后访问 `http://localhost:5001/ui` |

---

## 四、单元测试与静态检查执行结果

### 4.1 单元测试（执行命令 + 结果）

```powershell
# 命令一：解析 + 切分四件套
go test ./internal/documentparser/... ./internal/parserprovider/... ./internal/milvus/chunking/... -count=1 -timeout 120s
```

| 包 | 结果 | 耗时 |
|---|---|---|
| `internal/documentparser` | ✅ ok | 0.355s |
| `internal/documentparser/canonical` | ✅ ok | 0.311s |
| `internal/parserprovider` | ✅ ok | 0.324s |
| `internal/milvus/chunking` | ✅ ok | 0.367s |

```powershell
# 命令二：入库消费者 + 检索侧自闭环修复
go test ./internal/ragqueue/... ./internal/milvus/retrieval/... -count=1 -timeout 120s
```

| 包 | 结果 | 耗时 |
|---|---|---|
| `internal/ragqueue` | ✅ ok | 6.750s |
| `internal/milvus/retrieval` | ✅ ok | 6.555s |

### 4.2 静态检查

```powershell
go vet ./internal/documentparser/... ./internal/parserprovider/... ./internal/milvus/chunking/... ./internal/ragqueue/...
```

✅ 无任何告警输出。

---

## 五、需求逐条比对（FR/NFR ↔ 实现）

### 5.1 功能性需求逐项验收

| 需求编号 | 需求摘要 | 实现位置 | 测试覆盖 | 结论 |
|---|---|---|---|---|
| FR-A-01 | 上传支持 7 种类型 | [handler.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler.go) | [handler_upload_validation_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler_upload_validation_test.go) | ✅ |
| FR-A-02 | 20MB 上限、空文件拒绝 | 同上 | 同上 | ✅ |
| FR-A-03 | 同 KB 哈希查重 reused=true | 同上 | 现有单测覆盖；建议补集成 | ⚠️ |
| FR-A-04 | 前端同步支持新格式 | [knowledge-base-detail-page.tsx](file:///h:/GYT-CODE/rag-retrievalOps/admin/src/components/admin/knowledge-base-detail-page.tsx) | [knowledge-base-detail-page.test.tsx](file:///h:/GYT-CODE/rag-retrievalOps/admin/src/__tests__/knowledge-base-detail-page.test.tsx) | ✅ |
| FR-B-01 | NormalizedDocument 模型 | [types.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/types.go) | [types_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/types_test.go) | ✅ |
| FR-B-02 | NormalizeLocal 处理 txt/md | [local.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/local.go) | [local_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/local_test.go) | ✅ |
| FR-B-03 | Markdown pipe table 规范化 | `NormalizeMarkdownPipeTables` | local_test | ✅ |
| FR-B-04 | HTTPProvider 多部分请求 + ProviderError | [http_provider.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/http_provider.go) | [http_provider_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/http_provider_test.go) | ✅ |
| FR-B-05 | HTML lead-in 修复 | [html_leadin.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/html_leadin.go) | http_provider_test | ✅ |
| FR-C-01 | canonical 二次归一化保留 raw | [canonical/normalizer.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/canonical/normalizer.go) | [normalizer_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/canonical/normalizer_test.go) | ✅ |
| FR-C-02 | 9 条规则全部落地 | normalizer.go | normalizer_test | ✅ |
| FR-C-03 | canonicalization 元数据完整 | normalizer.go | normalizer_test | ✅ |
| FR-C-04 | 保守边界（不改业务字符） | 仅做结构层规则 | 未发现违反；建议补反例单测 | ⚠️ |
| FR-D-01 | parser-provider /parse + /healthz | [server.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/parserprovider/server.go) | docling_test | ✅ |
| FR-D-02~05 | Docling 适配链路 | [docling.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/parserprovider/docling.go) | [docling_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/parserprovider/docling_test.go)（637 行测试） | ✅ |
| FR-D-06 | PDF 表格 span 延展 | docling.go（含 `maxDoclingFragmentedTableExtensionBytes=4096`） | docling_test | ✅ |
| FR-D-07 | 表格行/列/单元上限保护 | 常量 `maxDoclingTableRows/Cols/Cells` | docling_test | ✅ |
| FR-D-08 | pdf_backend=pypdfium2 透传 | docling.go + http_provider | docling_test | ✅ |
| FR-D-09/10 | 启动脚本 + Dockerfile | [start-docling-parser-stack.sh](file:///h:/GYT-CODE/rag-retrievalOps/backend/scripts/start-docling-parser-stack.sh)、[Dockerfile.parser-provider](file:///h:/GYT-CODE/rag-retrievalOps/backend/Dockerfile.parser-provider) | 需端到端冒烟 | ⚠️ |
| FR-E-01/02 | sidecar 成功 / 错误两份 | [sidecar.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/sidecar.go) | [sidecar_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/sidecar_test.go) | ✅ |
| FR-E-03 | normalized_path 注入 chunk metadata | finalizeChunks（[helpers.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/helpers.go)） | router_test | ✅ |
| FR-E-04 | provenance 信息记录 | [provenance.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/provenance.go) | [provenance_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/provenance_test.go) | ✅ |
| FR-F-01/02 | 路由优先级 + chunking_route 标记 | [router.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/router.go) | [router_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/router_test.go) | ✅ |
| FR-F-03 | table-aware | [table_strategy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/table_strategy.go) | [table_strategy_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/table_strategy_test.go) | ✅ |
| FR-F-04 | ocr-aware + weak_evidence | [ocr_strategy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/ocr_strategy.go) | [ocr_strategy_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/ocr_strategy_test.go) | ✅ |
| FR-F-05 | structure-aware | [structure_strategy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/structure_strategy.go) | [structure_strategy_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/structure_strategy_test.go) | ✅ |
| FR-F-06 | markdown 兜底 + goldmark | [markdown_strategy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/markdown_strategy.go) | [markdown_strategy_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/markdown_strategy_test.go) | ✅ |
| FR-F-07 | 计费公式 → chunking_unit=formula | markdown_strategy | markdown_strategy_test | ✅ |
| FR-F-08 | finalizeChunks 父子元数据 | helpers.go（`phase3-parent-child-v1`） | router_test | ✅ |
| FR-F-09 | UTF-8 边界保护 | helpers.go `utf8Boundary` | 多策略测试覆盖 | ✅ |
| FR-G-01~06 | 入库消费者全流程 | [consumer.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/ragqueue/consumer.go) | [consumer_parser_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/ragqueue/consumer_parser_test.go) | ✅ |
| FR-H-01~06 | 检索侧自闭环修复 | retrieval/* | 各 *_test.go | ✅ |
| FR-I-01 | ark api type 兼容 | [embedding.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/storage/embedding.go) | [embedding_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/storage/embedding_test.go) | ✅ |
| FR-I-02 | document_parser 配置块 | [config.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/config/config.go) | [config_rag_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/config/config_rag_test.go) | ✅ |

**通过率**：35 条 FR，**31 条 ✅，4 条 ⚠️（属于"需补充验证"，不影响主流程）**。

### 5.2 非功能性需求

| 编号 | 要求 | 结论 | 备注 |
|---|---|---|---|
| NFR-01 可观测 | metrics.ObserveIngest 上报 | ✅ | consumer.go 中按 status + error_type 维度上报 |
| NFR-02 可审计 | sidecar 完整性 | ✅ | raw + canonical + canonicalization 元数据全保留 |
| NFR-03 可重放 | canonical 确定性 | ✅ | sha1(raw) / sha1(canonical) 双 hash 对比 |
| NFR-04 资源安全 | 20MB 用户上限 + 50MB provider 上限 | ✅ | 两层限制 |
| NFR-05 健壮性 | 表格/UTF-8 边界保护 | ✅ | 多处常量上限 + 单测保护 |
| NFR-06 兼容性 | 不破坏存量 txt/md 入库 | ✅ | local.go 路径独立 |
| NFR-07 可配置 | endpoint/timeout/sidecar/pdf_backend/ocr 全走配置 | ✅ | docker-compose env 已暴露 |

---

## 六、风险清单（按 P0/P1/P2 分级）

### P0（必须修才能合并）

| 风险 | 证据 | 建议处理 |
|---|---|---|
| **历史 commit 引入过二进制产物** | `git diff main...wubo --stat` 显示 `backend/main`、`backend/rag-server.exe` 曾被提交（虽然分支顶端已删除，但历史 blob 仍在 git pack 内，会拖慢 clone 并暴露内部信息） | 合并前确认 `.gitignore` 已覆盖；如果仓库历史敏感，考虑用 `git filter-repo` 清掉历史 blob。**最小要求**：合并 PR 前在 PR 描述里明确"已清理"并提供证据 |

### P1（建议修，作为合入条件）

| 风险 | 证据 | 建议处理 |
|---|---|---|
| **缺端到端冒烟用例** | 当前所有验证都在 mock 层，未跑过 docker-compose 起的真实 docling-serve | 至少跑一次：上传 1 PDF（含表格）+ 1 DOCX + 1 HTML（带 lead-in），人工核对 sidecar 正确性 |
| **`config.yaml` 直接被提交** | wubo 改动包含 [config.yaml](file:///h:/GYT-CODE/rag-retrievalOps/backend/config.yaml)（不是 example） | 检查是否含敏感配置；如无应改用 `config.example.yaml` |
| **docling-serve 未配置健康检查与重连策略** | docker-compose.yml 中 `docling-serve` 没有 healthcheck；parser-provider 用 120s 默认 timeout，但缺重试 | 给 docling-serve 加 `/healthz` 探针；parser-provider 增加 1-2 次幂等重试或熔断 |
| **`backend/cmd/parser-provider/main.go` 是新增二进制入口，缺少集成测试** | server.go 有部分测试但 main 路径未在 CI 中拉起 | 加一个 `cmd/parser-provider/integration_test.go`（用 httptest 起 server + mock docling） |

### P2（可改进，下一迭代）

| 风险 / 改进点 | 建议 |
|---|---|
| `blocks` 字段在默认 Docling 路径下基本为空 | 后续从 Docling JSON 抽取 paragraph/heading blocks，让 structure-aware 真正生效 |
| canonical 规则 `version=canonical-normalizer-v1`，没有版本演进策略 | 引入规则版本表 + 文档化变更日志，便于将来"重处理"决策 |
| OCR 阈值 `0.8` 是硬编码 | 提到配置层，方便不同知识库调优 |
| 多模态扩展点 `omitted_images` 只有占位 | 明确路线图：何时引入图像理解（CLIP / VL）、是否做图片向量化 |
| 每条 canonical 规则有 helper，但缺少"规则禁用"开关 | 后续支持按 KB / 按文档关闭某条规则，便于灰度 |
| 错误分类 `parse_error` 默认不重试，但有些 docling-serve 临时性错误（cold start）会被永久标记失败 | 区分"内容错误"与"服务错误"，后者应允许 1 次重试 |

---

## 七、验收结论与下一步

### 7.1 结论

| 维度 | 评分 | 说明 |
|---|---|---|
| 架构设计 | A | 三层解耦（rag-server / parser-provider / docling-serve）、领域模型清晰、可审计 |
| 代码质量 | A- | 单测充足，关键路径有边界保护；但 main 入口缺集成测试 |
| 可观测性 | B+ | 指标完整，但分布式追踪未接入 |
| 安全性 | B | 无明显敏感信息泄露；但配置文件直提交需复核 |
| 工程规范 | B- | git 历史曾存二进制，需清理证据 |
| **总评** | **B+ / 有条件通过** | 修完 P0 + 至少 1 项 P1（端到端冒烟），即可合入 main |

### 7.2 下一步动作

1. **立即处理**（P0）：
   - 验证 git 历史已无 `backend/main` 与 `backend/rag-server.exe` blob，或在 PR 描述里明确说明已清理。
2. **合入 main 前必须做**（P1）：
   - 跑一次 `docker compose --profile parser up -d docling-serve parser-provider`，上传 3 类典型文件（PDF 含表格 / DOCX / HTML 含 lead-in），核对 sidecar 与 chunk metadata 正确。
   - 复核 `config.yaml` 是否含敏感配置，如有则恢复为 example。
3. **下一迭代**（P2）：
   - blocks 字段补全 + structure-aware 真实生效
   - canonical 规则版本治理 + 反例单测
   - OCR 阈值 / 重试策略配置化
   - parser-provider 集成测试

---

## 八、文档版本

| 版本 | 日期 | 作者 | 变更 |
|---|---|---|---|
| v1.0 | 2026-06-17 | gaoerJJ（AI 协助） | 初版，含技术栈梳理 + docling-serve 专项说明 |

---

> **附**：本验收报告基于 [00-wubo分支-富文档解析与切分链路-反推需求文档.md](file:///h:/GYT-CODE/rag-retrievalOps/gaoerDocs/00-wubo分支-富文档解析与切分链路-反推需求文档.md) 中的 35 条 FR + 7 条 NFR 逐项核对得出。所有"✅"项已在本次验收中通过单元测试或代码走读确认；"⚠️"项不影响主流程但建议补强。
