# 项目功能模块划分与代码导读

本文档旨在帮助开发者快速熟悉 `agent-interview` 项目的架构、功能模块划分以及核心代码位置。

## 1. 项目概览

本项目是一个基于 AI 的模拟面试平台，采用 **Go (Hertz)** 作为后端框架，**Next.js** 作为前端框架，核心 AI 能力由字节跳动的 **Eino** 框架驱动。

**主要目录结构：**
- `backend/`: 后端服务代码
- `frontend/`: 前端应用代码
- `mcpserver/` & `backend/mcp-moduel/`: MCP (Model Context Protocol) 相关实现
- `doc/`: 项目文档

---

## 2. 后端功能模块 (Backend)

后端采用分层架构：`API Layer` -> `Service Layer` -> `Repository/Model Layer`，并包含独立的 `AI Agent Layer`。

### 2.1 API 接入层 (`backend/api`)
负责处理 HTTP 请求、路由分发、参数校验和响应格式化。

*   **路由配置 (`backend/api/router/`)**
    *   `register.go`: 路由注册入口。
    *   `interview/api.go`: 面试相关路由定义。
*   **请求处理器 (`backend/api/handler/`)**
    *   **面试服务 (`backend/api/handler/interview/mianshi/`)**
        *   `mianshi_service.go`: 核心面试流程控制。
            *   `StartMianshiStream`: **[核心]** 启动面试流程（SSE 流式响应），负责创建会话、初始化 Agent 并建立连接。
    *   **用户服务 (`backend/api/handler/interview/`)**
        *   `user_service.go`: 用户模型管理。
            *   `CreateUserModel`: 用户添加自定义 LLM 配置。
            *   `ListUserModels`: 获取用户配置的模型列表。
    *   **预测服务 (`backend/api/handler/interview/`)**
        *   `prediction_service.go`: 面试押题服务。

### 2.2 AI 智能体层 (`backend/chatApp`)
项目的核心业务逻辑，基于 Eino 框架实现的 AI Agents。

*   **智能体实现 (`backend/chatApp/agent/`)**
    *   **综合面试 (`backend/chatApp/agent/interview/comprehensive/`)**
        *   `school_comprehensive_agent.go`: 校招综合面试 Agent。
            *   `NewSchoolComprehensiveAgent`: 创建 Agent 实例，集成简历工具和 OpenAI 模型。
    *   **专项面试 (`backend/chatApp/agent/interview/specialized/`)**
        *   `go_agent.go`, `java_agent.go`, `redis_agent.go`: 针对特定技术栈的面试 Agent。
    *   **面试押题 (`backend/chatApp/agent/prediction/`)**
        *   `prediction_agent.go`: 根据简历预测面试题。
            *   `NewPredictionAgent`: 创建押题 Agent，包含特定的 Prompt 指令。
*   **工具集 (`backend/chatApp/tool/`)**
    *   `get_resume_info_tool.go`: 简历解析与信息获取工具。
    *   `milvus_retriever_tool.go`: 向量数据库检索工具（用于 RAG）。

### 2.3 核心业务服务层 (`backend/internal/service`)
封装通用的业务逻辑，供 API 层调用。

*   **面试管理 (`backend/internal/service/interviews/`)**
    *   `interface.go`: 定义接口。
        *   `InterviewManager`: 面试记录的增删改查。
        *   `ResumeManager`: 简历上传、解析和管理。
    *   `impl/interview_impl.go`: `InterviewManager` 的具体实现。
*   **用户管理 (`backend/internal/service/user/`)**
    *   `interface.go`: 定义接口。
        *   `ModelManager`: 管理用户自定义的 LLM 模型配置（API Key, Base URL 等）。
        *   `UserManager`: 用户注册、登录、个人信息管理。

### 2.4 基础设施与数据层 (`backend/internal`)
*   **Eino/RAG (`backend/internal/eino/`)**
    *   `milvus/`: 向量数据库 Milvus 的集成与管理。
        *   `retrieval/`: 检索逻辑。
        *   `storage/`: 向量存储逻辑。
