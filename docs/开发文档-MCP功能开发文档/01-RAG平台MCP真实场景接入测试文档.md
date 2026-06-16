# RAG平台MCP真实场景接入测试文档

## 1. 当前状态说明

当前 RAG 平台 MCP Server 的主体功能已经完成，可以进入真实场景测试阶段。

已具备能力：

- HTTP MCP 接入入口：`/mcp`
- MCP 工具：`retrieve_knowledge`
- 通过 HTTP Header 传递 `Authorization: Bearer <RAG_API_KEY>`
- MCP Server 将 Authorization 原样透传给内部 `rag-server`
- 由原 RAG Server 负责 API Key 校验、租户解析、知识库权限校验和审计记录
- 支持 Origin 白名单
- 支持 token 脱敏日志
- 支持 `/healthz`、`/readyz`、`/metrics`
- 支持 Docker Compose 部署
- 保留 stdio 本地调试模式

当前仍建议在真实场景中重点验收：

- 真实 Agent 是否能正常发现并调用 MCP 工具
- 真实 API Key 是否能访问授权知识库
- 无效、过期、吊销 API Key 是否会被拒绝
- 未授权知识库、跨租户知识库是否会被拒绝
- 成功调用后是否能通过 `request_id` 回查审计日志
- 日志中是否不会出现明文 API Key

## 2. 接入方式选择

### 2.1 生产/预发/多租户共享场景

使用 HTTP MCP。

推荐入口：

```text
POST http://<MCP_HOST>:8898/mcp
```

如果经过网关，则使用网关地址：

```text
POST https://<your-domain>/mcp
```

### 2.2 本地单人调试场景

可以使用 stdio。

stdio 只适合本地调试，不适合作为企业多租户共享入口。

## 3. HTTP MCP 接入信息

### 3.1 MCP 地址

本地 Docker Compose 默认地址：

```text
http://localhost:8898/mcp
```

容器内部地址：

```text
http://rag-mcp-server:8898/mcp
```

真实环境请替换为你的网关或服务地址：

```text
https://<your-rag-mcp-domain>/mcp
```

### 3.2 必须携带的请求头

```http
Authorization: Bearer <RAG_API_KEY>
Content-Type: application/json
Accept: application/json, text/event-stream
MCP-Protocol-Version: 2025-06-18
```

如果从浏览器、Agent 平台前端或网关访问，可能还会带：

```http
Origin: https://<your-agent-domain>
```

注意：

- 生产环境必须配置 `MCP_ALLOWED_ORIGINS`
- 如果 `Origin` 不在白名单中，请求会被拒绝
- 服务到服务调用可以不带 `Origin`，具体取决于 `MCP_REQUIRE_ORIGIN_HEADER` 配置

## 4. MCP 工具说明

### 4.1 工具名称

```text
retrieve_knowledge
```

### 4.2 工具作用

根据用户问题，在当前 API Key 有权限的知识库中检索知识片段，返回可引用证据。

### 4.3 输入参数

| 字段 | 类型 | 是否必填 | 说明 |
|---|---|---:|---|
| `query` | string | 是 | 用户问题或检索 query |
| `kb_ids` | number[] | 推荐 | 知识库 ID 列表 |
| `kb_id` | number | 可选 | 单个知识库 ID，兼容字段 |
| `top_k` | number | 可选 | 返回数量，建议 1-20 |
| `strategy_profile` | string | 可选 | 预留字段 |
| `metadata_filter` | object | 可选 | 预留字段 |

至少需要提供 `kb_ids` 或 `kb_id` 之一。

### 4.4 禁止传入的参数

工具参数中禁止传入：

```text
api_key
tenant_id
api_key_id
user_id
role
auth_type
```

这些身份信息只能由 RAG Server 根据 `Authorization` 解析，不能由客户端自己传。

## 5. HTTP MCP 手工测试

下面命令适合在 PowerShell 中执行。

请先准备：

```powershell
$MCP_URL = "http://localhost:8898/mcp"
$RAG_API_KEY = "替换成真实 RAG API Key"
$KB_ID = 3
```

### 5.1 健康检查

```powershell
Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:8898/healthz" | Select-Object -ExpandProperty Content
Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:8898/readyz" | Select-Object -ExpandProperty Content
```

期望结果：

```json
{"status":"ok"}
{"status":"ready"}
```

### 5.2 MCP initialize 测试

