# 面试吧 Interview Bar

一个可直接运行、可二开、可学习的 **AI 面试训练平台**。  
它覆盖了从「简历上传」到「AI 出题追问」再到「面试结果分析」的完整闭环，适合作为 AI 全栈项目实战样板。

> 这个仓库是开源学习版。你可以直接克隆运行，也可以基于它做自己的 AI 面试产品。

## 为什么这个项目值得学习

- 真实业务闭环：登录注册、简历管理、面试流程、结果分析、支付能力都齐全
- AI 场景完整：简历预测题、综合面试、专项面试、追问与评分
- 工程化友好：前后端分离 + Docker 一键启动 + 可选扩展（Milvus / LiteLLM / CozeLoop）
- 架构清晰：Go(Hertz + Eino) + Next.js，适合训练营教学和求职作品集展示

## 功能总览

- 首页落地页：产品介绍、核心能力展示
- 简历预测题：基于简历和岗位意向自动生成问题清单
- 综合面试：校园/社招等场景化模拟，支持难度和目标公司配置
- 专项面试：按技术栈（如 Go/Java/MySQL/Redis）专项训练
- 面试进行中：实时问答流程，支持语音输入转写（ASR）
- 面试结果分析：总分、能力雷达、点评与改进建议
- 用户中心：简历上传、默认简历管理、历史记录
- 登录认证：账号密码 + GitHub/Google OAuth
- 支付模块：Stripe / PayPal（按配置启用）

## 页面截图

### 1) 首页
![首页](./xiangmu-image/FSCpt-414.png)

### 2) 简历预测题
![简历预测题](./xiangmu-image/FSCpt-415.png)

### 3) 综合面试（社招）
![综合面试-社招](./xiangmu-image/FSCpt-416.png)

### 4) 综合面试（校招）
![综合面试-校招](./xiangmu-image/FSCpt-417.png)

### 5) 专项面试
![专项面试](./xiangmu-image/FSCpt-418.png)

### 6) 用户中心与简历管理
![用户中心](./xiangmu-image/FSCpt-419.png)

### 7) 预测题详情页
![预测题详情](./xiangmu-image/FSCpt-420.png)

### 8) 面试进行中
![面试进行中](./xiangmu-image/FSCpt-421.png)

### 9) 面试结果分析
![面试结果分析](./xiangmu-image/FSCpt-422.png)

## 技术栈

- 前端：Next.js 14、React 18、TypeScript、Ant Design、Zustand、TanStack Query
- 后端：Go 1.25、Hertz、Eino
- 数据层：MySQL、Redis、GORM
- 可选能力：Milvus（向量检索）、LiteLLM（模型网关）、CozeLoop（链路追踪）
- 部署：Docker / Docker Compose、Nginx

## 架构说明（简版）

```text
Frontend (Next.js)
    |
Nginx Gateway (:81)
    |
Backend API (Go + Hertz + Eino)
    |            \
 MySQL           Redis (缓存 / 队列 / 限流)

Optional:
- Milvus: 向量检索
- LiteLLM: 多模型统一网关
- CozeLoop: 链路追踪
```

## 快速启动（推荐：Docker 一键）

### 1. 准备环境

- Docker Desktop（Windows / macOS）或 Docker Engine（Linux）
- Git

### 2. 克隆项目

```bash
git clone <你的仓库地址>
cd mianshiba-eino-overseas
```

### 3. 配置环境变量

```bash
cp .env.example .env
```

按 `.env` 中注释填写你的密钥（项目已移除真实 Key，均为占位符说明）。

### 4. 一键启动

```bash
docker-compose up -d
```

### 5. 访问地址

- 统一入口（推荐）：`http://localhost:81`
- 前端直连：`http://localhost:3000`
- 后端 API：`http://localhost:8899`
- 健康检查：`http://localhost:8899/health`

## 本地开发模式（可选）

如果你希望前后端分开调试：

### 启动依赖

```bash
docker-compose up -d mysql redis
```

### 后端

```bash
cd backend
go run ./cmd/server/main.go
```

### 前端

```bash
cd frontend
npm install
npm run dev
```

## 核心目录

```text
backend/
  cmd/server/                 # 后端启动入口
  api/                        # 路由与 Handler
  internal/service/           # 核心业务（面试、简历、支付等）
  internal/agents/            # AI 代理逻辑

frontend/
  src/app/                    # 页面路由（首页、面试、用户中心等）
  src/components/             # 通用组件

xiangmu-image/                # README 页面截图
```

## 你可以从这个项目学到什么

- 如何设计 AI 产品的完整业务闭环
- 如何把大模型能力融入可交付的 Web 应用
- 如何组织 Go + Next.js 的前后端工程结构
- 如何做可扩展的配置管理与容器化部署

## 训练营引导

如果你希望系统学习这个项目背后的完整实现（从 0 到 1 搭建、AI 能力接入、工程化落地、简历优化与面试实战），欢迎加入：

- 训练营名称：**就业陪跑训练营**
- 报名链接：**https://wangzhongyang.com**
- 咨询方式：微信扫码咨询，备注 **【训练营】**

![微信二维码（扫码咨询，备注【训练营】）](./xiangmu-image/wechat-qr.png)

也欢迎先 `Star` 本仓库，后续会持续更新更多实战模块。

## License

MIT
