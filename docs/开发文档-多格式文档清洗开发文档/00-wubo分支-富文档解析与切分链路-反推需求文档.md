# wubo 分支 · 富文档解析与切分链路 · 反推需求文档

> **文档性质**：在原始 PRD 缺失的情况下，依据 `git log wubo --not main --no-merges --author=profoundwu` 的提交序列、`docs/知识库文件归一化流程讲解稿.md`（wubo 自述）、以及代码 + 测试用例反向重建的「事后需求文档」。
>
> **用途**：作为后续企业级验收（红线 / 黄线检查清单、验收报告）的基准。
>
> **生效范围**：仅覆盖 wubo（profoundwu）本人在 wubo 分支上提交的代码与文档。MCP Server、面试题文档、其他成员合并进来的内容**不在本文档范围内**。

---

## 1. 验收范围基线（净文件清单）

通过命令：

```powershell
git log wubo --not main --no-merges --author="profoundwu" --name-only --pretty=format: | Sort-Object -Unique
```

得到 wubo 真正动过的文件清单，按模块分组（仅列代码 / 配置 / 工程文件，文档类不作为代码验收对象，已剔除）：

### 1.1 文档解析与归一化（核心 1：parser + canonical）
- [backend/internal/documentparser/types.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/types.go)、[types_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/types_test.go)
- [backend/internal/documentparser/local.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/local.go)、[local_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/local_test.go)
- [backend/internal/documentparser/http_provider.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/http_provider.go)、[http_provider_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/http_provider_test.go)
- [backend/internal/documentparser/provider.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/provider.go)、[provider_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/provider_test.go)
- [backend/internal/documentparser/sidecar.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/sidecar.go)、[sidecar_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/sidecar_test.go)
- [backend/internal/documentparser/provenance.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/provenance.go)、[provenance_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/provenance_test.go)
- [backend/internal/documentparser/html_leadin.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/html_leadin.go)
- [backend/internal/documentparser/canonical/normalizer.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/canonical/normalizer.go)、[normalizer_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/canonical/normalizer_test.go)