*   **数据库模型 (`backend/internal/model/`)**
    *   `interview_record.go`: 面试记录表结构。
    *   `resume.go`: 简历表结构。
    *   `user_model.go`: 用户 LLM 配置表结构。

---

## 3. 前端功能模块 (Frontend)

前端基于 Next.js App Router 架构。

### 3.1 页面路由 (`frontend/src/app`)
*   **面试模块 (`frontend/src/app/interview/`)**
    *   `campus/`: 校招面试页面。
    *   `social/`: 社招面试页面。
    *   `special/`: 专项技术面试页面。
*   **用户中心 (`frontend/src/app/user/`)**
    *   `models/`: **[关键]** 用户大模型配置页面（对应后端的 `user_service.go`）。
    *   `center/`: 个人中心首页。
*   **简历模块 (`frontend/src/app/resume/`)**
    *   `page.tsx`: 简历上传与管理页面。

### 3.2 核心服务调用 (`frontend/src/services`)
*   `api/client.ts`: 统一的 API 请求客户端。
*   `api/prediction.ts`: 押题相关 API 调用。

---

## 4. 快速上手建议

1.  **想了解面试流程如何启动？**
    *   查看 `backend/api/handler/interview/mianshi/mianshi_service.go` 中的 `StartMianshiStream` 方法。
2.  **想修改 AI 的提示词（Prompt）？**
    *   前往 `backend/chatApp/agent/` 目录，找到对应的 Agent 文件（如 `school_comprehensive_agent.go`），查看 `Instruction` 字段或相关配置。
3.  **想增加新的面试类型？**
    *   后端：在 `backend/chatApp/agent/interview/` 下创建新的 Agent，并在 `backend/api/handler` 中注册调用。
    *   前端：在 `frontend/src/app/interview/` 下增加新页面。
4.  **想了解 RAG（检索增强生成）是如何实现的？**
    *   查看 `backend/internal/eino/milvus` 目录下的代码，以及 Agent 中如何挂载 `milvus_retriever_tool`。# 项目功能模块划分与代码导读

本文档旨在帮助开发者快速熟悉 `agent-interview` 项目的架构、功能模块划分以及核心代码位置。

## 1. 项目概览

本项目是一个基于 AI 的模拟面试平台，采用 **Go (Hertz)** 作为后端框架，**Next.js** 作为前端框架，核心 AI 能力由字节跳动的 **Eino** 框架驱动。

**主要目录结构：**
- `backend/`: 后端服务代码
- `frontend/`: 前端应用代码
- `mcpserver/` & `backend/mcp-moduel/`: MCP (Model Context Protocol) 相关实现
- `doc/`: 项目文档

---

## 2. 后端功能模块 (Backend)

后端采用分层架构：`API Layer` -> `Service Layer` -> `Repository/Model Layer`，并包含独立的 `AI Agent Layer`。

### 2.1 API 接入层 (`backend/api`)
负责处理 HTTP 请求、路由分发、参数校验和响应格式化。

*   **路由配置 (`backend/api/router/`)**
    *   `register.go`: 路由注册入口。
    *   `interview/api.go`: 面试相关路由定义。
*   **请求处理器 (`backend/api/handler/`)**
    *   **面试服务 (`backend/api/handler/interview/mianshi/`)**
        *   `mianshi_service.go`: 核心面试流程控制。
            *   `StartMianshiStream`: **[核心]** 启动面试流程（SSE 流式响应），负责创建会话、初始化 Agent 并建立连接。
    *   **用户服务 (`backend/api/handler/interview/`)**
        *   `user_service.go`: 用户模型管理。
            *   `CreateUserModel`: 用户添加自定义 LLM 配置。
            *   `ListUserModels`: 获取用户配置的模型列表。
    *   **预测服务 (`backend/api/handler/interview/`)**
        *   `prediction_service.go`: 面试押题服务。

### 2.2 AI 智能体层 (`backend/chatApp`)
项目的核心业务逻辑，基于 Eino 框架实现的 AI Agents。

