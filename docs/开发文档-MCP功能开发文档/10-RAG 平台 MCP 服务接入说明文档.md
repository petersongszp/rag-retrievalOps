# RAG 平台 MCP Agent 客户接入文档

本文档面向需要把企业 Agent、AI IDE 或桌面 MCP Client 接入 RAG 平台知识库检索能力的客户和交付人员。

通过本文档，客户可以完成以下接入：

- 远程 HTTP MCP 接入，适合生产、多租户、企业 Agent 平台。
- 本地 stdio MCP 接入，适合单用户本地调试和 IDE 验证。
- 常见 AI IDE / MCP Client 的可复制 JSON 配置。
- `retrieve_knowledge` 工具调用参数说明和排障说明。

## 1. MCP 能力概览

RAG 平台 MCP Server 会把平台检索能力包装为 MCP Tool，供 Agent 通过标准 MCP 协议发现和调用。

当前对外工具名称为：

```text
retrieve_knowledge
```

工具用途：

```text
根据用户问题，在授权知识库范围内检索可引用的知识证据，返回结构化片段供 Agent 生成答案或展示引用。
```

典型调用链路如下：

```text
AI IDE / Agent / MCP Client
        │
        │ MCP 协议调用 retrieve_knowledge
        ▼
RAG MCP Server
        │
        │ POST /v1/retrieve
        │ Authorization: Bearer <客户 API Key 或 JWT>
        ▼
RAG Server
        │
        ├─ 凭证校验
        ├─ 租户识别
        ├─ 知识库权限校验
        ├─ 检索执行
        └─ 审计记录
```

## 2. 接入方式选择

| 接入方式 | 推荐场景 | 是否适合生产 | 是否支持多租户并发 | 鉴权方式 |
|---|---|---:|---:|---|
| Streamable HTTP MCP | 企业 Agent、云端 IDE、统一网关接入 | 是 | 是 | HTTP `Authorization` 请求头 |
| stdio MCP | 本地 IDE、桌面客户端、单用户调试 | 否 | 否 | 本地环境变量 `RAG_ACCESS_TOKEN` |
| SSE MCP | 旧客户端兼容 | 视实际部署而定 | 视实际部署而定 | 通常仍走 HTTP `Authorization` |

建议：

- 生产环境、共享环境、多租户环境统一使用 HTTP MCP。
- 本地调试、单人验证、Claude Desktop / Cursor 本地运行可使用 stdio MCP。
- 如果客户端明确只支持 SSE，需要先确认当前部署是否额外提供 SSE 兼容入口；当前主路径是 Streamable HTTP。

## 3. 接入前准备

接入前请向平台管理员获取以下信息：

| 信息 | 示例 | 说明 |
|---|---|---|
| MCP HTTP 地址 | `https://rag.example.com/mcp` | HTTP MCP 入口，由平台管理员提供 |
| API Key 或 JWT | `你的真实APIKey` | 后端 RAG 平台创建的访问凭证 |
| 知识库 ID | `1001`、`1002` | 已授权可检索的知识库 |
| 允许的 Origin | `https://agent.example.com` | 客户端来源地址，浏览器、Web IDE 或网关访问时需要 |
| 本地二进制路径 | `D:\tools\rag-mcp-server.exe` | stdio 接入时需要 |

权限要求：

- API Key / JWT 必须有效、未过期、未吊销。
- API Key / JWT 所属租户必须有目标知识库读取权限。
- 不允许跨租户访问未授权知识库。
- 不要把 API Key 写入工具参数，应放在 HTTP header 或本地环境变量中。

API Key 填写说明：

- 客户只需要把示例中的 `你的真实APIKey` 替换成后端 RAG 平台创建的真实 API Key。
- 完整格式必须保留 `Bearer ` 前缀，例如 `Bearer sk_live_xxxxxxxxx`。
- 不要只填写 API Key 本身，也不要删除 `Bearer`。
- MCP Server 会把这个凭证转发给 RAG Server，RAG Server 负责完成租户识别、知识库权限校验和审计记录。

Origin 填写说明：

