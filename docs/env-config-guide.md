# 环境配置详细说明

这份文档用于说明本项目里各类 `env` / `config` 配置的来源、作用、填写方式和常见坑，方便后续开发和学习同学快速上手。

适用范围：

- 项目根目录 `.env`
- 后端 `backend/config.yaml`
- 前端 `frontend/.env.local`
- 管理后台 `admin/.env.local`
- 可选模型网关 `litellm/config.yaml`

## 1. 先理解这几个配置文件分别干什么

项目不是只有一个 `.env`，而是几层配置一起工作：

| 文件 | 作用 | 谁会读取 |
| --- | --- | --- |
| 根目录 `.env` | 项目主环境变量入口，放数据库、Redis、LLM、OAuth、RAG 等变量 | Docker Compose、后端服务 |
| `backend/config.yaml` | 后端结构化配置，里面大量字段会引用 `.env` 变量 | 后端服务 |
| `frontend/.env.local` | 前端本地开发环境变量 | 前端 Next.js |
| `admin/.env.local` | 管理后台本地开发环境变量 | Admin Next.js |
| `litellm/config.yaml` | LiteLLM 自身配置文件 | LiteLLM 服务 |

可以把它理解成：

1. 根目录 `.env` 负责放“真实值”
2. `backend/config.yaml` 负责定义“后端有哪些配置项”
3. 前端和 Admin 各自只关心自己的接口地址

## 2. 配置加载关系和优先级

### 2.1 后端怎么加载配置

后端启动入口在 [backend/cmd/server/main.go](/d:/Bear/mianshiba-eino-overseas/backend/cmd/server/main.go)。

后端启动时会做两件事：

1. 自动查找并加载项目根目录 `.env`
2. 再读取 [backend/config.yaml](/d:/Bear/mianshiba-eino-overseas/backend/config.yaml)

其中 `backend/config.yaml` 里的很多值不是写死的，而是这种形式：

```yaml
llm:
  api_key: '${LLM_API_KEY}'
```

也就是说，真正的值来自根目录 `.env`。

### 2.2 哪些配置优先级更高

对后端来说，优先级大致是：

1. 环境变量
2. `backend/config.yaml` 中写的值
3. 代码默认值

但要注意一个细节：

- 大部分配置项是 `config.yaml` 引用 `.env` 变量来取值
- `RAG_*` 这一组除了会被 `config.yaml` 读取，还会被代码再次用环境变量覆盖

也就是说，`RAG_*` 配置最容易出现“你改了 YAML，但实际运行又被环境变量覆盖”的情况。

### 2.3 前端和 Admin 怎么加载配置

前端和 Admin 都只需要一个变量：

- `NEXT_PUBLIC_API_BASE_URL`

它决定浏览器请求后端 API 的地址。

本地开发通常填：

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8899/api
```

Docker / Nginx 代理场景通常填：

```env
NEXT_PUBLIC_API_BASE_URL=/api
```

## 3. 新同学最推荐的配置顺序

建议按这个顺序配，不容易乱：

1. 复制根目录 [.env.example](/d:/Bear/mianshiba-eino-overseas/.env.example) 为 `.env`
2. 先填“最小可运行配置”：数据库、Redis、LLM
3. 前端新建 `frontend/.env.local`
4. Admin 新建 `admin/.env.local`
5. 如果暂时不用 OAuth、RAG、支付、ASR，就先留空或关闭

## 4. 最小可运行配置

如果你只是想先把项目跑起来，本地至少优先确认下面这些配置。

### 4.1 根目录 `.env` 最小必填

```env
DB_USER=root
DB_PASSWORD=root
DB_HOST=localhost
DB_PORT=3307
DB_NAME=interview_agent

REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=root

LLM_API_KEY=你的模型Key
LLM_BASE_URL=https://api.openai.com/v1
LLM_MODEL_NAME=gpt-4o
LLM_PROVIDER_NAME=OpenAI

APP_ENV=dev
RAG_ENABLED=false
```

### 4.2 前端最小必填

文件：`frontend/.env.local`

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8899/api
```

### 4.3 Admin 最小必填

