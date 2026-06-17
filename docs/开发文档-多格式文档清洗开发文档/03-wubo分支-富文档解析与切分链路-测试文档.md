# wubo 分支 · 富文档解析与切分链路 · 测试文档

> **配套文档**：
>
> - [00-wubo分支-富文档解析与切分链路-反推需求文档.md](file:///h:/GYT-CODE/rag-retrievalOps/gaoerDocs/00-wubo分支-富文档解析与切分链路-反推需求文档.md)
> - [02-wubo分支-富文档解析与切分链路-验收报告.md](file:///h:/GYT-CODE/rag-retrievalOps/gaoerDocs/02-wubo分支-富文档解析与切分链路-验收报告.md)
>
> **文档类型**：可执行测试计划，分「AI 自动化执行」与「人工手动验证」两部分。

***

## 前置：P0（git 历史二进制）已确认清理 ✅

### 执行事实（已验证）

| <br />        | <br />                                             | 检查项                  | 命令                | 结果                                                                                               |
| :------------ | :------------------------------------------------- | -------------------- | ----------------- | ------------------------------------------------------------------------------------------------ |
| 分支顶端是否还有裸二进制？ | \`git ls-tree -r wubo backend/                     | Select-String "main  | exe"\`            | ❌ 只看到各 cmd 子目录下的**源码** **`main.go`**（正常 Go 入口），`backend/main` 与 `backend/rag-server.exe` 均不存在于顶端 |
| 历史是否曾引入？      | \`git log wubo --name-only --pretty=format:"%h %s" | Select-String "main$ | rag-server.exe"\` | ✅ `83d7b538`（RAG 多模态 Phase0）添加过，`f1f2749`（"修改readme 删除没用的文件"）已删除                                 |

**结论**：wubo 分支顶端已无二进制产物残留；git pack 中可能还有历史 blob，但这是整个仓库的历史问题，不是 wubo 分支引入的新问题（main 分支也有同样历史）。合并时可在 PR 描述中注明"wubo 分支已清理，历史 blob 由仓库统一治理"即可。**本 P0 风险降级，不再阻塞合并**。

***

## 第一部分：AI 自动化执行测试

> 以下命令你直接复制粘贴给我执行即可，我会输出结果。

### T1. 核心模块单元测试（必跑）

**覆盖范围**：documentparser、canonical、parserprovider、chunking、ragqueue、retrieval

```powershell
# Windows PowerShell 执行
cd h:\GYT-CODE\rag-retrievalOps\backend
go test ./internal/documentparser/... ./internal/documentparser/canonical/... ./internal/parserprovider/... ./internal/milvus/chunking/... ./internal/ragqueue/... ./internal/milvus/retrieval/... -count=1 -timeout 180s -v 2>&1 | Tee-Object -FilePath ..\gaoerDocs\test-results-core-modules.log
```

**验收标准**：所有包 `ok`，无 FAIL / panic / timeout。

***

### T2. 静态检查（必跑）

**覆盖范围**：go vet + go fmt 检查

```powershell
cd h:\GYT-CODE\rag-retrievalOps\backend

# 2.1 go vet
go vet ./internal/documentparser/... ./internal/parserprovider/... ./internal/milvus/chunking/... ./internal/ragqueue/... ./internal/milvus/retrieval/... 2>&1 | Tee-Object -FilePath ..\gaoerDocs\test-results-govet.log

# 2.2 go fmt 检查（只看不自动改）
$formatFiles = go fmt -n ./internal/documentparser/... ./internal/parserprovider/... ./internal/milvus/chunking/... ./internal/ragqueue/... ./internal/milvus/retrieval/... 2>&1
if ($formatFiles) { Write-Output "需要格式化的文件: $formatFiles" | Tee-Object ..\gaoerDocs\test-results-gofmt.log } else { Write-Output "所有文件格式正确" | Tee-Object ..\gaoerDocs\test-results-gofmt.log }
```

**验收标准**：go vet 无输出（= 无告警）；go fmt -n 无输出（= 所有文件已格式化）。

***

### T3. 构建可执行性检查（必跑）

**覆盖范围**：确保 parser-provider 新入口能编译通过

```powershell
cd h:\GYT-CODE\rag-retrievalOps\backend

# 3.1 编译 parser-provider（只编译不运行）
go build -o parser-provider-test.exe ./cmd/parser-provider/main.go

# 3.2 检查生成的可执行文件存在
if (Test-Path parser-provider-test.exe) { Write-Output "parser-provider 编译成功" | Tee-Object ..\gaoerDocs\test-results-build.log } else { Write-Output "编译失败" | Tee-Object ..\gaoerDocs\test-results-build.log }

# 3.3 清理临时产物
Remove-Item parser-provider-test.exe -ErrorAction SilentlyContinue
```

**验收标准**：编译成功无错误。

***

### T4. 测试覆盖率统计（可选，用于质量评估）

```powershell
cd h:\GYT-CODE\rag-retrievalOps\backend
go test ./internal/documentparser/... ./internal/parserprovider/... ./internal/milvus/chunking/... -coverprofile=..\gaoerDocs\coverage-core.out -covermode=count -timeout 120s
go tool cover -func=..\gaoerDocs\coverage-core.out | Tee-Object ..\gaoerDocs\test-results-coverage.log
```

**验收标准**：核心包平均覆盖率 ≥ 60%（如低于则建议补单测，但不阻塞合并）。

***

## 第二部分：人工手动验证（需你执行）

> 以下测试需要你手动操作或观察，按优先级标记 🔴 必做 / 🟡 建议做。

***

### S1. Docker Compose 解析栈启动（🔴 必做）

**目的**：验证 docling-serve 与 parser-provider 两个容器能正常拉起、健康检查通过。

| 步骤 | 操作                                                                                                                    | 预期结果                                                    |
| -- | --------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| 1  | 打开 PowerShell，执行：`cd h:\GYT-CODE\rag-retrievalOpsdocker compose --profile parser up -d docling-serve parser-provider` | 两行 `Container ... Started`                              |
| 2  | 等待 30 秒后执行：`docker compose ps`                                                                                        | `docling-serve`、`parser-provider` 状态均为 `Up` / `healthy` |
| 3  | 浏览器访问：`http://localhost:9000/healthz`                                                                                 | HTTP 200，返回类似 `{"status":"ok"}` 或简单 OK 字符串              |
| 4  | 浏览器访问：`http://localhost:5001/ui`                                                                                      | Docling 可视化 UI 正常显示，无 404 / 502                         |

**如果失败**：查看日志 `docker compose logs parser-provider` / `docker compose logs docling-serve`，截图发给我定位。

***

### S2. 端到端冒烟：PDF（含表格）解析（🔴 必做）

**目的**：验证 Docling → parser-provider → canonical → sidecar 全链路贯通。

**前置**：S1 已成功、docling-serve 可访问。

**建议测试文件**：找一个真实业务 PDF（最好带表格，如产品报价单、技术参数表、费用清单）。

| 步骤 | 操作                                                                                                                                                                                                       | 预期结果                                           |
| -- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| 1  | 打开 Admin UI `http://localhost:3003`，进入某知识库，上传测试 PDF                                                                                                                                                      | 上传成功，创建 ingest job                             |
| 2  | 等待约 30\~60 秒（视文件大小），刷新知识库                                                                                                                                                                                | document 状态变为 `completed`，chunk\_count 有值（> 0） |
| 3  | 到服务器 backend 工作目录查找 sidecar：`ls -la *.normalized.json` 或 dir `backend\*.normalized.json`                                                                                                                 | 存在一份 `{document_id}.normalized.json`           |
| 4  | 用编辑器打开 sidecar：- 检查 `content_markdown_raw` 是否非空- 检查 `content_markdown` 是否非空- 检查 `canonicalization` 是否存在（含 version、rules\_applied、warnings、raw\_sha1、canonical\_sha1）- 检查 `tables` 数组长度是否与 PDF 中的表格数量大致吻合 | 四项全部非空 / 有值                                    |
| 5  | 在 Admin UI 的"检索实验室"中输入一个明确在 PDF 中存在的问题（如"XX 产品单价多少"）                                                                                                                                                     | 检索结果能命中，答案片段可见来源表格的单元格                         |

***

### S3. 端到端冒烟：DOCX / HTML 各一份（🟡 建议做）

**目的**：验证除 PDF 外其他格式也能走通。

| 步骤 | 操作                                                      | 预期结果                                                                        |
| -- | ------------------------------------------------------- | --------------------------------------------------------------------------- |
| 1  | 上传一份 DOCX（含标题层级 + 段落）                                   | 状态 completed、sidecar 存在、markdown 结构清晰                                       |
| 2  | 上传一份 HTML（含 `<h1>~<h3>` + 正文段落，最好 lead-in 不在 h1 之后立即开始） | 状态 completed；检查 sidecar：HTML lead-in 修复生效（heading 前的前置段落完整保留在 canonical 开头） |

***

### S4. 切分策略路由验证（🟡 建议做）

**目的**：验证 4 条策略按优先级正确触发。

**验证方法**：在 S2/S3 完成后，检查 sidecar 对应的 chunk metadata（需要你去数据库查或加一行调试输出）。

| 文件类型                     | 预期策略              | 预期 metadata 字段                                                                            |
| ------------------------ | ----------------- | ----------------------------------------------------------------------------------------- |
| PDF 含表格                  | `table-aware`     | `chunking_route=table-aware`、`chunking_strategy=table-aware`、`table_ids` 非空               |
| PDF 扫描件 / OCR 识别         | `ocr-aware`       | `chunking_route=ocr-aware`、`weak_evidence=true`、`weak_evidence_reason=low_ocr_confidence` |
| PDF 纯文本无表格               | `structure-aware` | `chunking_route=structure-aware`、`block_ids` 非空                                           |
| Markdown / TXT（无 blocks） | `markdown`        | `chunking_route=markdown`、`section_title` 非空、`heading_level` 有值                           |

***

### S5. 哈希查重 reused=true 验证（🟡 建议做）

**目的**：验证同一文件重复上传不会重复解析。

| 步骤 | 操作                      | 预期结果                                       |
| -- | ----------------------- | ------------------------------------------ |
| 1  | 上传 S2 测试用过的同一份 PDF 第二次  | 接口立即返回成功，不等待 30 秒                          |
| 2  | 查看返回 JSON               | `reused=true` 字段为 true，document\_id 与第一次相同 |
| 3  | 去 backend 工作目录看 sidecar | 没有生成新的 normalized.json                     |

***

### S6. 失败路径验证：故意停掉 docling-serve（🟡 建议做）

**目的**：验证错误降级、错误 sidecar、状态机一致性。

| 步骤 | 操作                                                    | 预期结果                                                  |
| -- | ----------------------------------------------------- | ----------------------------------------------------- |
| 1  | 执行：`docker compose stop docling-serve`                | docling-serve 变为 `Exited`                             |
| 2  | 上传一份新 PDF                                             | job 状态变为 `failed` 或 `retrying`（取决于错误分类）               |
| 3  | 去 backend 工作目录查找：`ls -la *.normalized.error.json`     | 存在一份 error sidecar，内容含 `error_code`、`message`、`stage` |
| 4  | 重启 docling-serve：`docker compose start docling-serve` | 服务恢复，retrying 状态的 job 应被补偿扫描器重新执行                     |

***

### S7. 父子 chunk 检索验证（🟡 建议做）

**目的**：验证 finalizeChunks 注入的 parent-child 元数据在检索侧正确使用。

| 步骤 | 操作                    | 预期结果                                                                   |
| -- | --------------------- | ---------------------------------------------------------------------- |
| 1  | 在检索实验室输入精确查询（如"计费公式"） | 检索结果包含精确答案片段                                                           |
| 2  | 查看返回的 knowledge 元数据   | `parent_build_version=phase3-parent-child-v1` 字段存在；有 child / parent 偏移 |
| 3  | 检查 topk 排序            | 同权重下，精确证据优先于模糊证据                                                       |

***

### S8. 多租户隔离验证（🔴 必做 —— 安全红线）

**目的**：确保新代码不会绕过你做的多租户隔离。

| 步骤 | 操作                                         | 预期结果                                         |
| -- | ------------------------------------------ | -------------------------------------------- |
| 1  | 用租户 A 的 API Key 上传一份 PDF，记下 document\_id   | 成功                                           |
| 2  | 用租户 B 的 API Key，调用 retrieve 查那个 PDF 里的精确内容 | 检索结果为空（查不到）或返回 403（取决于权限控制粒度），绝对不能返回租户 A 的内容 |

***

### S9. 配置兼容验证（🟡 建议做）

**目的**：验证 `document_parser` 配置块生效。

| 步骤 | 操作                                                          | 预期结果                                   |
| -- | ----------------------------------------------------------- | -------------------------------------- |
| 1  | 临时把 `DOCUMENT_PARSER_TIMEOUT_MS` 改成 1000（1 秒），重启 rag-server | 服务正常启动                                 |
| 2  | 上传一份较大 PDF（> 5 页）                                           | 超时错误（说明配置生效了）；或虽然成功但日志里能看到超时配置是 1000ms |
| 3  | 改回 60000                                                    | 恢复正常                                   |

***

## 第三部分：测试结果记录表

请你（或我执行自动化测试后）把结果填入下表，然后把更新后的文档提交到 gaoerDocs/。

| 测试编号 | 测试名称                 | 执行者    | 执行时间       | 结果（✅/❌/⚠️） | 备注                                                                                                                   |
| ---- | -------------------- | ------ | ---------- | ---------- | -------------------------------------------------------------------------------------------------------------------- |
| T1   | 核心模块单元测试             | AI 自动化 | 2026-06-17 | ✅          | 6 个包全绿：documentparser(0.30s)、canonical(0.26s)、parserprovider(0.30s)、chunking(0.27s)、ragqueue(0.35s)、retrieval(0.42s) |
| T2   | go vet + go fmt 静态检查 | AI 自动化 | 2026-06-17 | ✅          | go vet 零告警                                                                                                           |
| T3   | parser-provider 构建检查 | AI 自动化 | 2026-06-17 | ✅          | 编译成功无错误                                                                                                              |
| T4   | 覆盖率统计                | AI 自动化 | 2026-06-17 | ✅          | 平均 79.1%（documentparser 82.6%、canonical 90.4%、parserprovider 74.2%、chunking 69.1%），远超 60% 标准                         |
| S1   | 解析栈 Docker 启动        | <br /> | <br />     | <br />     | <br />                                                                                                               |
| S2   | PDF（含表格）端到端          | <br /> | <br />     | <br />     | <br />                                                                                                               |
| S3   | DOCX + HTML 端到端      | <br /> | <br />     | <br />     | <br />                                                                                                               |
| S4   | 切分策略路由               | <br /> | <br />     | <br />     | <br />                                                                                                               |
| S5   | 哈希查重 reused          | <br /> | <br />     | <br />     | <br />                                                                                                               |
| S6   | 失败路径 + error sidecar | <br /> | <br />     | <br />     | <br />                                                                                                               |
| S7   | 父子 chunk 检索          | <br /> | <br />     | <br />     | <br />                                                                                                               |
| S8   | 多租户隔离（安全）            | <br /> | <br />     | <br />     | <br />                                                                                                               |
| S9   | 配置兼容验证               | <br /> | <br />     | <br />     | <br />                                                                                                               |

***

# 0617验收记录：

自动化测试 T1\~T4 全部执行完成并写入文档。

## 执行结果汇总

测试 结果 关键数据 T1 单元测试 ✅ 6 个核心包全绿，单测最快 0.26s、最慢 0.42s T2 go vet ✅ 零告警 ，静态代码质量优秀 T3 构建检查 ✅ parser-provider 编译成功 T4 覆盖率统计 ✅ 平均 79.1% （documentparser 82.6%、 canonical 90.4% 、parserprovider 74.2%、chunking 69.1%）， 远超 60% 企业标准

## 下一步

自动化测试（T1\~T4）已全部通过 ✅

现在只剩下 3 项必做的手动验证（S1、S2、S8） ，你可以选择：

**方案 1**： 你自己做手动验证（S1 启动 Docker 解析栈 → S2 上传 PDF 跑通全链路 → S8 做多租户隔离检查），做完告诉我结果，我帮你更新文档并输出最终验收结论。

**方案 2**： 直接把这三份文档（反推需求 + 验收报告 + 测试文档）发给 wubo，让他按测试文档自行跑完 S1\~S9 自测，自测通过后再找你走合并评审。

建议选方案 2 ：自动化已经帮你把"硬骨头"啃完了，剩下的手动验证让开发者自己跑一次，对他也是一次全链路熟悉过程，你只需要最后复核结果即可。

# 验收通过标准

**自动化测试全部通过（T1\~T3） + 手动验证必做项全部通过（S1、S2、S8）** 即可确认 wubo 分支满足企业级合入标准。

**建议项（S3\~S7、S9）** 用于提升质量信心，不阻塞合并，但如发现问题应记录进后续迭代。

***

## 文档版本

| 版本   | 日期         | 作者             | 变更                                          |
| ---- | ---------- | -------------- | ------------------------------------------- |
| v1.0 | 2026-06-17 | gaoerJJ（AI 协助） | 初版，含 P0 已清理确认 + 自动化测试命令 + 9 项手动验证步骤 + 结果记录表 |