- `Origin` 表示“这个 MCP 请求来自哪个客户端页面、IDE 或网关”。
- 如果 MCP Server 开启了来源白名单，`Origin` 必须在服务端 `MCP_ALLOWED_ORIGINS` 配置中。
- 如果客户通过公司网关、Web Agent 或浏览器访问，通常填写实际访问域名，例如 `https://agent.example.com`。
- 如果客户在本地测试，可填写本地客户端地址，例如 `http://localhost:3000`。
- 如果客户端不支持配置 `Origin`，需要确认服务端没有开启 `MCP_REQUIRE_ORIGIN_HEADER=true`，否则请求可能被拒绝。

## 4. HTTP MCP 接入

HTTP MCP 是生产推荐方式。

### 4.1 服务地址

```text
https://rag.example.com/mcp
```

如果是本地或测试环境，可能是：

```text
http://localhost:8898/mcp
```

### 4.2 必要请求头

HTTP MCP 请求需要携带以下请求头：

```http
Authorization: Bearer 你的真实APIKey
Content-Type: application/json
Accept: application/json, text/event-stream
MCP-Protocol-Version: 2025-06-18
Origin: https://agent.example.com
```

说明：

- `Authorization`：客户 API Key 或 JWT，生产环境必须提供。只需要把 `你的真实APIKey` 替换成后端创建的真实 API Key，并保留 `Bearer ` 前缀。
- `Accept`：建议同时包含 `application/json` 和 `text/event-stream`，兼容 MCP 客户端行为，通常不需要修改。
- `MCP-Protocol-Version`：建议使用 `2025-06-18`，通常不需要修改。
- `Origin`：请求来源地址。当平台启用 Origin 白名单时，必须与服务端配置匹配；客户需要根据自己的 Agent 域名、IDE 网关地址或本地测试地址修改。

### 4.3 HTTP JSON-RPC 探测示例

可以用以下请求验证 `initialize` 是否可用：

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "capabilities": {},
    "clientInfo": {
      "name": "customer-agent",
      "version": "1.0.0"
    }
  }
}
```

调用 `tools/list` 查看工具：

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list",
  "params": {}
}
```

调用 `retrieve_knowledge` 检索知识库：

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "retrieve_knowledge",
    "arguments": {
      "query": "请介绍公司报销流程中发票提交的要求",
      "kb_ids": [1001, 1002],
      "top_k": 5
    }
  }
}
```

## 5. stdio MCP 接入

stdio 适合本地单用户调试，不建议用于生产共享环境。

### 5.1 启动要求

本地启动时需要设置：

```env
RAG_BASE_URL=http://localhost:8899
RAG_ACCESS_TOKEN=rag_local_debug_token
```

说明：

- `RAG_BASE_URL` 是 RAG Server 地址。
- `RAG_ACCESS_TOKEN` 是本地固定使用的 Bearer 凭证。
- stdio 模式不支持每次请求切换 token。
- stdio 模式不支持多租户共享并发隔离。

### 5.2 Windows 本地命令示例

```powershell
$env:RAG_BASE_URL="http://localhost:8899"
$env:RAG_ACCESS_TOKEN="rag_local_debug_token"
D:\tools\rag-mcp-server.exe --transport stdio
```

### 5.3 macOS / Linux 本地命令示例

```bash
RAG_BASE_URL=http://localhost:8899 \
RAG_ACCESS_TOKEN=rag_local_debug_token \
/opt/rag-mcp-server --transport stdio
```

## 6. 常见客户端 JSON 配置

以下 JSON 示例中的占位符需要替换：

| 占位符 | 替换说明 |
|---|---|
| `https://rag.example.com/mcp` | 实际 MCP HTTP 地址，客户需要替换成平台管理员提供的 MCP 地址 |
| `你的真实APIKey` | 后端 RAG 平台创建的真实 API Key，只替换这几个中文字，不要删除前面的 `Bearer ` |
| `https://agent.example.com` | 实际客户端 Origin，客户需要替换成自己的 Agent 域名、IDE 网关地址或本地测试地址 |
| `D:\\tools\\rag-mcp-server.exe` | Windows 本地 MCP Server 路径 |
| `/opt/rag-mcp-server` | macOS / Linux 本地 MCP Server 路径 |
| `1001` | 已授权知识库 ID |

