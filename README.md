# RAG Platform Admin Repository

这个仓库已经收敛为一套独立的 RAG 平台管理系统，只保留当前还在使用的两部分：

- `backend/`：RAG 后端服务，入口为 `backend/cmd/rag-server`
- `admin/`：RAG 管理后台，基于 Next.js

之前“面试吧”的业务前端、业务后端入口、支付/简历/面试相关接口与配套代码已经移除。

## 快速启动

项目根目录下的.env.example，复制粘贴，命名为新的文件.env
(.env文件不要同步到git仓库，避免团队协作环境冲突)

### 方式一：Docker Compose

```bash
docker-compose up -d --build
```

默认服务：

- RAG API: `http://localhost:8899`
- 管理后台: `http://localhost:3003`
- Attu: `http://localhost:8001`
- API 健康检查: `http://localhost:8899/healthz`

### 方式二：本地分开启动

先启动依赖：

```bash
docker-compose up -d mysql redis milvus attu
```

启动后端：

```bash
cd backend
go run ./cmd/rag-server
```

启动管理后台：

```bash
cd admin
npm install
npm run dev
```

## 目录结构

```text
admin/
  src/app/                     # 管理后台页面
  src/components/admin/        # RAG 管理功能组件

backend/
  cmd/rag-server/              # RAG 服务入口
  api/handler/kb/              # 知识库管理 API
  api/handler/rag/             # 公共检索 API
  api/ragrouter/               # RAG 专用路由注册
  internal/ragqueue/           # RAG 专用消息队列与入库消费
  internal/milvus/             # 向量检索、切分、评测
  internal/model/              # RAG 数据模型
  internal/service/kb/         # RAG 评测与知识库服务
```

## 常用命令

仓库根目录：

```bash
npm run dev
npm run build
npm run test
```

上面三个命令都会代理到 `admin/`。

后端：

```bash
cd backend
go build ./cmd/rag-server
```

## 配置说明

当前主要配置文件只有三类：

- 根目录 `.env`
- `backend/config.yaml`
- `admin/.env.local`

详细说明见 [docs/env-config-guide.md](docs/env-config-guide.md)。
