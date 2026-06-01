# 环境配置说明

当前仓库只需要关注这三类配置：

- 根目录 `.env`
- `backend/config.yaml`
- `admin/.env.local`

旧的 `frontend/.env.local` 已经不再存在。

## 1. 配置关系

### 根目录 `.env`

放运行环境变量，主要给：

- `docker-compose.yml`
- `backend/cmd/rag-server`

### `backend/config.yaml`

放后端结构化配置，很多值会通过 `${ENV_NAME}` 读取 `.env`。

### `admin/.env.local`

管理后台本地开发只需要这个：

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8899/api
```

如果你走反向代理，也可以写成：

```env
NEXT_PUBLIC_API_BASE_URL=/api
```

## 2. 推荐启动顺序

1. 复制 `.env.example` 为 `.env`
2. 补齐数据库、Redis、Milvus、LLM 相关变量
3. 新建 `admin/.env.local`
4. 启动后端和管理后台

## 3. 最小可运行配置

### 根目录 `.env`

```env
APP_ENV=dev

DB_HOST=localhost
DB_PORT=3308
DB_USER=root
DB_PASSWORD=root
DB_NAME=interview_agent

REDIS_HOST=localhost
REDIS_PORT=6380
REDIS_PASSWORD=root

LLM_API_KEY=your-key
LLM_BASE_URL=https://api.openai.com/v1
LLM_MODEL_NAME=gpt-4o
LLM_PROVIDER_NAME=OpenAI

MILVUS_ADDRESS=localhost:19531
RAG_ENABLED=true
```

### `admin/.env.local`

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8899/api
```

## 4. 本地开发命令

启动依赖：

```bash
docker-compose up -d mysql redis milvus
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

## 5. 排查建议

- 管理后台请求错地址：先确认 `admin/.env.local` 里的 `NEXT_PUBLIC_API_BASE_URL`
- 后端起不来：先确认 `.env` 和 `backend/config.yaml` 里数据库、Redis、Milvus 配置一致
- Docker 能跑但本地不能跑：优先检查端口是否和 `docker-compose.yml` 的映射一致