文件：`admin/.env.local`

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8899/api
```

## 5. 根目录 `.env` 逐项说明

下面按模块说明这些变量怎么填、有什么作用。

### 5.1 数据库 MySQL

| 变量 | 是否必填 | 示例 | 作用 | 怎么填 |
| --- | --- | --- | --- | --- |
| `DB_USER` | 是 | `root` | MySQL 用户名 | 和你的 MySQL 用户保持一致 |
| `DB_PASSWORD` | 是 | `root` | MySQL 密码 | 和你的 MySQL 密码保持一致 |
| `DB_HOST` | 是 | `localhost` | MySQL 主机地址 | 本地直连填 `localhost`，Docker 内通常会被覆盖成 `mysql` |
| `DB_PORT` | 是 | `3307` | MySQL 端口 | 本地用项目 docker-compose 时默认映射到 `3307` |
| `DB_NAME` | 是 | `interview_agent` | 业务数据库名 | 要和初始化数据库名称一致 |

补充说明：

- 根目录 `docker-compose.yml` 把宿主机 `3307` 映射到容器 `3306`
- 所以本地直连经常是 `3307`
- Docker 容器内部服务互相访问时会改成 `mysql:3306`

### 5.2 Redis

| 变量 | 是否必填 | 示例 | 作用 | 怎么填 |
| --- | --- | --- | --- | --- |
| `REDIS_HOST` | 是 | `localhost` | Redis 地址 | 本地填 `localhost` |
| `REDIS_PORT` | 是 | `6379` | Redis 端口 | 默认 `6379` |
| `REDIS_PASSWORD` | 是 | `root` | Redis 密码 | 要与 docker-compose 启动参数一致 |

补充说明：

- 当前 `docker-compose.yml` 里 Redis 启动命令带了 `--requirepass`
- 所以如果你本地用的是项目提供的 Redis，密码不能漏填

### 5.3 OAuth 登录

#### GitHub OAuth

| 变量 | 何时必填 | 示例 | 作用 | 填写建议 |
| --- | --- | --- | --- | --- |
| `GITHUB_CLIENT_ID` | 开启 GitHub 登录时 | `Iv1.xxxxxx` | GitHub OAuth 应用 ID | 去 GitHub Developer Settings 创建 OAuth App 获取 |
| `GITHUB_CLIENT_SECRET` | 开启 GitHub 登录时 | `xxxxxx` | GitHub OAuth 密钥 | 不要提交到仓库 |
| `GITHUB_REDIRECT_URL` | 开启 GitHub 登录时 | `http://localhost:3000/github/callback` | GitHub 登录回调地址 | 必须和 GitHub 后台配置完全一致 |

#### Google OAuth

| 变量 | 何时必填 | 示例 | 作用 | 填写建议 |
| --- | --- | --- | --- | --- |
| `GOOGLE_CLIENT_ID` | 开启 Google 登录时 | `xxxx.apps.googleusercontent.com` | Google OAuth 应用 ID | 去 Google Cloud Console 创建 |
| `GOOGLE_CLIENT_SECRET` | 开启 Google 登录时 | `xxxxxx` | Google OAuth 密钥 | 不要提交到仓库 |
| `GOOGLE_REDIRECT_URL` | 开启 Google 登录时 | `http://localhost:3000/google/callback` | Google 登录回调地址 | 必须和 Google 后台授权回调一致 |

常见坑：

- 回调地址必须完全一致，包含协议、域名、端口、路径
- 本地前端跑在 `3000`，就不要误填成 `8899`

### 5.4 LLM 主模型配置

这是后端最核心的一组配置。

| 变量 | 是否必填 | 示例 | 作用 | 怎么填 |
| --- | --- | --- | --- | --- |
| `LLM_API_KEY` | 是 | `sk-...` | 主模型 API Key | 填你实际使用模型平台的密钥 |
| `LLM_BASE_URL` | 是 | `https://api.openai.com/v1` | 模型服务地址 | 按供应商文档填写 |
| `LLM_MODEL_NAME` | 是 | `gpt-4o` | 模型名 | 填供应商实际模型 ID |
| `LLM_PROVIDER_NAME` | 是 | `OpenAI` | 接入协议类型 | 当前代码说明里支持 `OpenAI` / `Gemini` / `Ark` |

典型填写方式：

#### OpenAI 兼容接口

```env
LLM_API_KEY=你的Key
LLM_BASE_URL=https://api.openai.com/v1
LLM_MODEL_NAME=gpt-4o
LLM_PROVIDER_NAME=OpenAI
```

