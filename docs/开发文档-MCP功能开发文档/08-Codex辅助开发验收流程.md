# Codex 辅助开发验收流程

> 适用场景：本流程用于验收借助 Codex 完成的代码改动，尤其适用于本次 `RAG 平台 MCP Server 多租户接入合并设计方案` 中 Phase 0 - Phase 4 的实现交付。
>
> 核心原则：Codex 已完成的“基础验收”只能作为第一层自检，最终是否可合并应以需求契约、代码 Diff、自动化测试、安全边界、部署验证和人工 Review 的结果为准。

---

## 1. 本次改动背景

本次开发目标来自《RAG平台MCP Server多租户接入合并设计方案.md》的 Phase 0 - Phase 4：

- Phase 0：契约冻结与技术验证
- Phase 1：stdio 最小可用版本
- Phase 2：Streamable HTTP 多租户版本
- Phase 3：审计、限流与监控加固
- Phase 4：Docker Compose 与交付文档

当前 Git 未提交文件即本轮 Codex 辅助开发的主要改动范围，涉及：

- MCP Server 启动入口
- MCP 配置加载与环境变量
- `retrieve_knowledge` Tool Handler
- RAG Server `/v1/retrieve` 转发客户端
- HTTP transport、健康检查、ready 检查
- Origin 校验、token 脱敏等安全能力
- metrics 指标
- Dockerfile、Docker Compose 服务配置
- README / Runbook 文档
- 单元测试、安全测试、HTTP transport 测试

因此，本轮验收不应只关注“代码能否编译”，还需要重点验证：

1. MCP 协议链路是否可用；
2. 多租户鉴权边界是否正确；
3. token、API Key 等敏感信息是否不会泄露；
4. MCP Server 是否不会影响原有 `/v1/retrieve`；
5. Docker Compose 交付链路是否可启动、可健康检查、可回滚。

---

## 2. 推荐验收总流程

建议按以下顺序执行：

```text
需求契约复核
  ↓
Git Diff 范围审查
  ↓
Codex 二次自审
  ↓
本地静态检查与单元测试
  ↓
MCP 协议级功能验证
  ↓
多租户与安全边界验证
  ↓
Docker Compose 部署验证
  ↓
原有 RAG 能力回归验证
  ↓
人工 Review 与合并判断
```

不要跳过前面的契约和 Diff 审查。AI 生成代码最常见的问题不是“不能运行”，而是“看起来合理，但偏离了设计约束”。

---

## 3. 第一步：需求契约复核

先对照设计文档确认本次是否只实现 Phase 0 - Phase 4，不提前引入 Phase 5 / V2 能力。

### 3.1 必须满足的能力

| 阶段 | 必须验收的能力 |
|---|---|
| Phase 0 | Tool schema、错误语义、transport 选择已冻结，不通过 tool 参数传 API Key |
| Phase 1 | stdio 模式可注册并调用 `retrieve_knowledge`，stdout 不输出普通日志 |
| Phase 2 | HTTP `/mcp` 支持 initialize / tools/list / tools/call，多租户凭证来自 Authorization header |
| Phase 3 | 健康检查、ready 检查、metrics、审计关联、脱敏、安全测试具备基础能力 |
| Phase 4 | Dockerfile、docker-compose、环境变量示例、部署与排障文档可指导交付 |

### 3.2 明确不应出现的能力

以下能力如果出现在本次改动中，需要重点确认是否属于范围膨胀：

- 新增 `list_authorized_knowledge_bases` 等 V2 Tool；
- 让 MCP Server 自己成为新的权限中心；
- 允许 HTTP MCP 通过 tool 参数传 API Key；
- 默认开启 legacy `app_id` 生产访问；
- 大规模改造原有 `/v1/retrieve` 主链路；
- 修改租户、知识库授权、API Key 管理等核心模型。

---

## 4. 第二步：Git Diff 范围审查

### 4.1 查看当前改动范围

在仓库根目录执行：

```powershell
git status --short
git diff --stat
git diff --name-only
git ls-files --others --exclude-standard
```

重点确认：

- 是否只有 MCP 相关代码、配置、部署、文档和测试被修改；
- 是否误改了数据库模型、RAG 主检索逻辑、管理后台、前端页面；
- 是否新增了不必要依赖；
- 是否改动 `.env`、密钥、真实 token、真实 API Key；
- 是否出现大范围格式化导致 Review 困难。

### 4.2 本次建议重点 Review 的文件