### 6.1 Cursor：HTTP MCP

适用于支持远程 MCP Server 的 Cursor 版本，可放入项目级 `.cursor/mcp.json` 或用户级 MCP 配置中。

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "url": "https://rag.example.com/mcp",
      "headers": {
        "Authorization": "Bearer 你的真实APIKey",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18",
        "Origin": "https://agent.example.com"
      }
    }
  }
}
```

如果当前 Cursor 版本要求显式声明类型，可使用：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "type": "streamable-http",
      "url": "https://rag.example.com/mcp",
      "headers": {
        "Authorization": "Bearer 你的真实APIKey",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18",
        "Origin": "https://agent.example.com"
      }
    }
  }
}
```

### 6.2 Cursor：本地 stdio

Windows 示例：

```json
{
  "mcpServers": {
    "rag-retrieval-local": {
      "command": "D:\\tools\\rag-mcp-server.exe",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_local_debug_token"
      }
    }
  }
}
```

macOS / Linux 示例：

```json
{
  "mcpServers": {
    "rag-retrieval-local": {
      "command": "/opt/rag-mcp-server",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_local_debug_token"
      }
    }
  }
}
```

### 6.3 Claude Desktop：本地 stdio

Claude Desktop 常用 stdio 方式接入本地 MCP Server。

Windows 示例：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "command": "D:\\tools\\rag-mcp-server.exe",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_local_debug_token"
      }
    }
  }
}
```

macOS 示例：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "command": "/opt/rag-mcp-server",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_local_debug_token"
      }
    }
  }
}
```

### 6.4 VS Code：HTTP MCP

适用于支持 MCP 的 VS Code 扩展或内置 MCP 配置。可放在工作区 `.vscode/mcp.json` 中，具体字段以实际客户端版本为准。

```json
{
  "servers": {
    "rag-retrieval": {
      "type": "http",
      "url": "https://rag.example.com/mcp",
      "headers": {
        "Authorization": "Bearer 你的真实APIKey",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18",
        "Origin": "https://agent.example.com"
      }
    }
  }
}
```

如果扩展要求使用 `mcpServers` 字段，可使用：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "type": "streamable-http",
      "url": "https://rag.example.com/mcp",
      "headers": {
        "Authorization": "Bearer 你的真实APIKey",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18",
        "Origin": "https://agent.example.com"
      }
    }
  }
}
```

### 6.5 VS Code / Cline：HTTP MCP

Cline 类客户端通常使用 `mcpServers` 配置。

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "url": "https://rag.example.com/mcp",
      "headers": {
        "Authorization": "Bearer 你的真实APIKey",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18",
        "Origin": "https://agent.example.com"
      },
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

本地 stdio 示例：

```json
{
  "mcpServers": {
    "rag-retrieval-local": {
      "command": "D:\\tools\\rag-mcp-server.exe",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_local_debug_token"
      },
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

### 6.6 Windsurf：HTTP MCP

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "serverUrl": "https://rag.example.com/mcp",
      "headers": {
        "Authorization": "Bearer 你的真实APIKey",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18",
        "Origin": "https://agent.example.com"
      }
    }
  }
}
```

如果当前客户端版本使用 `url` 字段，可改为：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "url": "https://rag.example.com/mcp",
      "headers": {
        "Authorization": "Bearer 你的真实APIKey",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18",
        "Origin": "https://agent.example.com"
      }
    }
  }
}
```

### 6.7 Windsurf：本地 stdio

```json
{
  "mcpServers": {
    "rag-retrieval-local": {
      "command": "D:\\tools\\rag-mcp-server.exe",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_local_debug_token"
      }
    }
  }
}
```

### 6.8 Trae / 通用 MCP Client：HTTP MCP