#### Gemini

```env
LLM_API_KEY=你的Key
LLM_BASE_URL=https://generativelanguage.googleapis.com/v1beta
LLM_MODEL_NAME=gemini-1.5-pro
LLM_PROVIDER_NAME=Gemini
```

#### 火山 Ark

```env
LLM_API_KEY=你的Key
LLM_BASE_URL=https://ark.cn-beijing.volces.com/api/v3
LLM_MODEL_NAME=你的模型ID
LLM_PROVIDER_NAME=Ark
```

建议：

- 新同学本地先用自己最熟悉的 OpenAI 兼容平台最省事
- 如果接口返回 401，先检查 `LLM_API_KEY`
- 如果接口返回 404 / model not found，优先检查 `LLM_MODEL_NAME`

### 5.5 ASR 语音识别配置

这组配置只有在你要使用语音输入转写时才需要。

| 变量 | 何时必填 | 示例 | 作用 | 怎么填 |
| --- | --- | --- | --- | --- |
| `OPENAI_ASR_BASE_URL` | 开启语音转写时 | `https://api.siliconflow.cn/v1` | ASR 服务地址 | 当前文档示例是 SiliconFlow |
| `OPENAI_ASR_API_KEY` | 开启语音转写时 | `sk-...` | ASR 服务密钥 | 必填 |
| `OPENAI_ASR_MODEL_NAME` | 开启语音转写时 | `FunAudioLLM/SenseVoiceSmall` | 语音识别模型名 | 必填 |
| `OPENAI_ASR_MODIFY_LLM_MODEL` | 可选 | `Qwen/Qwen3.5-4B` | 转写后文本修正模型 | 不填也能完成基础 ASR，只是少一层文本修正 |

重点：

- 这组配置缺任何一个关键项，ASR 就会被视为不可用
- ASR 不会自动回退到通用 `LLM_*` 配置

### 5.6 CozeLoop 链路追踪

| 变量 | 何时必填 | 示例 | 作用 | 怎么填 |
| --- | --- | --- | --- | --- |
| `COZELOOP_ENABLED` | 要启用链路追踪时 | `true` | 总开关 | 不需要就填 `false` 或不填 |
| `COZELOOP_API_BASE_URL` | 启用时 | `http://localhost:8888` | CozeLoop API 地址 | 按实际部署填写 |
| `COZELOOP_WORKSPACE_ID` | 启用时 | `workspace_xxx` | 工作区 ID | 从 CozeLoop 获取 |
| `COZELOOP_API_TOKEN` | 启用时 | `token_xxx` | 访问令牌 | 从 CozeLoop 获取 |
| `COZELOOP_SERVICE_NAME` | 建议填写 | `mianshiba-backend` | 服务名 | 用于区分链路来源 |
| `COZELOOP_DEPLOYMENT_ENV` | 建议填写 | `local` | 运行环境名 | 如 `local` / `staging` / `prod` |

说明：

- 代码会直接读 `COZELOOP_ENABLED`
- CozeLoop SDK 自身还会读取 `COZELOOP_API_BASE_URL / COZELOOP_WORKSPACE_ID / COZELOOP_API_TOKEN`

### 5.7 Embedding 配置

这组主要在 RAG / Milvus 检索场景需要。

| 变量 | 何时必填 | 示例 | 作用 | 怎么填 |
| --- | --- | --- | --- | --- |
| `EMBEDDING_API_KEY` | 开启 RAG 时通常必填 | `sk-...` | Embedding 服务密钥 | 最常用写法 |
| `EMBEDDING_PROVIDER` | 建议填写 | `openai` | Embedding 提供商类型 | 当前 `backend/config.yaml` 支持引用它 |
| `EMBEDDING_MODEL` | 开启 RAG 时必填 | `BAAI/bge-m3` | 向量模型 ID | 要和服务端实际支持模型一致 |
| `EMBEDDING_BASE_URL` | 开启 RAG 时必填 | `https://api.siliconflow.cn/v1` | Embedding 服务地址 | 按供应商填写 |
| `EMBEDDING_REGION` | 部分平台需要 | `cn-beijing` | 区域信息 | Ark 等区域型服务常见 |

补充说明：