```powershell
$headers = @{
  Authorization = "Bearer $RAG_API_KEY"
  "Content-Type" = "application/json"
  Accept = "application/json, text/event-stream"
  "MCP-Protocol-Version" = "2025-06-18"
}

$body = '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"manual-test","version":"0.1.0"}}}'

Invoke-WebRequest -UseBasicParsing -Method POST -Uri $MCP_URL -Headers $headers -Body $body | Select-Object -ExpandProperty Content
```

期望结果：

- HTTP 状态码为 `200`
- 返回中包含：

```text
serverInfo
rag-mcp-server
tools
```

### 5.3 查询工具列表

```powershell
$body = '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'

Invoke-WebRequest -UseBasicParsing -Method POST -Uri $MCP_URL -Headers $headers -Body $body | Select-Object -ExpandProperty Content
```

期望结果：

- 返回工具列表
- 包含工具名：

```text
retrieve_knowledge
```

### 5.4 调用 retrieve_knowledge

```powershell
$bodyObj = @{
  jsonrpc = "2.0"
  id = 3
  method = "tools/call"
  params = @{
    name = "retrieve_knowledge"
    arguments = @{
      query = "知识库里关于 Go 并发的内容是什么？"
      kb_ids = @($KB_ID)
      top_k = 5
    }
  }
}

$body = $bodyObj | ConvertTo-Json -Depth 10 -Compress

Invoke-WebRequest -UseBasicParsing -Method POST -Uri $MCP_URL -Headers $headers -Body $body | Select-Object -ExpandProperty Content
```

期望结果：

- HTTP 状态码为 `200`
- MCP result 中 `isError` 不应为 `true`
- 返回内容中包含：
  - `request_id`
  - `items`
  - `kb_id`
  - `document_id`
  - `chunk_id`
  - `score`

示例成功特征：

```text
Retrieved 5 item(s).
request_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

## 6. 安全验收测试

### 6.1 不带 Authorization

```powershell
$headersNoAuth = @{
  "Content-Type" = "application/json"
  Accept = "application/json, text/event-stream"
  "MCP-Protocol-Version" = "2025-06-18"
}

$body = '{"jsonrpc":"2.0","id":11,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"manual-test","version":"0.1.0"}}}'

try {
  Invoke-WebRequest -UseBasicParsing -Method POST -Uri $MCP_URL -Headers $headersNoAuth -Body $body
} catch {
  $_.Exception.Response.StatusCode.value__
  $_.ErrorDetails.Message
}
```

期望结果：

```text
401
no bearer token
```

### 6.2 无效 API Key

```powershell
$headersInvalid = @{
  Authorization = "Bearer rag_invalid_security_test_token"
  "Content-Type" = "application/json"
  Accept = "application/json, text/event-stream"
  "MCP-Protocol-Version" = "2025-06-18"
}

$bodyObj = @{
  jsonrpc = "2.0"
  id = 12
  method = "tools/call"
  params = @{
    name = "retrieve_knowledge"
    arguments = @{
      query = "Go concurrency"
      kb_ids = @($KB_ID)
      top_k = 1
    }
  }
}

$body = $bodyObj | ConvertTo-Json -Depth 10 -Compress

Invoke-WebRequest -UseBasicParsing -Method POST -Uri $MCP_URL -Headers $headersInvalid -Body $body | Select-Object -ExpandProperty Content
```

期望结果：

- 不返回检索数据
- MCP result 中 `isError=true`
- 错误语义应表现为未授权或无效凭证

注意：当前实现中，无效但格式不完整的 key 可能返回 legacy app_id 相关错误。只要没有放行和没有返回数据，安全拒绝是成立的；但后续建议把错误语义统一优化为 `unauthorized`。

### 6.3 工具参数尝试传身份字段

```powershell
$bodyObj = @{
  jsonrpc = "2.0"
  id = 13
  method = "tools/call"
  params = @{
    name = "retrieve_knowledge"
    arguments = @{
      query = "Go concurrency"
      kb_ids = @($KB_ID)
      top_k = 1
      tenant_id = 999
      api_key = "should_not_be_allowed"
    }
  }
}

$body = $bodyObj | ConvertTo-Json -Depth 10 -Compress

Invoke-WebRequest -UseBasicParsing -Method POST -Uri $MCP_URL -Headers $headers -Body $body | Select-Object -ExpandProperty Content
```

期望结果：

- 调用失败
- 返回内容中包含类似：

```text
unexpected additional properties
api_key
tenant_id
```

### 6.4 访问不存在知识库

```powershell
$bodyObj = @{
  jsonrpc = "2.0"
  id = 14
  method = "tools/call"
  params = @{
    name = "retrieve_knowledge"
    arguments = @{
      query = "Go concurrency"
      kb_ids = @(999999)
      top_k = 1
    }
  }
}