如果客户端支持 `mcpServers` 标准结构，可直接使用以下配置：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "type": "streamable-http",
      "url": "https://rag.example.com/mcp",
      "headers": {
        "Authorization": "Bearer 你的真实APIKey",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18",
        "Origin": "https://agent.example.com"
      }
    }
  }
}
```

Trae 配置中通常需要客户改动：

- `url`：改成平台管理员提供的 MCP HTTP 地址。
- `Authorization`：只替换 `你的真实APIKey`，保留 `Bearer ` 前缀。
- `Origin`：改成客户自己的 Agent 域名、IDE 网关地址或本地测试地址；如果服务端未强制 Origin，也可以按实际客户端能力处理。

如果客户端只支持 stdio，可使用：

```json
{
  "mcpServers": {
    "rag-retrieval-local": {
      "command": "D:\\tools\\rag-mcp-server.exe",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "https://rag-api.example.com",
        "RAG_ACCESS_TOKEN": "你的真实APIKey"
      }
    }
  }
}
```

### 6.9 Continue / 其他 JSON 型客户端

如果客户端要求 MCP 配置为数组结构，可参考以下写法：

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "name": "rag-retrieval",
        "transport": {
          "type": "streamable-http",
          "url": "https://rag.example.com/mcp",
          "headers": {
            "Authorization": "Bearer 你的真实APIKey",
            "Accept": "application/json, text/event-stream",
            "MCP-Protocol-Version": "2025-06-18",
            "Origin": "https://agent.example.com"
          }
        }
      }
    ]
  }
}
```

如果客户端不识别该格式，请优先查找其 MCP 配置入口中是否支持以下字段：

```json
{
  "name": "rag-retrieval",
  "type": "streamable-http",
  "url": "https://rag.example.com/mcp",
  "headers": {
    "Authorization": "Bearer 你的真实APIKey",
    "Accept": "application/json, text/event-stream",
    "MCP-Protocol-Version": "2025-06-18",
    "Origin": "https://agent.example.com"
  }
}
```

## 7. 企业 Agent 平台接入

企业自研 Agent 平台推荐使用 HTTP MCP。

### 7.1 服务配置

```json
{
  "name": "rag-retrieval",
  "transport": "streamable-http",
  "endpoint": "https://rag.example.com/mcp",
  "headers": {
    "Authorization": "Bearer 你的真实APIKey",
    "Accept": "application/json, text/event-stream",
    "MCP-Protocol-Version": "2025-06-18",
    "Origin": "https://agent.example.com"
  },
  "tools": [
    "retrieve_knowledge"
  ]
}
```

### 7.2 Agent 工具使用建议

建议在 Agent 的工具说明或系统提示词中加入：

```text
当用户问题需要基于企业知识库、制度、产品文档、项目资料或内部 FAQ 回答时，优先调用 retrieve_knowledge。调用时必须传入用户问题 query，并传入当前业务允许访问的 kb_ids。回答时应基于检索结果组织答案，必要时展示来源文件、知识库 ID 或 request_id。
```

### 7.3 推荐调用参数

```json
{
  "query": "用户提出的具体问题",
  "kb_ids": [1001, 1002],
  "top_k": 5
}
```

不建议：

```json
{
  "query": "用户提出的具体问题",
  "api_key": "不要这样填写APIKey",
  "tenant_id": "tenant-a",
  "user_id": "user-1"
}
```

原因：身份信息和凭证必须由平台从 `Authorization` 中解析，不能由工具参数传入。

## 8. retrieve_knowledge 工具参数

### 8.1 参数说明

| 参数 | 类型 | 是否必填 | 建议值 | 说明 |
|---|---|---:|---|---|
| `query` | string | 是 | 用户原始问题或改写后的检索问题 | 最大 2000 字符 |
| `kb_ids` | integer array | 推荐 | `[1001, 1002]` | 知识库 ID 列表，推荐使用 |
| `kb_id` | integer | 否 | `1001` | 单知识库兼容字段 |
| `top_k` | integer | 否 | `5` | 范围 1 到 20，默认 5 |
| `strategy_profile` | string | 否 | 不填 | V1 预留字段 |
| `metadata_filter` | object | 否 | 不填 | V1 预留字段 |