- `backend/config.yaml` 里 `Embedding.Provider` 会引用 `EMBEDDING_PROVIDER`
- `.env.example` 里目前没有列出 `EMBEDDING_PROVIDER`，如果你使用依赖该字段的链路，建议手动补上

### 5.8 Milvus 配置

| 变量 | 何时必填 | 示例 | 作用 | 怎么填 |
| --- | --- | --- | --- | --- |
| `MILVUS_ADDRESS` | 开启 RAG 时必填 | `localhost:19530` | Milvus 地址 | 本地 Docker 默认 `localhost:19530` |
| `MILVUS_USERNAME` | 按需 | `root` | Milvus 用户名 | 如果你的 Milvus 启用了鉴权再填 |
| `MILVUS_PASSWORD` | 按需 | `password` | Milvus 密码 | 如果你的 Milvus 启用了鉴权再填 |

说明：

- 如果 `RAG_ENABLED=true`，后端会校验 `MILVUS_ADDRESS`、Embedding 等是否齐全
- 没打算启用 RAG 时，可以先不填完整

### 5.9 RAG 基础与高级开关

这组配置是项目里最复杂的一组，建议分层理解。

#### 第一层：是否启用

| 变量 | 示例 | 作用 |
| --- | --- | --- |
| `APP_ENV` | `dev` | 当前运行环境，推荐填 `dev` / `staging` / `prod` |
| `RAG_ENABLED` | `false` | 是否开启 RAG 主功能 |

建议：

- 新同学第一次跑项目先设 `RAG_ENABLED=false`
- 等 MySQL、Redis、LLM 都稳定后，再单独接 RAG

#### 第二层：L0/L1 基础能力开关

| 变量 | 默认建议 | 作用 |
| --- | --- | --- |
| `RAG_ENABLE_PROD_GUARD` | `false` | 生产保护开关 |
| `RAG_ENABLE_INGEST_RETRY` | `false` | 导入重试开关 |
| `RAG_ENABLE_RETRIEVE_AUDIT` | `true` | 检索审计开关 |
| `RAG_MAX_RETRY_COUNT` | `3` | 重试次数 |
| `RAG_RETRY_BACKOFF_MS` | `500` | 重试退避毫秒数 |
| `RAG_RETRIEVE_TIMEOUT_MS` | `3000` | 检索超时时间 |
| `RAG_USER_QPS_LIMIT` | `20` | 单用户 QPS 限制 |

#### 第三层：Phase2 检索策略开关

| 变量 | 作用 |
| --- | --- |
| `RAG_ENABLE_HYBRID_RETRIEVAL` | 混合检索 |
| `RAG_ENABLE_QUERY_REWRITE` | 查询改写 |
| `RAG_ENABLE_DYNAMIC_TOPK` | 动态 TopK |
| `RAG_ENABLE_ADVANCED_RERANK` | 高级重排 |
| `RAG_HYBRID_DENSE_WEIGHT` | 稠密向量权重 |
| `RAG_HYBRID_SPARSE_WEIGHT` | 稀疏检索权重 |
| `RAG_CANDIDATE_TOPK` | 候选集大小 |
| `RAG_MIN_TOPK` | 最小返回文档数 |
| `RAG_MAX_TOPK` | 最大返回文档数 |
| `RAG_TOKEN_BUDGET` | 上下文 token 预算 |
| `RAG_MIN_ANSWER_CHUNKS` | 最少答案片段数 |
| `RAG_REWRITE_TIMEOUT_MS` | 查询改写超时 |
| `RAG_REWRITE_MAX_EXPANSIONS` | 改写最大扩展数 |
| `RAG_RERANK_TIMEOUT_MS` | 重排超时 |
| `RAG_RERANK_MODEL` | 重排模型名 |

#### 第四层：Phase3 高级策略开关