| 类别 | 文件 |
|---|---|
| 启动入口 | `backend/cmd/rag-mcp-server/main.go` |
| 配置 | `backend/internal/mcp/config.go`、`backend/internal/mcp/config_test.go`、`.env.example` |
| Server 初始化 | `backend/internal/mcp/server.go`、`backend/internal/mcp/server_test.go` |
| Tool Handler | `backend/internal/mcp/handler/retrieve.go`、`backend/internal/mcp/handler/retrieve_test.go` |
| RAG 调用 | `backend/internal/mcp/client/rag_client.go` |
| HTTP transport | `backend/internal/mcp/transport/http.go`、`http_test.go`、`http_security_test.go` |
| 安全 | `backend/internal/mcp/security/origin.go`、`redact.go` |
| 指标 | `backend/internal/mcp/metrics/metrics.go` |
| 部署 | `backend/Dockerfile.mcp`、`docker-compose.yml` |
| 文档 | `backend/README.md`、`backend/docs/mcp-deployment-runbook.md` |

### 4.3 Diff 审查重点

人工 Review 时优先看这些问题：

- Authorization 是否只从 HTTP header 获取；
- tool 入参是否无法覆盖 `tenant_id`、`api_key`、`auth_type` 等身份字段；
- 转发到 RAG Server 时是否保留原始 Authorization；
- 错误映射是否会泄露敏感信息；
- 日志中是否可能输出明文 token；
- Origin 白名单是否默认安全；
- upstream timeout 是否有合理默认值；
- healthz / readyz 是否区分进程存活和依赖可用；
- Docker Compose 中 `rag-mcp-server` 是否通过内部网络访问 `rag-server`；
- `rag-mcp-server` 停止后是否不会影响 `rag-server`。

---

## 5. 第三步：Codex 二次自审

虽然本次代码由 Codex 完成，并且它已经做过基础验收，但建议在人工验收前再让 Codex 或其他 AI 工具基于当前 Diff 做一次“只审查、不改代码”的二次自审。

推荐 Prompt：

```text
请只审查当前 git diff，不要修改代码。

背景：本次改动实现 RAG 平台 MCP Server 多租户接入方案的 Phase 0 - Phase 4。
重点能力包括 stdio、HTTP /mcp、retrieve_knowledge、Authorization header 鉴权转发、Origin 校验、token 脱敏、metrics、healthz/readyz、Docker Compose 部署。

请按以下维度输出问题清单：
1. 是否偏离设计文档 Phase 0 - Phase 4；
2. 是否错误引入 Phase 5 / V2 范围；
3. 是否存在多租户越权风险；
4. 是否存在 token/API Key 泄露风险；
5. 是否破坏原有 /v1/retrieve；
6. 是否缺少关键测试；
7. 是否有部署或回滚风险；
8. 是否有无关改动。

要求：
- 只基于实际 diff 判断；
- 对每个风险给出具体文件和原因；
- 不要泛泛而谈；
- 不要为了凑数编造问题。
```

二次自审的结论不要直接采信，应作为人工 Review 的辅助输入。

---

## 6. 第四步：本地静态检查与单元测试

### 6.1 Go 编译检查

在 `backend` 目录执行：

```powershell
go test ./internal/mcp/...
go test ./cmd/rag-mcp-server ./internal/mcp/...
go test ./...
```

如果全量 `go test ./...` 因外部依赖、数据库、Milvus、历史测试问题失败，需要记录：

- 失败包名；
- 是否与本次 MCP 改动相关；
- 是否有可复现日志；
- 是否需要单独修复或暂时豁免。

不建议只因为“历史测试本来就失败”而跳过 MCP 相关包测试。

### 6.2 构建检查

在 `backend` 目录执行：

```powershell
go build ./cmd/rag-mcp-server
go build ./cmd/rag-server
```

验收点：

- MCP Server 能单独构建；
- 原有 RAG Server 仍能构建；
- 没有因为新增 MCP 代码破坏原服务入口。

### 6.3 建议重点通过的测试包

至少应通过：

```powershell
go test ./internal/mcp/...
```

该命令应覆盖：

- 配置默认值和环境变量解析；
- Tool schema / handler 参数校验；
- RAG client 请求构造和错误处理；
- HTTP initialize / tools/list / tools/call；
- Origin 白名单；
- Authorization 缺失、无效、脱敏；
- healthz / readyz；
- metrics 基础记录。

---

## 7. 第五步：MCP 协议级功能验证

### 7.1 stdio 模式验收

目标：验证 Phase 1 的本地调试链路。

建议检查：

- MCP Client 能发现 `retrieve_knowledge`；
- tools/list 返回的名称、描述、schema 与设计一致；
- tools/call 能调用 `/v1/retrieve`；
- 缺少必要环境变量时启动失败且错误清晰；
- stdout 不输出普通日志，避免污染 MCP JSON-RPC。

建议场景：

| 场景 | 预期 |
|---|---|
| 正常配置 `RAG_BASE_URL`、`RAG_ACCESS_TOKEN` | MCP Server 可启动 |
| 缺少 `RAG_BASE_URL` | 启动失败或配置校验失败 |
| 缺少本地 token | stdio 调试链路按设计失败 |
| tools/list | 能看到 `retrieve_knowledge` |
| tools/call | 返回可读文本 + 结构化 JSON |

