# RAG平台MCP 配置说明文档

本文档说明 `rag-mcp-server` 当前可用的核心配置项、用途、典型取值和生产建议。

## 1. 配置总览

MCP Server 当前主要通过环境变量配置，核心入口代码在：

- `backend/internal/mcp/config.go`
- `backend/cmd/rag-mcp-server/main.go`

配置大体分为 6 类：

1. 运行开关
2. 传输层配置
3. 安全配置
4. 上游 RAG 配置
5. 可观测性配置
6. 会话配置

## 2. 配置项说明

### APP_ENV

- 作用：标识运行环境，当前重点区分 `prod` 和非 `prod`
- 常见值：`dev`、`test`、`prod`
- 当前行为：
  - 当 `APP_ENV=prod` 且 `MCP_TRANSPORT=stdio` 时，启动失败
  - 当 `APP_ENV=prod` 且 `MCP_TRANSPORT=http` 且 `MCP_ALLOWED_ORIGINS` 为空时，启动失败
- 建议：
  - 本地开发用 `dev`
  - 预发/生产明确设置为 `prod`

### MCP_ENABLED

- 作用：是否启用 MCP Server 配置
- 常见值：`true`、`false`
- 当前行为：
  - `false` 时配置校验直接失败，进程不会继续启动
- 建议：
  - 只有在确实需要运行 MCP Server 的环境里设为 `true`

### MCP_TRANSPORT

- 作用：选择 MCP 传输方式
- 常见值：`http`、`stdio`
- 当前行为：
  - `http`：启动 HTTP MCP 服务，监听 `/mcp`
  - `stdio`：启动本地标准输入输出 MCP 服务
  - `prod + stdio`：禁止启动
- 建议：
  - 多租户共享部署统一使用 `http`
  - `stdio` 仅用于本地单用户调试

### MCP_HOST

- 作用：HTTP 模式监听地址
- 常见值：`127.0.0.1`、`0.0.0.0`
- 当前行为：
  - 仅 `http` transport 会使用
- 建议：
  - 本地开发可用 `127.0.0.1`
  - 容器内通常使用 `0.0.0.0`

### MCP_PORT

- 作用：HTTP 模式监听端口
- 常见值：`8898`
- 当前行为：
  - 端口必须在 `1-65535`
- 建议：
  - 与网关、Compose、健康检查配置保持一致

### MCP_ENDPOINT

- 作用：HTTP MCP 入口路径
- 常见值：`/mcp`
- 当前行为：
  - 必须以 `/` 开头
- 建议：
  - 保持默认 `/mcp`，方便客户端和文档统一

### MCP_ALLOWED_ORIGINS

- 作用：配置允许访问 HTTP MCP 的 Origin 白名单
- 常见值：
  - `http://localhost:3000,http://localhost:3001`
  - `https://agent.example.com,https://admin.example.com`
- 当前行为：
  - 为空时：非生产环境可以启动，但带 `Origin` 的请求会被拒绝
  - `prod + http + 为空`：禁止启动
- 建议：
  - 生产必须显式配置
  - 只放真正需要调用 MCP 的前端/网关 Origin

### MCP_REQUIRE_ORIGIN_HEADER

- 作用：是否要求请求必须携带 `Origin`
- 常见值：`true`、`false`
- 当前行为：
  - `true`：没有 `Origin` 头会被拒绝
  - `false`：无 `Origin` 请求允许继续走
- 建议：
  - 纯服务到服务调用通常保留 `false`
  - 如果明确所有调用都会经过浏览器或固定网关，可按需开启

### MCP_UPSTREAM_TIMEOUT_MS

- 作用：MCP 调用内部 `rag-server` 的超时时间
- 常见值：`5000`
- 当前行为：
  - 必须为正整数毫秒
  - 控制 `/v1/retrieve` 上游 HTTP client timeout
- 建议：
  - 默认 `5000` 比较合适
  - 若检索链路较重，可在预发观测后适度放宽

### MCP_SESSION_TIMEOUT_MS

- 作用：Streamable HTTP 会话空闲超时
- 常见值：`300000`（5 分钟）
- 当前行为：
  - 必须为正整数毫秒
  - 当前 HTTP handler 虽然使用无状态模式，但仍保留该配置兼容后续扩展
- 建议：
  - 默认值即可