### 1.2 外部解析适配层（核心 2：parser-provider + Docling）
- [backend/cmd/parser-provider/main.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/cmd/parser-provider/main.go)
- [backend/internal/parserprovider/server.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/parserprovider/server.go)
- [backend/internal/parserprovider/docling.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/parserprovider/docling.go)、[docling_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/parserprovider/docling_test.go)
- [backend/Dockerfile.parser-provider](file:///h:/GYT-CODE/rag-retrievalOps/backend/Dockerfile.parser-provider)
- [backend/scripts/start-docling-parser-stack.sh](file:///h:/GYT-CODE/rag-retrievalOps/backend/scripts/start-docling-parser-stack.sh)
- [backend/scripts/start-docling-serve.sh](file:///h:/GYT-CODE/rag-retrievalOps/backend/scripts/start-docling-serve.sh)
- [docker-compose.yml](file:///h:/GYT-CODE/rag-retrievalOps/docker-compose.yml)（仅 docling / parser-provider 相关 service）

### 1.4 切分策略路由与实现（核心 3：chunking）
- [backend/internal/milvus/chunking/strategy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/strategy.go)
- [backend/internal/milvus/chunking/router.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/router.go)、[router_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/router_test.go)
- [backend/internal/milvus/chunking/helpers.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/helpers.go)
- [backend/internal/milvus/chunking/markdown_strategy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/markdown_strategy.go)、[markdown_strategy_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/markdown_strategy_test.go)
- [backend/internal/milvus/chunking/structure_strategy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/structure_strategy.go)、[structure_strategy_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/structure_strategy_test.go)
- [backend/internal/milvus/chunking/table_strategy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/table_strategy.go)、[table_strategy_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/table_strategy_test.go)
- [backend/internal/milvus/chunking/ocr_strategy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/ocr_strategy.go)、[ocr_strategy_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/chunking/ocr_strategy_test.go)

### 1.4 入库链路接入（核心 4：ingest 联动）
- [backend/internal/ragqueue/consumer.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/ragqueue/consumer.go)
- [backend/internal/ragqueue/consumer_parser_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/ragqueue/consumer_parser_test.go)
- [backend/internal/milvus/init.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/init.go)、[init_chunking_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/init_chunking_test.go)
- [backend/internal/milvus/storage/embedding.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/storage/embedding.go)、[embedding_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/storage/embedding_test.go)（含 ark api type 兼容）
- [backend/api/handler/kb/handler.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler.go)（上传校验扩展）
- [backend/api/handler/kb/handler_upload_validation_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler_upload_validation_test.go)
- [backend/api/handler/kb/knowledge_base_binding.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/knowledge_base_binding.go)
- [backend/api/handler/kb/handler_eval_dataset.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler_eval_dataset.go)、[handler_eval_dataset_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler_eval_dataset_test.go)
- [backend/api/handler/kb/handler_l4_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler_l4_test.go)

### 1.6 检索侧适配修复（核心 5：retrieval 与切分一致性的自闭环修复）
- [backend/internal/milvus/retrieval/dedupe.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/dedupe.go)（去重表格行重复）
- [backend/internal/milvus/retrieval/hybrid_search.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/hybrid_search.go)
- [backend/internal/milvus/retrieval/parent_child.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/parent_child.go)、[parent_child_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/parent_child_test.go)
- [backend/internal/milvus/retrieval/reranker.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/reranker.go)、[reranker_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/reranker_test.go)
- [backend/internal/milvus/retrieval/search.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/search.go)
- [backend/internal/milvus/retrieval/title_boost.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/title_boost.go)、[title_boost_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/title_boost_test.go)
- [backend/internal/milvus/retrieval/topk_policy.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/topk_policy.go)、[topk_policy_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/topk_policy_test.go)
- [backend/internal/milvus/retrieval/fusion_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/retrieval/fusion_test.go)

### 1.6 配置 / 工程基线
- [backend/internal/config/config.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/config/config.go)、[config_rag_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/config/config_rag_test.go)
- [backend/config.example.yaml](file:///h:/GYT-CODE/rag-retrievalOps/backend/config.example.yaml)、[config.rag.example.yaml](file:///h:/GYT-CODE/rag-retrievalOps/backend/config.rag.example.yaml)、[config.yaml](file:///h:/GYT-CODE/rag-retrievalOps/backend/config.yaml)
- [backend/go.mod](file:///h:/GYT-CODE/rag-retrievalOps/backend/go.mod)、[backend/go.sum](file:///h:/GYT-CODE/rag-retrievalOps/backend/go.sum)
- [admin/src/components/admin/knowledge-base-detail-page.tsx](file:///h:/GYT-CODE/rag-retrievalOps/admin/src/components/admin/knowledge-base-detail-page.tsx)、[admin/src/__tests__/knowledge-base-detail-page.test.tsx](file:///h:/GYT-CODE/rag-retrievalOps/admin/src/__tests__/knowledge-base-detail-page.test.tsx)（前端上传扩展格式适配）
- [.env.example](file:///h:/GYT-CODE/rag-retrievalOps/.env.example)、[.gitignore](file:///h:/GYT-CODE/rag-retrievalOps/.gitignore)、[README.md](file:///h:/GYT-CODE/rag-retrievalOps/README.md)
- [backend/internal/milvus/benchmark/resource_usage_unix.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/milvus/benchmark/resource_usage_unix.go)（仅基线测试稳定化）

> **不在本范围**：`backend/cmd/rag-mcp-server/`、`backend/internal/mcp/`、`backend/Dockerfile.mcp`、`docs/开发文档-MCP功能开发文档/*`、`docs/面试题和参考答案/*` 等所有非 profoundwu 提交。

---

## 2. 业务目标（一句话）

> 让 RAG 平台支持 PDF / Word / HTML / Markdown / 纯文本等**多格式知识源**，统一收敛成可审计、可溯源、可结构化切分的标准文档模型 `NormalizedDocument`，使后续切分、向量化、检索阶段**只面对一份稳定正文**，并能在排障时还原任何中间产物。

---

## 3. 用户角色与场景

| 角色 | 场景 |
|---|---|
| 知识运营人员 | 上传 PDF/Word/HTML/Markdown 形式的业务知识到 RAG 平台 |
| RAG 检索调用方 | 期望同一份知识不论以何种格式入库，检索结果稳定、引用可信 |
| 平台运维 / 排障人员 | 出现"检索答非所问 / 表格乱"时，能通过 sidecar 还原解析过程 |
| 平台开发 | 解析器、归一化规则、切分策略升级时能做版本对比 |

---

## 4. 功能性需求（FR）

### FR-A 上传入口扩展

| 编号 | 描述 | 验收点 | 代码落点 |
|---|---|---|---|
| FR-A-01 | 上传支持的文件类型扩展为 `pdf / txt / md / markdown / docx / html / htm` | 上传校验通过 + 不支持类型（如 xlsx）被拒绝 | [handler.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler.go)、[handler_upload_validation_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler_upload_validation_test.go) |
| FR-A-02 | 单文件大小限制 20MB，空文件拒绝 | 超限 / 空文件返回明确错误 | 同上 |
| FR-A-03 | 同 KB 内按 sha256 文件哈希查重，命中返回已有 document/job 并标 `reused=true` | 重复上传同一文件不重复解析 | [handler.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/api/handler/kb/handler.go) |
| FR-A-04 | 前端 Admin 知识库页面同步支持新格式 | 前端可见 + 单测覆盖 | [knowledge-base-detail-page.tsx](file:///h:/GYT-CODE/rag-retrievalOps/admin/src/components/admin/knowledge-base-detail-page.tsx) |

### FR-B 文档归一化（NormalizedDocument 模型）

| 编号 | 描述 | 验收点 |
|---|---|---|
| FR-B-01 | 定义统一领域模型 `NormalizedDocument`，字段含 `content_markdown / content_markdown_raw / source / blocks / tables / omitted_images / quality / extractor / canonicalization` | 模型字段完整、`Validate()` 拒绝缺字段文档 |
| FR-B-02 | `txt / md / markdown` 走 `NormalizeLocal`：本地解析、无外部依赖 | 单测覆盖 quality.status=ok / extractor.provider=local |
| FR-B-03 | Markdown pipe table 规范化：识别表头、清理 `*/_/反引号`、生成 `tables` 元数据（含 ID、span、行列、质量） | 单测验证表格 span 与正文偏移一致 |
| FR-B-04 | `pdf / docx / html / htm` 走 `HTTPProvider`，向 `DOCUMENT_PARSER_ENDPOINT` 发 multipart 请求并解析返回 JSON 为 `NormalizedDocument` | provider 错误以 `ProviderError` 透出（含 code/stage/page/retryable） |
| FR-B-05 | HTML lead-in 修复：把第一个 heading 之前的前置段落（≤8 块、≤4096B）补回到 canonical 之前 | `RestoreHTMLLeadInBeforeFirstHeading` 单测 |

### FR-C Canonical Normalizer（确定性二次归一化）

| 编号 | 描述 | 验收点 |
|---|---|---|
| FR-C-01 | 在 sidecar 与 chunking 之前执行确定性 canonical 归一化，保留 raw 内容到 `content_markdown_raw` | 同一 raw 输入产生稳定 canonical 输出（hash 可比） |
| FR-C-02 | 已落地规则：unicode/换行归一、空行压缩、parser 噪声清理（`## ·`、`- ·`、`- -`）、标题强调清理、编号标题加空格、CJK 字间空格清除、确定性标题续接合并、表格 span 重算 | 单测覆盖每条规则 |
| FR-C-03 | 写入 `canonicalization` 元数据：`version / applied_rules / warnings / raw_sha1 / canonical_sha1` | 字段非空 |
| FR-C-04 | **保守边界**：不做 LLM 改写，不强行修改业务字符（如 `128MB` 不变成 `128 MB`） | 反例单测保护 |

### FR-D parser-provider Adapter（外置解析服务）

| 编号 | 描述 | 验收点 |
|---|---|---|
| FR-D-01 | 提供独立服务 `parser-provider`，对 RAG 后端暴露 `POST /parse` 与 `GET /healthz` | 服务可独立启动、健康探针返回 200 |
| FR-D-02 | 内部对接 Docling Serve：`POST /v1/convert/file`，请求 `to_formats=md & json`、`image_export_mode=placeholder` | 单测打桩 Docling HTTP |
| FR-D-03 | Docling 返回选择正文：优先 `md_content`，缺失时回退 `text_content` | 单测覆盖两种路径 |
| FR-D-04 | Docling Markdown pipe table 二次规范化（复用 `NormalizeMarkdownPipeTables`） | 单测：脏表格输入 → 规范化输出 |
| FR-D-05 | Docling JSON 中存在结构化 tables 但 Markdown 缺失时，渲染为 Markdown 追加到正文末尾 `## Extracted Tables` 并补 `tables` 元数据 | 单测覆盖 |
| FR-D-06 | PDF 表格 Markdown span 智能延展：识别紧随表格的延续行，但在新编号章节 / 同级 heading 处停止（限制：`maxDoclingFragmentedTableExtensionBytes=4096`） | 单测覆盖正反两种情况 |
| FR-D-07 | 表格安全上限：`maxRows=10000 / maxCols=200 / maxCells=50000`，超限不渲染 | 单测保护 |
| FR-D-08 | 配置项 PDF 默认 `pdf_backend=pypdfium2`，并把 `engine / strict_mode / ocr.*` 透传给 Docling | 配置生效 |
| FR-D-09 | 一键启动脚本 `start-docling-parser-stack.sh / start-docling-serve.sh` | 脚本可执行（Linux/Mac） |
| FR-D-10 | 提供 `Dockerfile.parser-provider` 与 docker-compose service 定义 | `docker compose up parser-provider` 能起 |

### FR-E Sidecar 与 Provenance（溯源 / 排障）

| 编号 | 描述 | 验收点 |
|---|---|---|
| FR-E-01 | 解析成功在源文件旁写 `*.normalized.json`（含 raw + canonical 全量） | 文件存在、JSON 合法、含 canonicalization 字段 |
| FR-E-02 | 解析失败写 `*.normalized.error.json`（`error_code / message / stage / page / quality`） | 失败路径不丢上下文 |
| FR-E-03 | chunk metadata 中带 `normalized_path`，可由检索结果反查 sidecar | metadata 字段非空 |
| FR-E-04 | provenance 模块为入库链路提供来源/溯源信息（解析器版本、canonical 版本等） | 单测覆盖 |

### FR-F Chunking 路由 + 多策略

| 编号 | 描述 | 验收点 |
|---|---|---|
| FR-F-01 | 路由优先级：`table-aware → ocr-aware → structure-aware → markdown` | router_test 覆盖 4 种命中 |
| FR-F-02 | 路由结果在 chunk metadata 写 `chunking_route`；策略实现写 `chunking_strategy / chunking_unit` | metadata 字段一致 |
| FR-F-03 | **table-aware**：`len(Tables)>0` 触发；先委托 structure 生成基础 chunk，再为每张表生成专门 chunk；带 `table_ids / table_row_count / table_quality_status / table_merged_cells / table_nested / page_start / page_end / child_*_offset` | 单测覆盖 |
| FR-F-04 | **ocr-aware**：`pdf` 且 quality.warnings 含 `ocr` 或 blocks 含 confidence 触发；委托 structure，再按 chunk 平均 confidence < 0.8 标 `weak_evidence=true / weak_evidence_reason=low_ocr_confidence` | 单测覆盖 |
| FR-F-05 | **structure-aware**：blocks 非空 + 文件类型为 pdf/docx/html/htm 触发；按 Markdown span 排序合并相邻块为窗口（默认 1000 字节）；带 `block_ids / block_types / page_* / ocr_confidence / child_*_offset` | 单测覆盖 |
| FR-F-06 | **markdown**（兜底）：用 goldmark 解析 heading 切章节；带 `section_title / hierarchy_path / heading_level / section_segment_*`；无 heading 回退递归 splitter | 单测覆盖 |
| FR-F-07 | 章节标题含 `计费公式` 时 `chunking_unit=formula` | 单测覆盖 |
| FR-F-08 | `finalizeChunks` 给所有 chunk 补齐 parent-child 元数据：`chunk_index / total_chunks / chunk_id / child_id / parent_id / child_*_offset / parent_*_offset / parent_token_count / parent_build_strategy / parent_build_version=phase3-parent-child-v1 / parent_child_available / section_title / hierarchy_path / normalized_path` | 单测覆盖 |
| FR-F-09 | UTF-8 边界安全 (`utf8Boundary` 等 helper)，避免切坏多字节字符 | 单测覆盖 |

### FR-G 入库链路接入（ingest 联动）

| 编号 | 描述 | 验收点 |
|---|---|---|
| FR-G-01 | `knowledge_ingest` 消费者按 `file_type` 分流：local / provider | consumer_parser_test |
| FR-G-02 | 解析后顺序：raw → canonical → sidecar → chunking → embedding → milvus 写入；任一步失败按错误类型分类（parse_error / embedding_error / milvus_write_error / unknown） | 状态机覆盖 |
| FR-G-03 | parse_error 默认不重试；embedding/milvus 错误含 timeout/connection refused/network 时进入 `retrying`，并由补偿扫描器重发；超限进入 `dead` | 单测覆盖 |
| FR-G-04 | 成功时更新文档 `chunk_count`、job 与 document 状态置 `completed`；失败保留状态机一致性 | 单测覆盖 |
| FR-G-05 | Milvus collection 解析：知识库已绑定 collection 时使用 `NewIndexerServiceForCollection`，否则用默认 collection | 集成测试 |
| FR-G-06 | base metadata 含 `tenant_id / operator_admin_id / kb_id / document_id / file_name / collection / normalized_path`，向 chunk 透传以保证多租户隔离 | metadata 完整 |

### FR-H 检索侧适配（与切分一致性的自闭环）

| 编号 | 描述 | 验收点 |
|---|---|---|
| FR-H-01 | 检索结果按 display score 排序（`fix(kb): sort retrieve results by display score`） | 单测覆盖 |
| FR-H-02 | 表格行重复去重（`fix(chunking): avoid duplicated table retrieval rows`） | dedupe_test |
| FR-H-03 | 父块填充结果归一化（`fix(retrieval): normalize parent filled results`） | parent_child_test |
| FR-H-04 | 精确查询结果收紧 + 偏好精确计费证据（`tighten precise query results` + `prefer precise billing evidence`） | 单测覆盖 |
| FR-H-05 | score cliff 使用 ranking score（`use ranking score for score cliff`） | topk_policy_test |
| FR-H-06 | title boost 增强 | title_boost_test |

### FR-I 配置兼容

| 编号 | 描述 | 验收点 |
|---|---|---|
| FR-I-01 | embedding 配置兼容 ark api type（`fix(embedding): support ark api type configuration`） | embedding_test |
| FR-I-02 | 新增 `document_parser` 配置块：endpoint、timeout、save_sidecar、provider options（engine / strict_mode / ocr.*） | config_rag_test |

---

## 5. 非功能性需求（NFR）

| 编号 | 类别 | 要求 |
|---|---|---|
| NFR-01 | 可观测 | ingest 链路保留 `metrics.ObserveIngest`，按 status + error_type 维度上报 |
| NFR-02 | 可审计 | 任何一份成功入库文档，都能通过 `*.normalized.json` 还原 raw 与 canonical；任何失败都能通过 `*.normalized.error.json` 看到 stage/page |
| NFR-03 | 可重放 | 同一 raw 输入 + 同一 canonical version → 相同 canonical_sha1（确定性） |
| NFR-04 | 资源安全 | parser-provider multipart 上限 50MB；上传入口先于 provider 命中 20MB |
| NFR-05 | 健壮性 | Docling 表格 span 计算限定 4096B 边界；表格行/列/单元上限保护；UTF-8 边界保护 |
| NFR-06 | 兼容性 | 新增解析路径不影响存量 txt/md 入库；旧文档无 canonicalization 字段时仍能正常检索 |
| NFR-07 | 可配置 | parser endpoint / timeout / sidecar 开关 / pdf_backend / OCR provider 全部走配置，无硬编码 |

---

## 6. 明确的"非目标"（防止验收范围蔓延）

1. **不包含 MCP Server**（`backend/cmd/rag-mcp-server/`、`backend/internal/mcp/`、所有 mcp 相关文档）—— 属于 gaoerjj 分支已合入 main 的内容。
2. **不包含面试题文档**、面试 Agent 对接（wangzhongyang 提交）。
3. **不做多模态入库**：图片在 Docling 中以 placeholder 模式处理，不进入文本向量库。
4. **canonical normalizer 不做语义改写**：不补业务知识、不替换术语。
5. **blocks 字段已就位但当前 Docling adapter 主要输出正文 + 表格**，不强求全部填充 blocks（仅作为后续扩展点）。
6. **不替换原 splitter / retrieval 主架构**，只做与新切分链路一致性所需的局部修复。

---

## 7. 接口与配置契约

### 7.1 对外契约（parser-provider）
- `POST /parse`：multipart/form-data，字段：`file`（必选）、`file_type`（必选）、`options`（可选 JSON）
  - 成功：HTTP 2xx，body 为 `NormalizedDocument` JSON
  - 失败：HTTP 4xx/5xx，body 为 `ProviderError`（`error_code / message / stage / page / retryable`）
- `GET /healthz`：HTTP 200 + 简单健康标识

### 7.2 对内契约（NormalizedDocument JSON Schema）
- 见 [types.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/documentparser/types.go)。任何新增字段需保持向后兼容（`omitempty`）。

### 7.3 配置契约（document_parser 配置块）
- `endpoint`、`timeout`、`save_sidecar`、`pdf_backend`、`ocr.{provider,endpoint,timeout_ms}`、`engine`、`strict_mode`
- 测试：[config_rag_test.go](file:///h:/GYT-CODE/rag-retrievalOps/backend/internal/config/config_rag_test.go)

### 7.4 chunk metadata 契约（关键字段）
- 路由：`chunking_route`、`chunking_strategy`、`chunking_unit`
- 父子：`chunk_index`、`total_chunks`、`chunk_id`、`child_id`、`parent_id`、`child_start_offset`、`child_end_offset`、`parent_start_offset`、`parent_end_offset`、`parent_token_count`、`parent_build_strategy`、`parent_build_version=phase3-parent-child-v1`、`parent_child_available`
- 章节：`section_title`、`hierarchy_path`、`heading_level`、`section_segment_index`、`section_segment_count`
- 表格：`table_ids`、`table_row_count`、`table_quality_status`、`table_merged_cells`、`table_nested`
- 页码：`page_start`、`page_end`
- OCR：`ocr_confidence`、`weak_evidence`、`weak_evidence_reason`
- 溯源：`normalized_path`

---

## 8. 端到端流程图（从讲解稿提炼）

```text
用户上传文件
   │
   ▼
上传校验（类型/大小/非空） ─→ 同 KB 哈希查重 ─→ 已存在则 reused=true 返回
   │
   ▼
保存源文件 + 创建 KBDocument & KBIngestJob ─→ 发布 knowledge_ingest 消息
   │
   ▼
异步消费者 claim job → 读取源文件
   │
   ├─ txt/md/markdown ──► NormalizeLocal
   └─ pdf/docx/html ────► HTTPProvider ──► parser-provider /parse
                                              └─► Docling /v1/convert/file
   │
   ▼
Raw NormalizedDocument
   │
   ▼
Canonical Normalize（保留 raw、生成 canonical hash）
   │
   ├─ 失败 ──► 写 *.normalized.error.json ──► job=failed/retrying/dead
   │
   ▼
写 *.normalized.json
   │
   ▼
Chunking Router ──► table-aware / ocr-aware / structure-aware / markdown
   │
   ▼
finalizeChunks（补 parent-child / section / normalized_path）
   │
   ▼
Embedding ──► Milvus 写入 ──► 更新 chunk_count / job=completed / document=completed
```

---

## 9. 验收清单建议（与本反推需求成对的清单文件，建议下一份产出）

下一份建议产出：`gaoerDocs/01-wubo分支-富文档解析与切分链路-企业级验收清单.md`，包含：

- **红线项（必须通过）**：FR-A、FR-B、FR-C、FR-F、FR-G 全过；NFR-04 资源安全；分支不得包含已删除二进制（main、rag-server.exe 历史问题需确认已清理）；多租户 metadata 不得为空。
- **黄线项（建议改进）**：blocks 字段在默认 Docling 路径下尚未完整填充（FR-F-05 弱化）；测试覆盖率 ≥ 70%；docling-serve 重连 / 限流 / 重试策略；canonical 规则版本号管理。
- **冒烟用例**：上传 7 类文件 × 是否含表格 / 含 OCR / 含 HTML lead-in 的 9 宫格；同名文件 reused 验证；故意停掉 docling-serve 验证错误 sidecar。

---

## 10. 文档版本

| 版本 | 日期 | 变更 | 作者 |
|---|---|---|---|
| v1.0 | 2026-06-17 | 初版反推（依据 wubo 分支 profoundwu 提交 + 讲解稿 + 代码/测试） | gaoerJJ（AI 协助） |

---

> **下一步动作建议**：
> 1. 把本文档发给 wubo 本人 review 一轮，让他确认"反推 = 实做"，不一致处当场对齐。
> 2. 基于本文档产出 **`01-…-企业级验收清单.md`**（红线 / 黄线 / 冒烟用例可勾选）。
> 3. 再据此执行验收，输出 **`02-…-验收报告.md`**。