### 7.2 HTTP `/mcp` 模式验收

目标：验证 Phase 2 的企业共享部署主链路。

建议场景：

| 场景 | 预期 |
|---|---|
| initialize | 返回 MCP 初始化结果 |
| tools/list | 返回 `retrieve_knowledge` |
| tools/call + 有效 Authorization | 能转发到 RAG Server 并返回结果 |
| tools/call + 缺少 Authorization | 失败 |
| tools/call + 无效 / 过期 / 吊销 API Key | 失败 |
| tools/call + 未授权 KB | 失败 |
| tools/call + 跨租户 KB | 失败 |
| Origin 不在白名单 | 被拒绝 |
| RAG Server 超时 | 返回可理解错误 |
| RAG Server 503 | 返回可理解错误 |

---

## 8. 第六步：多租户与安全边界验证

本次最重要的验收点是多租户隔离和凭证安全。

### 8.1 必测安全场景

| 场景 | 验收方式 | 预期 |
|---|---|---|
| tool 参数传 `api_key` | 构造恶意 tools/call | 不允许覆盖真实身份 |
| tool 参数传 `tenant_id` | 构造恶意 tools/call | 不允许覆盖真实租户 |
| tool 参数传 `auth_type` | 构造恶意 tools/call | 不允许覆盖鉴权类型 |
| 租户 A 访问租户 B KB | 使用 A 的 Authorization 调 B 的 KB | 失败 |
| 未授权 KB | 使用有效 Authorization 调未授权 KB | 失败 |
| 无 Authorization | HTTP tools/call 不带 header | 失败 |
| token 日志泄露 | 查看服务日志 | 不出现明文 token/API Key |
| Origin 非白名单 | 带非法 Origin 请求 | 被拒绝 |
| 并发请求 | 多 token 并发调用 | 不串 token、不串租户 |

### 8.2 日志脱敏检查

验收时建议主动构造带 token 的失败请求，然后检查日志。

日志中不应出现：

- 完整 API Key；
- 完整 JWT；
- `Authorization: Bearer xxx` 明文；
- 用户传入的疑似密钥字段原文；
- 数据库连接密码。

允许出现：

- token 前后少量字符的脱敏形式；
- request_id；
- 错误码；
- tenant_id、api_key_id 等审计字段，前提是不包含密钥本身。

---

## 9. 第七步：Docker Compose 部署验证

目标：验证 Phase 4 的交付链路。

### 9.1 构建验证

在仓库根目录执行：

```powershell
docker compose build rag-mcp-server
```

验收点：

- `backend/Dockerfile.mcp` 可构建；
- 构建上下文正确；
- 不依赖本地未提交文件以外的隐藏环境；
- 镜像中不包含 `.env`、密钥等敏感文件。

### 9.2 启动验证

在仓库根目录执行：

```powershell
docker compose up -d rag-server rag-mcp-server
```

验收点：

- `rag-mcp-server` 能启动；
- `rag-mcp-server` 能通过内部网络访问 `http://rag-server:8899`；
- `rag-mcp-server` 对外暴露端口符合 `MCP_PORT`；
- `rag-server` 不需要为了 MCP 额外暴露宿主机端口。

### 9.3 健康检查

建议检查：

```powershell
docker compose ps rag-mcp-server
docker compose logs rag-mcp-server
```

验收点：

- `/healthz` 表示 MCP Server 进程存活；
- `/readyz` 表示依赖可用或至少状态可解释；
- healthcheck 不依赖不稳定的外部网络；
- 日志无明文 token。

### 9.4 回滚验证

至少验证：

```powershell
docker compose stop rag-mcp-server
```

预期：

- 原有 `rag-server` 仍可继续服务；
- 原有 `/v1/retrieve` 不受影响；
- 管理后台不受影响；
- 不需要回滚数据库结构即可关闭 MCP Server。

---

## 10. 第八步：原有能力回归验证

本次 MCP Server 是适配层，不应破坏原有 RAG 能力。

### 10.1 必做回归

| 回归项 | 预期 |
|---|---|
| `/v1/retrieve` 原 HTTP 检索 | 行为不变 |
| API Key 鉴权 | 行为不变 |
| JWT 鉴权 | 行为不变 |
| 知识库授权 | 行为不变 |
| 检索审计 | 行为不变 |
| 管理后台 | 不受 MCP 改动影响 |
| Docker Compose 原服务 | 不因新增 MCP 服务启动失败 |

### 10.2 回归判断原则

如果 MCP 相关测试通过，但原有 `/v1/retrieve` 出现异常，本次不能直接合并。

除非能证明：

- 异常是环境问题或历史问题；
- 与本次 MCP 改动无关；
- 已记录复现方式和后续处理结论。

