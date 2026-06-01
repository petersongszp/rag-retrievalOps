# Backend

`backend/` 现在只承载 RAG 平台后端能力，不再包含旧的面试业务 API。

## 当前入口

- 运行入口：`cmd/rag-server`
- Docker 构建：`Dockerfile.rag`

## 主要模块

- `api/handler/kb`：知识库管理、入库、日志、评测、策略、审计
- `api/handler/rag`：公共检索 API
- `api/ragrouter`：RAG 专用路由注册
- `internal/ragqueue`：知识入库队列与消费逻辑
- `internal/milvus`：检索、切分、索引、评测
- `internal/service/kb`：评测执行和知识库相关服务
- `internal/model`：RAG 平台数据表模型

## 本地运行

```bash
go run ./cmd/rag-server
```

## 构建

```bash
go build ./cmd/rag-server
```