| 变量 | 作用 |
| --- | --- |
| `RAG_ENABLE_PARENT_CHILD_RETRIEVAL` | 父子块检索 |
| `RAG_ENABLE_STRATEGIC_TOPK` | 策略型 TopK |
| `RAG_ENABLE_EVIDENCE_REFUSAL` | 证据不足拒答 |
| `RAG_ENABLE_CITATION_CONSISTENCY` | 引用一致性检查 |
| `RAG_ENABLE_DOMAIN_TERMS` | 领域术语增强 |
| `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE` | 路由级改写 |
| `RAG_ENABLE_MODEL_ASSISTED_REWRITE` | 模型辅助改写 |
| `RAG_PARENT_CHILD_FILL_STRATEGY` | 父子块补全策略 |
| `RAG_PARENT_CHILD_WINDOW_SIZE` | 窗口大小 |
| `RAG_PARENT_CHILD_MAX_TOKENS` | 父子块最大 token |
| `RAG_STRATEGIC_TOPK_MIN_K` | 最小 K |
| `RAG_STRATEGIC_TOPK_MAX_K` | 最大 K |
| `RAG_STRATEGIC_TOPK_BUDGET_RATIO` | token 预算比例 |
| `RAG_EVIDENCE_MIN_RERANK_SCORE` | 证据最小重排分 |
| `RAG_EVIDENCE_MIN_DENSITY` | 证据密度阈值 |
| `RAG_EVIDENCE_MIN_CITATION_COVERAGE` | 引用覆盖率阈值 |
| `RAG_CITATION_CHECK_THRESHOLD` | 引用一致性阈值 |
| `RAG_CITATION_CHECK_VERSION` | 引用检查版本号 |
| `RAG_DOMAIN_TERM_TIMEOUT_MS` | 术语增强超时 |
| `RAG_MODEL_REWRITE_TIMEOUT_MS` | 模型改写超时 |
| `RAG_MODEL_REWRITE_SHADOW_RATIO` | 影子流量比例 |

#### 第五层：灰度发布开关

| 变量 | 作用 |
| --- | --- |
| `RAG_RELEASE_ENABLED` | 是否启用发布控制 |
| `RAG_RELEASE_STAGE` | 发布阶段，如 `internal` / `small_flow` / `batch` / `full` |
| `RAG_RELEASE_CANARY_PERCENT` | 小流量百分比 |
| `RAG_RELEASE_BATCH_PERCENT` | 批量阶段百分比 |
| `RAG_RELEASE_INTERNAL_ROLES` | 内部角色白名单，逗号分隔 |
| `RAG_RELEASE_USER_ALLOWLIST` | 用户白名单，逗号分隔数字 ID |

#### 第六层：Phase4 治理开关

当前代码还支持以下别名/治理开关：

| 变量 | 作用 |
| --- | --- |
| `RAG_ENABLE_COST_GOVERNANCE` | 成本治理 |
| `RAG_ENABLE_AUDIT_CENTER` | 审计中心 |
| `RAG_ENABLE_VECTOR_OPS` | 向量运维总开关 |
| `RAG_ENABLE_GOVERNANCE_ALERTS` | 治理告警 |
| `RAG_ENABLE_WEEKLY_REPORT` | 周报 |
| `RAG_ENABLE_EXPERIMENT_PLATFORM` | 兼容旧实验平台开关 |
| `RAG_ENABLE_INDEX_LIFECYCLE` | 兼容旧索引生命周期开关 |
| `RAG_ENABLE_COST_DASHBOARD` | 兼容旧成本看板开关 |
| `RAG_ENABLE_COMPLIANCE_AUDIT` | 兼容旧合规审计开关 |
| `RAG_ENABLE_MILVUS_OPS_TOOLING` | 兼容旧 Milvus 运维开关 |
| `RAG_ENABLE_COLLECTION_SWITCH_GUARD` | 兼容旧集合切换保护开关 |

说明：

- 这几组里有新旧别名兼容关系
- 如果看到文档里有旧名字，不代表不能用，只是当前代码会做兼容归一

### 5.10 SMTP 邮件配置

| 变量 | 何时必填 | 示例 | 作用 | 怎么填 |
| --- | --- | --- | --- | --- |
| `SMTP_HOST` | 开启邮件功能时 | `smtp.qq.com` | SMTP 服务器地址 | 按邮箱服务商提供内容填写 |
| `SMTP_PORT` | 开启邮件功能时 | `587` | SMTP 端口 | 通常 `465` 或 `587` |
| `SMTP_USER` | 开启邮件功能时 | `xxx@example.com` | SMTP 登录账号 | 通常是邮箱地址 |
| `SMTP_PASS` | 开启邮件功能时 | `授权码` | SMTP 密码/授权码 | 很多邮箱不是登录密码，而是 SMTP 授权码 |
| `SMTP_FROM` | 开启邮件功能时 | `xxx@example.com` | 发件人邮箱 | 一般和 `SMTP_USER` 一样 |