---

## 11. 人工 Review 清单

合并前建议人工逐项确认：

### 11.1 需求范围

- [ ] 只实现 Phase 0 - Phase 4；
- [ ] 没有提前实现 V2 Tool；
- [ ] 没有让 MCP Server 成为权限中心；
- [ ] 没有大规模改造 RAG 主链路。

### 11.2 多租户与鉴权

- [ ] HTTP MCP 只从 Authorization header 获取凭证；
- [ ] tool 参数不能传 API Key；
- [ ] tool 参数不能覆盖租户身份；
- [ ] 未授权 KB 访问失败；
- [ ] 跨租户 KB 访问失败；
- [ ] API Key 过期 / 吊销访问失败。

### 11.3 安全

- [ ] Origin 白名单有效；
- [ ] token/API Key 日志脱敏；
- [ ] 错误响应不泄露内部细节；
- [ ] 无真实密钥提交；
- [ ] `.env.example` 只包含示例值；
- [ ] Docker 镜像不包含本地敏感文件。

### 11.4 稳定性

- [ ] upstream timeout 有默认值；
- [ ] RAG Server 503 / timeout 有错误映射；
- [ ] healthz / readyz 可用；
- [ ] metrics 不影响主流程；
- [ ] 并发调用不串 token。

### 11.5 部署与回滚

- [ ] `rag-mcp-server` 可独立构建；
- [ ] Docker Compose 可启动 MCP Server；
- [ ] MCP Server 通过内部网络访问 RAG Server；
- [ ] 停止 MCP Server 不影响 RAG Server；
- [ ] Runbook 包含接入、排障和回滚说明。

---

## 12. 建议的最终验收命令清单

以下命令可以作为本次合并前的最小验收命令集。

### 12.1 代码范围

```powershell
git status --short
git diff --stat
git diff --name-only
```

### 12.2 Go 测试与构建

```powershell
cd backend
go test ./internal/mcp/...
go build ./cmd/rag-mcp-server
go build ./cmd/rag-server
```

如环境允许，再执行：

```powershell
go test ./...
```

### 12.3 Docker 验证

```powershell
cd ..
docker compose build rag-mcp-server
docker compose up -d rag-server rag-mcp-server
docker compose ps rag-mcp-server
docker compose logs rag-mcp-server
```

### 12.4 回滚验证

```powershell
docker compose stop rag-mcp-server
docker compose ps rag-server
```

---

## 13. 合并前结论模板

建议每次 Codex 辅助开发完成后，在 PR 描述或提交说明中保留以下结论：

```markdown
## Codex 辅助开发验收结论

### 改动范围
- [ ] MCP Server 入口
- [ ] MCP 配置
- [ ] retrieve_knowledge Tool
- [ ] HTTP transport
- [ ] 安全与脱敏
- [ ] metrics / healthz / readyz
- [ ] Docker Compose / Dockerfile
- [ ] 文档与 Runbook

### 已执行验证
- [ ] git diff 已人工审查
- [ ] Codex 二次自审已完成
- [ ] go test ./internal/mcp/... 通过
- [ ] go build ./cmd/rag-mcp-server 通过
- [ ] go build ./cmd/rag-server 通过
- [ ] Docker Compose 构建通过
- [ ] Docker Compose 启动通过
- [ ] MCP Server 健康检查通过
- [ ] 停止 MCP Server 不影响 rag-server

### 安全验证
- [ ] 无 Authorization 请求失败
- [ ] 无效 / 过期 / 吊销 API Key 失败
- [ ] 未授权 KB 失败
- [ ] 跨租户 KB 失败
- [ ] Origin 非白名单失败
- [ ] 日志无明文 token/API Key
- [ ] tool 参数不能覆盖身份字段

### 回归验证
- [ ] 原 /v1/retrieve 不受影响
- [ ] API Key 管理不受影响
- [ ] 知识库授权不受影响
- [ ] 检索审计不受影响
- [ ] 管理后台不受影响

### 遗留问题
- 无 / 有：请列出

### 合并判断
- [ ] 可以合并
- [ ] 暂不合并，需修复上述问题
```

---

## 14. 我的建议

对于你当前这种“Codex 已完成基础开发与基础验收”的情况，最适合的验收策略不是重新手工测一遍所有细节，而是做三层把关：

1. **契约把关**：严格对照 Phase 0 - Phase 4，确认没有偏离设计和范围膨胀；
2. **安全把关**：重点验证 Authorization、租户隔离、Origin、token 脱敏、tool 参数不可覆盖身份；
3. **交付把关**：确认 `go test ./internal/mcp/...`、构建、Docker Compose、健康检查和回滚链路可用。

只要这三层都通过，本次 MCP 多租户接入改动就具备进入人工 Review / PR 合并的基础条件。