*   **智能体实现 (`backend/chatApp/agent/`)**
    *   **综合面试 (`backend/chatApp/agent/interview/comprehensive/`)**
        *   `school_comprehensive_agent.go`: 校招综合面试 Agent。
            *   `NewSchoolComprehensiveAgent`: 创建 Agent 实例，集成简历工具和 OpenAI 模型。
    *   **专项面试 (`backend/chatApp/agent/interview/specialized/`)**
        *   `go_agent.go`, `java_agent.go`, `redis_agent.go`: 针对特定技术栈的面试 Agent。
    *   **面试押题 (`backend/chatApp/agent/prediction/`)**
        *   `prediction_agent.go`: 根据简历预测面试题。
            *   `NewPredictionAgent`: 创建押题 Agent，包含特定的 Prompt 指令。
*   **工具集 (`backend/chatApp/tool/`)**
    *   `get_resume_info_tool.go`: 简历解析与信息获取工具。
    *   `milvus_retriever_tool.go`: 向量数据库检索工具（用于 RAG）。

### 2.3 核心业务服务层 (`backend/internal/service`)
封装通用的业务逻辑，供 API 层调用。

*   **面试管理 (`backend/internal/service/interviews/`)**
    *   `interface.go`: 定义接口。
        *   `InterviewManager`: 面试记录的增删改查。
        *   `ResumeManager`: 简历上传、解析和管理。
    *   `impl/interview_impl.go`: `InterviewManager` 的具体实现。
*   **用户管理 (`backend/internal/service/user/`)**
    *   `interface.go`: 定义接口。
        *   `ModelManager`: 管理用户自定义的 LLM 模型配置（API Key, Base URL 等）。
        *   `UserManager`: 用户注册、登录、个人信息管理。

### 2.4 基础设施与数据层 (`backend/internal`)
*   **Eino/RAG (`backend/internal/eino/`)**
    *   `milvus/`: 向量数据库 Milvus 的集成与管理。
        *   `retrieval/`: 检索逻辑。
        *   `storage/`: 向量存储逻辑。
*   **数据库模型 (`backend/internal/model/`)**
    *   `interview_record.go`: 面试记录表结构。
    *   `resume.go`: 简历表结构。
    *   `user_model.go`: 用户 LLM 配置表结构。

---

## 3. 前端功能模块 (Frontend)

前端基于 Next.js App Router 架构。

### 3.1 页面路由 (`frontend/src/app`)
*   **面试模块 (`frontend/src/app/interview/`)**
    *   `campus/`: 校招面试页面。
    *   `social/`: 社招面试页面。
    *   `special/`: 专项技术面试页面。
*   **用户中心 (`frontend/src/app/user/`)**
    *   `models/`: **[关键]** 用户大模型配置页面（对应后端的 `user_service.go`）。
    *   `center/`: 个人中心首页。
*   **简历模块 (`frontend/src/app/resume/`)**
    *   `page.tsx`: 简历上传与管理页面。

### 3.2 核心服务调用 (`frontend/src/services`)
*   `api/client.ts`: 统一的 API 请求客户端。
*   `api/prediction.ts`: 押题相关 API 调用。

---

## 4. 快速上手建议

1.  **想了解面试流程如何启动？**
    *   查看 `backend/api/handler/interview/mianshi/mianshi_service.go` 中的 `StartMianshiStream` 方法。
2.  **想修改 AI 的提示词（Prompt）？**
    *   前往 `backend/chatApp/agent/` 目录，找到对应的 Agent 文件（如 `school_comprehensive_agent.go`），查看 `Instruction` 字段或相关配置。
3.  **想增加新的面试类型？**
    *   后端：在 `backend/chatApp/agent/interview/` 下创建新的 Agent，并在 `backend/api/handler` 中注册调用。
    *   前端：在 `frontend/src/app/interview/` 下增加新页面。
4.  **想了解 RAG（检索增强生成）是如何实现的？**
    *   查看 `backend/internal/eino/milvus` 目录下的代码，以及 Agent 中如何挂载 `milvus_retriever_tool`。
