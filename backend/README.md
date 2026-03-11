# 后端服务 (Backend Service)

基于 Hertz 和 Eino 框架构建的高并发 AI 面试代理服务。

## 目录结构说明

本项目采用符合 Go 标准布局 (Standard Go Project Layout) 的目录结构，并结合了领域驱动设计 (DDD) 的思想。

### 1. 核心目录

*   **`cmd/`**: 应用程序入口
    *   `server/main.go`: 后端 API 服务主入口。
*   **`internal/`**: 私有应用代码（外部项目无法引用）
    *   **`service/`**: 业务逻辑层（Domain Service）。
        *   `interview/engine/`: **面试引擎核心**。包含面试状态机、SSE 实时流处理、LLM 交互编排等核心逻辑。
    *   **`agents/`**: **AI 智能体层**。封装了具体的 AI 编排逻辑（UseCase），如简历分析、专项面试问答等。
    *   **`config/`**: 配置管理模块。
    *   **`repository/`**: 数据持久层（MySQL, Redis）。
    *   **`milvus/`**: 向量数据库相关操作及工具。
*   **`api/`**: 接口层
    *   `handler/`: HTTP 请求处理器。负责参数绑定、校验和响应格式化，不包含复杂业务逻辑。
    *   `router/`: 路由定义。
    *   `idl/`: Thrift 接口定义文件。
*   **`pkg/`**: 公共工具库（可被外部引用）。
*   **`scripts/`**: 运维与评估脚本（原 `tests` 目录）。

### 2. 关键架构变更 (2026-01-31)

为了提升可维护性和安全性，我们进行了以下架构优化：

1.  **领域封装**: 将原根目录下的 `agents` 移至 `internal/agents`，强制隐藏核心 AI 逻辑。
2.  **逻辑下沉**: 将 `api/handler/interview/core` 中的面试引擎逻辑下沉至 `internal/service/interview/engine`，实现了 Handler 层与 Service 层的彻底解耦。
3.  **配置动态化**: `config.yaml` 全面支持环境变量注入（如 `${DB_HOST}`），完美适配 Docker 及多环境部署。
4.  **模块重命名**: Go module 名称已统一为 `interview-agents`。

## 快速开始

### 本地开发

1.  **配置环境**:
    复制 `.env.example` 为 `.env` 并填入必要的 API Key。
    ```bash
    cp .env.example .env
    ```

2.  **启动依赖**:
    使用 Docker Compose 启动 MySQL, Redis, Milvus 等基础组件。
    ```bash
    docker-compose up -d mysql redis milvus etcd minio
    ```

3.  **运行服务**:
    ```bash
    cd backend
    go run ./cmd/server/main.go
    ```

### 部署

构建并启动所有服务：
```bash
docker-compose up -d --build
```
