# RAG平台MCP 验收文档

生成时间：2026-06-11

参考依据：

- `gaoerDocs/shaoshuai开发文档/RAG平台MCP Server多租户接入合并设计方案.md`
- 第 16 节测试计划
- 第 17 节上线检查清单

本次验收范围：

- Phase 2：Streamable HTTP 多租户版本
- Phase 3：审计、限流与监控加固
- Phase 4：Docker Compose 与交付文档
- 生产启动保护补齐

已完成的主要实现：

- 新增 HTTP MCP `/mcp` 入口，支持 Streamable HTTP
- `Authorization` 通过请求头透传到上游 `/v1/retrieve`
- 新增 Origin 白名单保护、日志脱敏、`/healthz`、`/readyz`、MCP 指标
- 保留 stdio 本地调试模式，但新增生产环境禁用保护
- 新增 `Dockerfile.mcp`、`docker-compose` 服务、`.env.example` MCP 配置、部署/排障/回滚文档
- 新增生产启动保护：
  - `APP_ENV=prod && MCP_TRANSPORT=http && MCP_ALLOWED_ORIGINS 为空` 时启动失败
  - `APP_ENV=prod && MCP_TRANSPORT=stdio` 时启动失败

## 通过项

### 16.1 单元测试

- `Tool schema 校验`：通过
- `空 query`：通过
- `缺少 kb_id / kb_ids`：通过
- `kb_id 和 kb_ids 合并`：通过
- `禁止身份覆盖字段`：通过
- `top_k 边界`：通过部分自动验证
- `错误映射`：通过
- `token 脱敏`：通过
- `生产 HTTP 缺少 Origin 白名单启动保护`：通过  
  证据：`backend/internal/mcp/config_test.go`
- `生产 stdio 禁止启动`：通过  
  证据：`backend/internal/mcp/config_test.go`

### 16.2 集成测试

- `stdio tools/list`：通过  
  命令：`go test ./cmd/rag-mcp-server/...`
- `stdio tools/call`：通过  
  命令：`go test ./cmd/rag-mcp-server/...`
- `HTTP initialize`：通过  
  命令：`go test ./internal/mcp/...`
- `HTTP tools/list`：通过  
  命令：`go test ./internal/mcp/...`
- `HTTP tools/call`：通过  
  命令：`go test ./internal/mcp/...`
- `无 Authorization`：通过
- `RAG Server 503`：通过
- `RAG Server timeout`：通过

### 16.3 安全测试

- `Origin 白名单`：通过
- `Authorization 不落日志`：通过
- `tool 参数不能覆盖 tenant`：通过
- `tool 参数不能传 api_key`：通过
- `metadata_filter 大对象`：通过
- `并发调用下无 token 串用`：通过

### 16.4 回归测试

- `API Key 管理不受影响`：通过  
  命令：`go test ./api/handler/auth`
- `检索审计不受影响`：通过部分自动验证  
  命令：`go test ./api/handler/kb -run "TestEnrichRetrieveLogWithPlatformContextForAPIKey|TestEnrichRetrieveLogWithPlatformContextForLegacy|TestBuildAuditContractGaps|TestMaskAuditPayload"`
- `Docker Compose 原有服务不受影响`：通过部分自动验证  
  命令：`docker compose config`、`docker compose up -d mysql redis milvus rag-server rag-mcp-server`

### 17. 上线检查清单

- `Tool 名称统一为 retrieve_knowledge`：通过
- `V1 不通过 tool 参数传 API Key`：通过
- `HTTP MCP 使用 Authorization header`：通过
- `stdio 仅用于本地调试`：通过  
  说明：现在生产环境已被启动保护禁止，非生产环境仍允许本地调试
- `生产默认关闭 legacy app_id`：通过  
  说明：V1 中显式不支持，配置为 `true` 时直接启动失败
- `token 不落日志`：通过
- `RAG_BASE_URL 指向内部服务地址`：通过
- `关闭 MCP Server 不影响 /v1/retrieve`：通过部分自动验证
- `Docker Compose 部署验证通过`：通过

## 未通过项

- `Origin 白名单已配置`：未通过  
  当前代码已强制生产环境必须配置，但本地当前 Compose 展开的有效值仍为空，所以“当前环境已配置完成”这一条仍不能判通过。
