# Backend

`backend/` 现在承载 RAG 平台后端能力，并新增了 MCP Server 接入入口。

## 当前入口

- 运行入口：`cmd/rag-server`
- MCP 入口：`cmd/rag-mcp-server`
- Docker 构建：`Dockerfile.rag`
- MCP Docker 构建：`Dockerfile.mcp`

## 主要模块

- `api/handler/kb`：知识库管理、入库、日志、评测、策略、审计
- `api/handler/rag`：公共检索 API
- `api/ragrouter`：RAG 专用路由注册
- `internal/ragqueue`：知识入库队列与消费逻辑
- `internal/milvus`：检索、切分、索引、评测
- `internal/service/kb`：评测执行和知识库相关服务
- `internal/model`：RAG 平台数据表模型
- `internal/mcp`：MCP Server 配置、Tool、Handler 和 Transport

## 本地运行

```bash
go run ./cmd/rag-server
```

本地 stdio 调试 MCP Server：

```bash
RAG_BASE_URL=http://localhost:8899 RAG_ACCESS_TOKEN=rag_local_debug_token go run ./cmd/rag-mcp-server --transport stdio
```

## 构建

```bash
go build ./cmd/rag-server
go build ./cmd/rag-mcp-server
```

## MCP 文档

- 部署与运维 Runbook：`docs/mcp-deployment-runbook.md`