### 8.2 最小调用参数

```json
{
  "query": "公司差旅报销需要哪些材料？",
  "kb_ids": [1001]
}
```

### 8.3 多知识库调用参数

```json
{
  "query": "产品 A 的安装步骤和常见报错如何处理？",
  "kb_ids": [1001, 1002, 1003],
  "top_k": 8
}
```

### 8.4 带兼容字段的调用参数

```json
{
  "query": "如何申请 VPN 权限？",
  "kb_id": 1001,
  "top_k": 5
}
```

## 9. 返回结果说明

工具返回内容通常包含可读文本和结构化数据。

可读文本示例：

```text
Retrieved 2 item(s).
request_id: 9f5d2b1c

[1] score=0.9123 source=expense-policy.md kb_id=1001 document_id=12 chunk_index=3
发票提交需要包含发票原件、费用明细和审批单...

[2] score=0.8731 source=travel-policy.md kb_id=1001 document_id=15 chunk_index=7
差旅报销应在出差结束后 30 天内提交...
```

结构化字段示例：

```json
{
  "request_id": "9f5d2b1c",
  "items": [
    {
      "content": "发票提交需要包含发票原件、费用明细和审批单...",
      "score": 0.9123,
      "citation": {
        "kb_id": 1001,
        "document_id": 12,
        "chunk_id": "chunk-3",
        "file_name": "expense-policy.md",
        "chunk_index": 3
      },
      "source": {
        "route": "dense",
        "collection": "kb_collection",
        "retriever_version": "hybrid-v1"
      }
    }
  ]
}
```

建议 Agent 回答时：

- 优先基于 `items[].content` 组织答案。
- 涉及制度、合同、技术文档时，展示 `citation.file_name` 或 `document_id`。
- 排障或审计时保留 `request_id`。
- 检索结果不足时，不要编造答案，应提示未找到足够依据。

## 10. 服务端配置参考

如果客户需要自部署 MCP Server，可参考以下环境变量。

### 10.1 生产 HTTP 部署

```env
APP_ENV=prod
MCP_ENABLED=true
MCP_TRANSPORT=http
MCP_HOST=0.0.0.0
MCP_PORT=8898
MCP_ENDPOINT=/mcp
MCP_ALLOWED_ORIGINS=https://agent.example.com,https://admin.example.com
MCP_REQUIRE_ORIGIN_HEADER=false
MCP_UPSTREAM_TIMEOUT_MS=5000
MCP_SESSION_TIMEOUT_MS=300000
MCP_DISABLE_HTTP_AUTH=false
MCP_DISABLE_METRICS=false
MCP_ENABLE_LEGACY_APP_ID=false
RAG_BASE_URL=http://rag-server:8899
```

### 10.2 本地 HTTP 调试

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

### 10.3 本地 stdio 调试

```env
APP_ENV=dev
MCP_ENABLED=true
MCP_TRANSPORT=stdio
RAG_BASE_URL=http://localhost:8899
RAG_ACCESS_TOKEN=rag_local_debug_token
```

## 11. 安全要求

客户接入时必须遵守：

1. 生产环境使用 HTTP MCP，不使用 stdio 作为共享入口。
2. 生产 HTTP MCP 必须携带 `Authorization: Bearer <token>`。
3. API Key / JWT 不要写入工具参数、用户问题或对话上下文。
4. 不要在客户端日志中打印明文 API Key / JWT。
5. 不要把 `tenant_id`、`user_id`、`api_key_id` 等身份字段作为工具参数传入。
6. 生产环境应配置 `MCP_ALLOWED_ORIGINS`。
7. 生产环境不要设置 `MCP_DISABLE_HTTP_AUTH=true`。
8. `RAG_BASE_URL` 应指向内网 RAG 服务地址。

## 12. 常见问题排查

### 12.1 客户端看不到 `retrieve_knowledge`

检查项：