$body = $bodyObj | ConvertTo-Json -Depth 10 -Compress

Invoke-WebRequest -UseBasicParsing -Method POST -Uri $MCP_URL -Headers $headers -Body $body | Select-Object -ExpandProperty Content
```

期望结果：

```text
not_found
Knowledge base not found
```

### 6.5 跨租户知识库访问

准备数据：

- 租户 A 的 API Key
- 租户 B 的知识库 ID

用租户 A 的 API Key 调用租户 B 的知识库。

```powershell
$CROSS_TENANT_KB_ID = 2

$bodyObj = @{
  jsonrpc = "2.0"
  id = 15
  method = "tools/call"
  params = @{
    name = "retrieve_knowledge"
    arguments = @{
      query = "Go concurrency"
      kb_ids = @($CROSS_TENANT_KB_ID)
      top_k = 1
    }
  }
}

$body = $bodyObj | ConvertTo-Json -Depth 10 -Compress

Invoke-WebRequest -UseBasicParsing -Method POST -Uri $MCP_URL -Headers $headers -Body $body | Select-Object -ExpandProperty Content
```

期望结果：

- 不返回检索数据
- 返回 `forbidden` 或 `not_found`

如果系统为了避免泄露其他租户知识库是否存在而返回 `not_found`，这是可以接受的。

### 6.6 过期 API Key

准备一个已过期 API Key。

用该 key 调用 `retrieve_knowledge`。

期望结果：

- 不返回检索数据
- 返回未授权错误
- 不应写入成功检索审计

### 6.7 吊销 API Key

准备一个状态为 revoked 的 API Key。

用该 key 调用 `retrieve_knowledge`。

期望结果：

- 不返回检索数据
- 返回未授权错误
- 不应写入成功检索审计

## 7. 审计回查

成功调用 `retrieve_knowledge` 后，响应中会包含 `request_id`。

例如：

```text
request_id: ad506b92-c8a1-44a1-b7fa-5d2e10605c99
```

可以通过数据库回查：

```powershell
docker compose exec -T mysql mysql -uroot -p"root" interview_agent -e "SELECT request_id, tenant_id, app_id, api_key_id, auth_type, source_api, permission_result, kb_ids, result_status FROM kb_retrieve_log WHERE request_id='<替换成真实 request_id>'\G"
```

期望结果：

```text
request_id: <真实 request_id>
tenant_id: <租户 ID>
app_id: <应用 ID>
api_key_id: <API Key ID>
auth_type: api_key
source_api: v1
permission_result: allowed
kb_ids: <本次请求的 KB ID>
result_status: success
```

说明：

- MCP Server 本身不做租户决策
- 审计事实来源仍然是 RAG Server
- MCP 只负责协议适配和 Authorization 透传

## 8. 日志脱敏检查

执行一次成功调用后，检查 MCP Server 日志：

```powershell
docker compose logs --no-color --tail=200 rag-mcp-server
```

期望结果：

- 不应出现完整 API Key
- 可以出现脱敏后的指纹，例如：

```text
auth=sha256:e6bd273a
```

如果日志中出现完整 `rag_xxx`，则验收失败，需要立即修复。

## 9. Origin 白名单测试

如果你的真实场景会从浏览器或 Agent 平台前端访问 MCP，需要测试 Origin。

### 9.1 配置示例

```env
APP_ENV=prod
MCP_ALLOWED_ORIGINS=https://agent.example.com,https://admin.example.com
```

### 9.2 合法 Origin

```powershell
$headersWithOrigin = @{
  Authorization = "Bearer $RAG_API_KEY"
  "Content-Type" = "application/json"
  Accept = "application/json, text/event-stream"
  "MCP-Protocol-Version" = "2025-06-18"
  Origin = "https://agent.example.com"
}
```

期望结果：

- 请求可以继续进入 MCP 协议处理

### 9.3 非法 Origin

```powershell
$headersBadOrigin = @{
  Authorization = "Bearer $RAG_API_KEY"
  "Content-Type" = "application/json"
  Accept = "application/json, text/event-stream"
  "MCP-Protocol-Version" = "2025-06-18"
  Origin = "https://blocked.example.com"
}
```

期望结果：

```text
403
```

## 10. Agent 平台接入建议

### 10.1 远程 HTTP MCP 接入

如果 Agent 平台支持 Streamable HTTP MCP，请填写：

```text
MCP Server URL: https://<your-domain>/mcp
Authorization: Bearer <RAG_API_KEY>
Protocol Version: 2025-06-18
```

并确保平台会透传：

```text
Authorization
Accept
Content-Type
MCP-Protocol-Version
Origin
```

### 10.2 工具调用参数示例

```json
{
  "query": "知识库里关于 Go 并发的内容是什么？",
  "kb_ids": [3],
  "top_k": 5
}
```

### 10.3 Agent 提示词建议

可以告诉 Agent：

```text
当用户问题需要查询企业知识库、项目文档、技术文档或内部资料时，调用 retrieve_knowledge 工具。
调用时必须传入 query 和授权范围内的 kb_ids。
不要在工具参数中传 API Key、tenant_id、user_id 等身份字段。
返回结果中的 citation 可作为回答引用来源。
```

## 11. 本地 stdio 接入示例

仅用于本地调试。

### 11.1 Claude Desktop / Cursor 示例

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "command": "h:/GYT-CODE/rag-retrievalOps/backend/rag-mcp-server.exe",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_xxx"
      }
    }
  }
}
```