### MCP_DISABLE_HTTP_AUTH

- 作用：是否关闭 HTTP Bearer 认证中间层
- 常见值：`true`、`false`
- 当前行为：
  - `false`：HTTP MCP 要求 `Authorization: Bearer ...`
  - `true`：跳过 Bearer 存在性检查
- 建议：
  - 生产始终保持 `false`
  - 仅测试场景临时使用

### MCP_DISABLE_METRICS

- 作用：是否关闭 `/metrics`
- 常见值：`true`、`false`
- 当前行为：
  - `false`：暴露 `/metrics`
  - `true`：不注册该路由
- 建议：
  - 生产建议保持开启，结合 Prometheus 抓取

### MCP_ENABLE_LEGACY_APP_ID

- 作用：历史兼容开关占位
- 当前行为：
  - 任何情况下只要设为 `true`，启动就失败
- 原因：
  - MCP Server V1 明确不支持 legacy `app_id` 模式
- 建议：
  - 一律保持 `false`

### RAG_BASE_URL

- 作用：上游内部 RAG 服务地址
- 常见值：
  - 本地：`http://localhost:8899`
  - Compose：`http://rag-server:8899`
- 当前行为：
  - 必须存在
  - 必须是合法的绝对 `http(s)` URL
- 建议：
  - 生产必须指向内网服务地址，不要指向外网代理地址

### RAG_ACCESS_TOKEN

- 作用：stdio 模式下固定使用的 Bearer 凭证
- 常见值：`rag_local_debug_token`
- 当前行为：
  - `stdio` 模式必填
  - `http` 模式不作为每请求认证来源
- 建议：
  - 只用于本地调试
  - 不要把生产多租户认证设计成依赖它

## 3. 不同场景推荐配置

### 本地 stdio 调试

```env
APP_ENV=dev
MCP_ENABLED=true
MCP_TRANSPORT=stdio
RAG_BASE_URL=http://localhost:8899
RAG_ACCESS_TOKEN=rag_local_debug_token
```

### 本地 HTTP 调试

```env
APP_ENV=dev
MCP_ENABLED=true
MCP_TRANSPORT=http
MCP_HOST=127.0.0.1
MCP_PORT=8898
MCP_ENDPOINT=/mcp
MCP_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
RAG_BASE_URL=http://localhost:8899
```

### 生产 HTTP 部署

```env
APP_ENV=prod
MCP_ENABLED=true
MCP_TRANSPORT=http
MCP_HOST=0.0.0.0
MCP_PORT=8898
MCP_ENDPOINT=/mcp
MCP_ALLOWED_ORIGINS=https://agent.example.com,https://admin.example.com
MCP_UPSTREAM_TIMEOUT_MS=5000
MCP_DISABLE_HTTP_AUTH=false
MCP_ENABLE_LEGACY_APP_ID=false
RAG_BASE_URL=http://rag-server:8899
```

## 4. 当前生产保护

当前已经实现两条关键生产保护：

1. 当 `APP_ENV=prod` 且 `MCP_TRANSPORT=http` 时，如果 `MCP_ALLOWED_ORIGINS` 为空，启动失败。
2. 当 `APP_ENV=prod` 且 `MCP_TRANSPORT=stdio` 时，启动失败。

对应测试位于：

- `backend/internal/mcp/config_test.go`

## 5. 还建议继续加的保护

- 当 `APP_ENV=prod` 且 `MCP_DISABLE_HTTP_AUTH=true` 时，启动失败
- 当 `APP_ENV=prod` 且 `MCP_ALLOWED_ORIGINS` 中出现通配符或非 HTTPS 域名时，给出告警或拒绝启动
- 当 `APP_ENV=prod` 且 `RAG_BASE_URL` 指向公网地址时，给出告警

## 6. 你下一步应该做什么

最建议优先做这三件事：

1. 在预发/生产环境真实配置 `MCP_ALLOWED_ORIGINS`，然后重新做一次带 `Origin` 的 HTTP MCP 探针验证。
2. 准备真实租户、KB、API Key 数据，完成“过期 key / 吊销 key / 未授权 KB / 跨租户 KB / request_id 审计回查”的联调。
3. 把当前 MCP 测试和 `docker compose` smoke 校验接进 CI，避免后续改动把生产保护打掉。