- MCP Server 地址是否正确。
- 客户端配置是否保存并重启。
- HTTP 客户端是否能访问 `/mcp`。
- stdio 的 `command` 是否为绝对路径。
- stdio 进程是否因为缺少 `RAG_BASE_URL` 或 `RAG_ACCESS_TOKEN` 启动失败。

### 12.2 HTTP 返回 401

常见原因：

- 未携带 `Authorization` 请求头。
- `Authorization` 格式不是 `Bearer <token>`。
- API Key / JWT 无效、过期或已吊销。
- 网关没有透传 `Authorization` 请求头。

处理建议：

```json
{
  "headers": {
    "Authorization": "Bearer 你的真实APIKey"
  }
}
```

### 12.3 HTTP 返回 403

常见原因：

- 当前凭证没有目标知识库权限。
- 访问了其他租户的知识库。
- 租户被禁用。
- 请求 `Origin` 不在白名单中。

处理建议：

- 确认 `kb_ids` 是否属于当前客户租户。
- 联系平台管理员确认知识库授权。
- 确认 `Origin` 已加入 `MCP_ALLOWED_ORIGINS`。

### 12.4 工具返回 `invalid_request`

常见原因：

- `query` 为空。
- 未传 `kb_id` 或 `kb_ids`。
- `top_k` 小于 1 或大于 20。
- `metadata_filter` 太大或嵌套过深。

正确示例：

```json
{
  "query": "请说明合同审批流程",
  "kb_ids": [1001],
  "top_k": 5
}
```

### 12.5 检索结果为空

可能原因：

- 知识库中没有相关内容。
- `query` 太短或语义不明确。
- 选择了错误的知识库 ID。
- 当前凭证没有访问真实目标知识库。

建议：

- 改写 query，让问题更具体。
- 增加相关知识库 ID。
- 适当提高 `top_k`，例如从 5 调整到 10。
- 联系管理员确认知识库内容是否已完成索引。

### 12.6 stdio 启动失败

检查项：

- `command` 是否为真实存在的绝对路径。
- Windows JSON 路径是否使用双反斜杠，例如 `D:\\tools\\rag-mcp-server.exe`。
- 是否设置 `RAG_BASE_URL`。
- 是否设置 `RAG_ACCESS_TOKEN`。
- 本地 RAG Server 是否可访问。

## 13. 客户验收清单

接入完成后，建议按以下清单验收：

- [ ] 客户端可以发现 `retrieve_knowledge` 工具。
- [ ] 使用有效 API Key / JWT 可以检索授权知识库。
- [ ] 传入未授权知识库时会被拒绝。
- [ ] 不传 Authorization 时 HTTP 调用失败。
- [ ] 返回结果包含内容片段和来源信息。
- [ ] 返回结果包含或可追踪 `request_id`。
- [ ] Agent 回答不会编造未检索到的内容。
- [ ] 客户端配置中没有把 API Key 写入工具参数。
- [ ] 生产环境 Origin 白名单已配置。
- [ ] 网关已透传 `Authorization`、`Accept`、`MCP-Protocol-Version`。

## 14. 交付给客户的最小配置包

如果客户只需要最快完成远程接入，可交付以下信息：

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "type": "streamable-http",
      "url": "https://rag.example.com/mcp",
      "headers": {
        "Authorization": "Bearer 你的真实APIKey",
        "Accept": "application/json, text/event-stream",
        "MCP-Protocol-Version": "2025-06-18",
        "Origin": "https://agent.example.com"
      }
    }
  }
}
```

客户需要修改的地方：

- `url`：替换为实际 MCP HTTP 地址。
- `Authorization`：替换 `你的真实APIKey` 为后端创建的 API Key，保留 `Bearer `。
- `Origin`：替换为客户自己的 Agent 域名、IDE 网关地址或本地测试地址。

同时告知客户可用知识库：

```json
{
  "kb_ids": [1001, 1002],
  "tool": "retrieve_knowledge"
}
```

推荐测试问题：

```json
{
  "query": "请根据知识库说明当前业务流程的关键步骤",
  "kb_ids": [1001],
  "top_k": 5
}
```