### 5.11 LiteLLM 相关配置

`.env.example` 里列出了这组：

- `LITELLM_BASE_URL`
- `LITELLM_API_KEY`
- `LITELLM_MASTER_KEY`
- `LITELLM_SALT_KEY`
- `LITELLM_UI_USERNAME`
- `LITELLM_UI_PASSWORD`
- `LITELLM_POSTGRES_DB`
- `LITELLM_POSTGRES_USER`
- `LITELLM_POSTGRES_PASSWORD`

但要注意：

- 当前主项目代码里没有直接读取这组变量
- 这组更像是“你如果自己要部署 LiteLLM 服务时的约定配置”
- 是否需要填写，取决于你是否真的打算引入 LiteLLM 作为模型网关

如果暂时不用 LiteLLM，可以先不处理。

### 5.12 Admin 相关配置

`.env.example` 里还有：

- `ADMIN_PORT`
- `ADMIN_API_BASE_URL`

当前仓库代码里没有直接读取这两个变量作为 Admin 本地启动入口配置；实际 Admin 项目当前主要读取的是：

- `NEXT_PUBLIC_API_BASE_URL`
- `API_PROXY_TARGET`（主要出现在 Docker 构建和 rewrite 代理里）

因此：

- 本地开发 Admin 时，优先配置 `admin/.env.local` 中的 `NEXT_PUBLIC_API_BASE_URL`
- `ADMIN_PORT` / `ADMIN_API_BASE_URL` 目前更适合作为部署说明或团队约定，不是当前代码里的核心运行项

## 6. `backend/config.yaml` 里那些配置怎么理解

文件入口：[backend/config.yaml](/d:/Bear/mianshiba-eino-overseas/backend/config.yaml)

这个文件建议理解为“后端配置结构说明书”。

### 6.1 可以优先关注的几段

#### `database`

- 后端数据库连接配置
- 其中 `dsn` 会拼接 `DB_USER / DB_PASSWORD / DB_HOST / DB_PORT / DB_NAME`

#### `redis`

- Redis 地址、密码、连接池、超时参数

#### `security`

- `jwt_secret`：JWT 签名密钥
- `jwt_expiration`：JWT 过期时间
- `cors`：跨域设置

建议：

- 本地能跑不代表生产能直接用
- `jwt_secret` 不要长期保留示例值

#### `llm`

- 所有智能体统一走这里
- 实际值来自 `LLM_*`

#### `rag`

- RAG 功能总开关、各阶段策略、灰度发布都在这里定义结构
- 但实际运行时可能被 `RAG_*` 环境变量覆盖

#### `Embedding` / `Milvus`

- 向量化和向量检索的核心配置
- 开 RAG 时必须认真核对这两段

#### `payment`

- Stripe 和 PayPal 的后端支付配置
- 当前 `backend/config.yaml` 里已经有字段
- 但根目录 `.env.example` 还没把 Stripe 变量完整列出来，实际接支付时建议自己补充

建议补充到本地 `.env` 的支付变量：

```env
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
STRIPE_PUBLISHABLE_KEY=
FEISHU_WEBHOOK_URL=
```

## 7. 前端和 Admin 配置怎么填

### 7.1 前端 `frontend/.env.local`

常用写法：

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8899/api
```

作用：

- 浏览器访问后端 API 的基础地址

什么时候改成 `/api`：

- 当你通过 Nginx 或 Next.js rewrite 做统一代理时
- 例如 Docker 场景里，`frontend` 容器常用 `/api`

### 7.2 Admin `admin/.env.local`

常用写法：

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8899/api
```

Admin 额外有一个代理相关概念：

- `API_PROXY_TARGET`

它主要在 [admin/next.config.js](/d:/Bear/mianshiba-eino-overseas/admin/next.config.js) 的 rewrite 里使用。

含义：

- `NEXT_PUBLIC_API_BASE_URL`：给前端页面代码使用
- `API_PROXY_TARGET`：给 Next.js 服务端代理转发使用

本地纯开发一般只配 `NEXT_PUBLIC_API_BASE_URL` 就够了。