- `共享 HTTP 客户端带 Origin 的真实探针`：未通过  
  实测带 `Origin: http://localhost:3000` 的请求会被 `403` 拒绝。  
  这说明保护逻辑正常，但也说明当前环境尚未填入允许的 Origin。

## 风险项

- `无效 API Key / 过期 API Key / 吊销 API Key`：当前未做真实环境端到端验证  
  原因：MCP 层只校验 Bearer 非空并透传，上游 RAG 才是凭证事实来源。  
  人工验证方法：准备 1 个有效 key、1 个过期 key、1 个已吊销 key，分别通过 HTTP MCP 调用 `retrieve_knowledge`，确认返回 `200 / unauthorized / unauthorized`，并检查 `request_id` 与日志。

- `未授权 KB / 跨租户 KB`：当前未做真实环境端到端验证  
  原因：需要当前数据库里存在可区分租户和 KB 授权的数据集。  
  人工验证方法：准备 A/B 两个租户，给 A 的 API Key 调 A 的 KB、B 的 KB、未授权 KB，确认分别返回成功、`forbidden`、`forbidden/not_found`。

- `审计日志可通过 request_id 回查`：当前只验证了代码路径和已有审计单测，未做完整联调  
  人工验证方法：成功调用一次 HTTP MCP，记录响应 `request_id`，再通过 RAG 后台或审计查询接口 `/retrieve/audit/:request_id` 回查，确认 `tenant_id`、`api_key_id`、`auth_type`、`source_api` 字段完整。

- `管理后台不受影响`、`原有 /v1/retrieve` 完整业务响应不受影响`：当前自动化验证仍不足`  
  本次验证了服务存活、Compose 拓扑和回滚后 `rag-server` 健康状态正常，但没有覆盖完整业务回归请求集。

- `全量 api/handler/kb` 测试`：当前环境缺少本地 MySQL 3307，无法全部自动通过`  
  失败不是 MCP 改动导致，而是测试环境依赖缺失。  
  命令输出显示 `dial tcp 127.0.0.1:3307 ... actively refused`。

## 下一步建议

- 在预发/生产环境真实配置 `MCP_ALLOWED_ORIGINS`，然后重新做一次带 `Origin` 的 HTTP MCP 探针验证。
- 准备真实租户、KB、API Key 数据，完成：
  `无效/过期/吊销 key`、`未授权 KB`、`跨租户 KB`、`request_id 审计回查`
- 继续增加一条生产保护：
  `APP_ENV=prod && MCP_DISABLE_HTTP_AUTH=true` 时直接拒绝启动
- 把当前 MCP 测试和 `docker compose` smoke 校验接进 CI

## 本次执行过的主要自动化命令

```bash
go test ./internal/mcp/... ./cmd/rag-mcp-server/...
go test ./api/handler/auth
go test ./api/handler/kb -run "TestEnrichRetrieveLogWithPlatformContextForAPIKey|TestEnrichRetrieveLogWithPlatformContextForLegacy|TestBuildAuditContractGaps|TestMaskAuditPayload"
go build ./cmd/rag-mcp-server
docker compose config
docker build -f backend/Dockerfile.mcp backend
docker compose up -d mysql redis milvus rag-server rag-mcp-server
docker compose ps rag-server rag-mcp-server
docker compose exec rag-mcp-server sh -lc 'curl -fsS http://127.0.0.1:${MCP_PORT:-8898}/healthz && echo && curl -fsS http://rag-server:8899/healthz && echo && curl -fsS http://rag-server:8899/readyz'
docker compose stop rag-mcp-server
docker compose up -d rag-mcp-server
```

## 结论

当前实现已经完成设计文档中 Phase 2、Phase 3、Phase 4 的主体开发，并补齐了关键生产启动保护。MCP 侧核心自动化测试、容器构建和 Compose 部署验证均已通过。

当前仍不建议直接按“完全通过”上线，主要剩余阻塞不再是代码缺失，而是环境配置和业务联调未闭环：

- 环境配置层：当前运行环境还没有真实填入 `MCP_ALLOWED_ORIGINS`
- 业务安全层：真实 API Key 生命周期、未授权 KB、跨租户 KB、审计回查尚未完成预发联调

补齐这两类项后，再按本文档复跑验收，会更稳妥。