注意：

- `command` 要替换为真实可执行文件路径
- `RAG_ACCESS_TOKEN` 替换为本地测试 API Key
- stdio 模式不支持每次请求切换租户 token
- 生产不要用 stdio

## 12. 上线前最小验收清单

真实场景测试时，建议至少完成以下项目：

| 项目 | 是否通过 | 备注 |
|---|---|---|
| MCP `/healthz` 正常 |  |  |
| MCP `/readyz` 正常 |  |  |
| MCP initialize 成功 |  |  |
| tools/list 能看到 `retrieve_knowledge` |  |  |
| 有效 API Key + 授权 KB 调用成功 |  |  |
| 返回结果包含 `request_id` |  |  |
| 审计日志能通过 `request_id` 回查 |  |  |
| 无 Authorization 被拒绝 |  |  |
| 无效 API Key 被拒绝 |  |  |
| 过期 API Key 被拒绝 |  |  |
| 吊销 API Key 被拒绝 |  |  |
| 未授权 KB 被拒绝 |  |  |
| 跨租户 KB 被拒绝 |  |  |
| tool 参数传 `api_key` 被拒绝 |  |  |
| tool 参数传 `tenant_id` 被拒绝 |  |  |
| MCP 日志不出现明文 API Key |  |  |
| 合法 Origin 通过 |  |  |
| 非法 Origin 被拒绝 |  |  |
| 停止 MCP Server 不影响原 `/v1/retrieve` |  |  |

## 13. 常见问题

### 13.1 返回 `Accept must contain both application/json and text/event-stream`

原因：请求头缺少：

```http
Accept: application/json, text/event-stream
```

解决：补上该请求头。

### 13.2 返回 `401 no bearer token`

原因：没有携带：

```http
Authorization: Bearer <RAG_API_KEY>
```

解决：检查 Agent 平台或网关是否透传 Authorization。

### 13.3 返回 `403`

可能原因：

- Origin 不在 `MCP_ALLOWED_ORIGINS`
- API Key 没有访问目标知识库权限
- 租户状态异常

### 13.4 返回 `Knowledge base not found`

可能原因：

- 知识库 ID 不存在
- 当前租户无权访问该知识库
- 跨租户访问被隐藏为 not_found

### 13.5 有效 key 也无法检索

检查：

- API Key 是否属于正确租户
- API Key 是否 active
- KB 是否 active
- `rag_tenant_kb_permission` 是否有对应授权
- `RAG_BASE_URL` 是否指向内部 `rag-server`
- `rag-server` 的 `/readyz` 是否正常

## 14. 当前建议结论

当前 MCP 功能可以进入真实场景测试。

建议测试顺序：

1. 先用本文档中的 PowerShell 命令验证 `/mcp` 基础协议可用。
2. 再用真实 API Key 调授权 KB，确认能返回结果。
3. 再接入真实 Agent 平台，看 Agent 是否能发现并调用 `retrieve_knowledge`。
4. 最后补齐安全场景：过期 key、吊销 key、未授权 KB、跨租户 KB、Origin 白名单、审计回查。

如果这些都通过，就可以认为 MCP Server 已具备进入内部试点的条件。