## 8. LiteLLM 怎么理解

项目里有 [litellm/config.yaml](/d:/Bear/mianshiba-eino-overseas/litellm/config.yaml)。

它不是主项目后端启动的必需文件，而是：

- 你要单独部署 LiteLLM 服务时使用
- 适合拿来做统一模型网关、计费、审计、路由

当前文件里示例模型是：

- `qwen3-max`
- 通过 OpenAI 兼容协议接阿里云百炼

如果你暂时不用 LiteLLM，可以忽略这部分。

## 9. 本地开发与 Docker 的填写差异

### 9.1 本地直连模式

常见写法：

```env
DB_HOST=localhost
DB_PORT=3307
REDIS_HOST=localhost
REDIS_PORT=6379
MILVUS_ADDRESS=localhost:19530
NEXT_PUBLIC_API_BASE_URL=http://localhost:8899/api
```

### 9.2 Docker Compose 模式

Docker Compose 启动时，容器内服务名会互相访问，比如：

- MySQL 走 `mysql:3306`
- Redis 走 `redis:6379`
- Milvus 走 `milvus:19530`

这也是为什么 `docker-compose.yml` 里会覆盖：

- `DB_HOST=mysql`
- `REDIS_HOST=redis`
- `MILVUS_ADDRESS=milvus:19530`

所以不要把“宿主机访问地址”和“容器内部访问地址”混在一起。

## 10. 新同学最容易踩的坑

### 10.1 只改了 `.env.example`，没有创建真正的 `.env`

`.env.example` 只是模板，不会自动生效。

正确做法：

```bash
cp .env.example .env
```

然后编辑 `.env`。

### 10.2 改了环境变量但没重启服务

以下情况都建议重启：

- 改了根目录 `.env`
- 改了 `frontend/.env.local`
- 改了 `admin/.env.local`

### 10.3 RAG 配置改了 YAML 但没生效

优先排查：

- 是否还有同名 `RAG_*` 环境变量把它覆盖了
- 是否 Docker Compose 里传了别的值

### 10.4 OAuth 回调地址填错

最常见的错误是：

- 本地前端明明是 `3000`
- 却把回调地址填成了 `8899`

OAuth 回调地址通常应该是前端页面地址，不是后端 API 地址。

### 10.5 以为 RAG 一开就能跑

实际上 `RAG_ENABLED=true` 后，后端会校验：

- `MILVUS_ADDRESS`
- `Embedding.Model`
- `Embedding.BaseURL`
- `Embedding.APIKey` 或 AK/SK
- `Embedding.Dimensions`

所以建议先关掉 RAG，把主链路跑通，再单独接入。

### 10.6 误把示例里的敏感配置当成正式值

当前仓库某些 YAML 段里可能带示例占位内容，尤其是支付相关。

上线前一定要：

- 替换成你自己的密钥
- 不要把真实生产密钥提交到仓库

## 11. 推荐给新同学的上手流程

### 方案 A：先只跑主站

1. 配 `.env` 的数据库、Redis、LLM
2. 设置 `RAG_ENABLED=false`
3. 配 `frontend/.env.local`
4. 启动 MySQL、Redis
5. 启动 backend 和 frontend

### 方案 B：再加 Admin

1. 在方案 A 基础上
2. 新建 `admin/.env.local`
3. 启动 Admin

### 方案 C：最后再加 RAG / ASR / OAuth / 支付

建议按这个顺序开功能：

1. OAuth
2. ASR
3. RAG
4. 支付
5. 观测和治理类能力

## 12. 建议维护方式

为了后续团队协作更顺畅，建议这样维护配置：

1. `.env.example` 只放模板和注释，不放真实密钥
2. 新增环境变量时，同时更新 `.env.example`
3. 新增后端配置结构时，同时更新 `backend/config.yaml` 和 `backend/config.example.yaml`
4. 对“仅部署使用、代码不直接读取”的变量，单独写注释，避免误解

## 13. 一句话总结

如果你只想快速上手：

- 根目录 `.env` 先填数据库、Redis、LLM
- 前端和 Admin 只先填 `NEXT_PUBLIC_API_BASE_URL`
- `RAG_ENABLED` 先关掉
- 其他高级配置按功能逐步开启

这样最稳，也最适合新同学理解整个项目的配置体系。
