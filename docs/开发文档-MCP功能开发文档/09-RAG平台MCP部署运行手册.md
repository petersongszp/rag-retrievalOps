# RAG 平台 MCP 部署运行手册

本文档用于说明 MCP Server 在第四阶段交付中的部署、客户端配置、排障和回滚方案，主要覆盖以下内容：

- Docker Compose 部署
- 本地 stdio 客户端配置
- HTTP MCP 客户端配置
- 常见问题排查
- 回滚方案

## 1. 适用范围与安全约束

- 共享的多租户部署场景应使用 HTTP MCP。
- 本地 stdio 仅用于单用户调试。
- 生产环境的 HTTP MCP 请求必须携带 `Authorization: Bearer <token>`。
- 生产环境的 HTTP MCP 应启用 `MCP_ALLOWED_ORIGINS` 来源校验。
- 不要通过工具参数传递租户身份或 API Key。

## 2. 环境变量

仓库根目录的 `.env.example` 已包含 Docker Compose 使用的 MCP 相关环境变量。

重要字段说明：

- `MCP_ENABLED`：MCP 相关配置的部署开关。
- `MCP_TRANSPORT`：传输方式。共享部署使用 `http`，本地调试才使用 `stdio`。
- `MCP_HOST`：HTTP 服务绑定地址，容器中通常为 `0.0.0.0`。
- `MCP_PORT`：HTTP MCP 监听端口，默认值为 `8898`。
- `MCP_ENDPOINT`：MCP 接口路径，默认值为 `/mcp`。
- `MCP_ALLOWED_ORIGINS`：允许访问的 Origin 白名单，多个值使用英文逗号分隔，用于浏览器或网关访问控制。
- `MCP_UPSTREAM_TIMEOUT_MS`：MCP 调用内部 `rag-server` 时的超时时间。
- `MCP_ENABLE_LEGACY_APP_ID`：是否启用旧版 App ID 兼容逻辑，新部署应保持为 `false`。
- `RAG_BASE_URL`：内部 RAG API 基础地址，Docker Compose 场景通常为 `http://rag-server:8899`。
- `RAG_ACCESS_TOKEN`：仅作为本地 stdio 调试时的兜底凭证。

## 3. Docker Compose 部署

根目录的 `docker-compose.yml` 已额外暴露 `rag-mcp-server` 服务。

推荐部署流程：

```bash
cp .env.example .env
docker compose build rag-mcp-server
docker compose up -d rag-server rag-mcp-server
docker compose ps rag-server rag-mcp-server
docker compose logs -f rag-mcp-server
```

预期部署拓扑：

- `rag-mcp-server` 是对外提供 MCP 能力的入口服务。
- `rag-server` 保持在 Docker 内部网络中。
- `rag-mcp-server` 通过 `http://rag-server:8899` 访问 `rag-server`。

生产环境建议：

- 仅发布客户端需要访问的 MCP 端口。
- TLS 终止和请求日志建议放在网关或 Ingress 层处理。
- 网关需要透传 `Authorization`、`Accept` 和 `MCP-Protocol-Version` 请求头。
- 明确配置 `MCP_ALLOWED_ORIGINS`，不要在生产环境中留空。

## 4. 本地 stdio 客户端示例

### Claude Desktop

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "command": "/absolute/path/to/rag-mcp-server",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_local_debug_token"
      }
    }
  }
}
```

### Cursor

```json
{
  "mcpServers": {
    "rag-retrieval": {
      "command": "/absolute/path/to/rag-mcp-server",
      "args": ["--transport", "stdio"],
      "env": {
        "RAG_BASE_URL": "http://localhost:8899",
        "RAG_ACCESS_TOKEN": "rag_local_debug_token"
      }
    }
  }
}
```

stdio 使用提醒：

- stdio 仅应在可信的本地机器上使用。
- `RAG_ACCESS_TOKEN` 在进程级别固定，不支持按请求切换租户。
- 如果需要并发多租户隔离，应切换到 HTTP MCP。

## 5. HTTP MCP 客户端示例

共享部署应使用带 Bearer 认证的 HTTP MCP。

```http
POST https://rag.example.com/mcp
Authorization: Bearer rag_xxx
Content-Type: application/json
Accept: application/json, text/event-stream
MCP-Protocol-Version: 2025-06-18
Origin: https://agent.example.com
```

连通性探测示例：

```bash
curl -i \
  -H "Authorization: Bearer rag_xxx" \
  -H "Accept: application/json, text/event-stream" \
  -H "MCP-Protocol-Version: 2025-06-18" \
  -H "Origin: https://agent.example.com" \
  http://localhost:8898/mcp
