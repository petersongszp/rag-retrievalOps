# AgentDesk + RetrievalOps 本地部署

## 当前连接方式

- AgentDesk 负责客服会话、工作流、工单和转人工。
- RetrievalOps 负责文档入库、Milvus 检索、融合排序、证据门禁和审计。
- AgentDesk 通过 `POST http://rag-server:8899/v1/retrieve` 获取证据。
- 两套系统不共享业务数据库，也不需要把同一份文档重复写入 Qdrant 和 Milvus。

本地映射目前为：

| AgentDesk 知识库 | RetrievalOps 知识库 |
| --- | --- |
| `1` (`RetrievalOps / agent面试`) | `1` (`agent面试`) |

映射配置位于 `integrations/agent-desk/docker/agent-desk.yaml` 的
`retrieval.knowledgeBaseMap`。

## 服务入口

- AgentDesk 管理后台：<http://localhost:8083/dashboard>
- AgentDesk 客服工作台：<http://localhost:8083/dashboard/conversations>
- AgentDesk 客户聊天页：<http://localhost:8083/support/chat>
- RetrievalOps 管理后台：<http://localhost:3003>
- RetrievalOps API：<http://localhost:8899>

AgentDesk 初始管理员为 `admin` / `ChangeMe123!`。本地首次登录后也应修改密码。

## 常用命令

首次启动前创建本地环境文件，并填写 RetrievalOps API Key 与随机会话密钥：

```powershell
Copy-Item .env.agent-desk.example .env.agent-desk.local
```

构建 AgentDesk：

```powershell
docker compose -f docker-compose.yml -f docker-compose.agent-desk.yml build agent-desk
```

启动或恢复服务：

```powershell
docker compose -f docker-compose.yml -f docker-compose.agent-desk.yml up -d --no-build agent-desk
```

查看状态：

```powershell
docker compose -f docker-compose.yml -f docker-compose.agent-desk.yml ps agent-desk rag-server
```

只停止客服侧服务：

```powershell
docker compose -f docker-compose.yml -f docker-compose.agent-desk.yml stop agent-desk
```

## 密钥与数据

- RetrievalOps 专用 API Key 和 AgentDesk 会话密钥保存在根目录
  `.env.agent-desk.local`。
- 该文件已加入 `.gitignore`，不要提交或复制到公开位置。
- AgentDesk 客服数据保存在 Docker 卷 `rag-platform_agent-desk-data`。
- 文档仍从 RetrievalOps 管理后台上传；AgentDesk 的本地知识库只承担 ID 映射和工作流选择。

## 完整客服回复

检索链路已经可用。要让客户聊天页生成自然语言回复，还需要在 AgentDesk 的
“AI 配置”中添加一个 OpenAI-compatible 聊天模型，并把 AI Agent、工作流和渠道关联到
`RetrievalOps / agent面试` 知识库。Embedding 模型不能直接充当聊天模型。

当 RetrievalOps 返回 `evidence_gate_result=refused` 时，适配器会向 AgentDesk 返回空证据，
Answerability Gate 会进入拒答、澄清或转人工分支。网络错误在 `failClosed=true` 时也按空证据处理。