```

### 5.1 客户端快捷配置说明

以下 JSON 示例用于 Cursor、VS Code、Cline、Windsurf、Trae 等客户端快速接入 HTTP MCP。使用前需要替换：

- `https://rag.example.com/mcp`：替换为实际 MCP HTTP 地址。
- `Bearer 你的真实APIKey`：替换为后端 RAG 平台创建的真实 API Key，并保留 `Bearer ` 前缀。
- `https://agent.example.com`：替换为实际客户端 Origin，例如企业 Agent 域名、IDE 网关地址或本地测试地址。

### 5.2 Cursor：HTTP MCP

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

### 5.3 VS Code：HTTP MCP

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

### 5.4 VS Code / Cline：HTTP MCP

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

### 5.5 Windsurf：HTTP MCP

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

### 5.6 Trae / 通用 MCP Client：HTTP MCP

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

### 5.7 Continue / 其他 JSON 型客户端

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

## 6. 常见问题排查

### MCP 容器无法进入健康状态

检查项：

```bash
docker compose ps rag-mcp-server
docker compose logs --tail=200 rag-mcp-server
docker compose exec rag-mcp-server sh -lc 'env | sort | grep -E "^(MCP|RAG_)="'
```

可能原因：

- `MCP_TRANSPORT=http`，但当前运行的二进制文件尚未暴露 HTTP 监听或 `/healthz` 健康检查接口。
- `MCP_PORT` 与实际发布端口不一致。
- 进程启动时缺少必需环境变量，导致启动失败。

### MCP 无法访问 `rag-server`

检查项：

```bash
docker compose exec rag-mcp-server sh -lc 'curl -fsS http://rag-server:8899/healthz'
docker compose logs --tail=200 rag-server
```

可能原因：

- `rag-server` 未处于健康状态。
- `RAG_BASE_URL` 指向了错误的主机或端口。
- 两个服务不在同一个 Docker 网络中。

### HTTP 客户端收到 `401 unauthorized`

检查项：

- 确认请求中包含 `Authorization: Bearer <token>`。
- 确认 Token 未过期，也未被撤销。
- 确认网关正确转发了 `Authorization` 请求头。

### HTTP 客户端收到 `403 forbidden`

检查项：

- 确认 Token 对目标知识库有访问权限。
- 确认租户状态为启用状态。
- 如果启用了 Origin 校验，确认请求中的 `Origin` 已包含在 `MCP_ALLOWED_ORIGINS` 中。

### stdio 客户端启动后立即失败

检查项：

- 确认 `RAG_BASE_URL` 是完整的 `http://` 或 `https://` URL。
- 确认已设置 `RAG_ACCESS_TOKEN`。
- 直接运行一次二进制文件，查看标准错误输出：

```bash
RAG_BASE_URL=http://localhost:8899 RAG_ACCESS_TOKEN=rag_local_debug_token ./rag-mcp-server --transport stdio
```

## 7. 回滚方案

设计目标是：禁用 MCP 不应影响原有 RAG HTTP 检索链路。

回滚步骤：

1. 在网关或 Ingress 层移除或禁用 MCP 路由。
2. 仅停止 `rag-mcp-server` 服务。
3. 保持 `rag-server`、`mysql`、`redis` 和 `milvus` 继续运行。
4. 如有需要，回退 MCP 相关的 `.env` 覆盖配置。
5. 验证原有 RAG HTTP 检索链路仍可正常工作。

示例命令：

```bash
docker compose stop rag-mcp-server
docker compose rm -f rag-mcp-server
docker compose ps rag-server
curl -fsS http://localhost:8899/healthz
```

回滚验证项：

- `rag-server` 保持健康状态。
- 现有非 MCP 检索 API 仍可正常响应。
- 不再有新的 MCP 流量进入网关路由。
- 监控和告警恢复到接入 MCP 之前的基线状态。

## 8. 第四阶段手工验收清单

- `docker compose config` 能够成功渲染配置。
- `docker compose up -d rag-server rag-mcp-server` 能够启动部署。
- `rag-mcp-server` 能够在内部网络中解析并访问 `rag-server`。
- 选定的 MCP 健康检查能够通过。
- 新同事可以按照 stdio 示例完成本地连接。
- 共享 HTTP 客户端可以携带 `Authorization` 和 `Origin` 调用 MCP 端点。
