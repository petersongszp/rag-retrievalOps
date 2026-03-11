# MCP Server 教学方案

## 一、教学目标

1. **理解MCP协议**：掌握Model Communication Protocol (MCP)的基本概念和工作原理
2. **掌握MCP Server架构**：理解mcpserver的核心组件和代码结构
3. **学会使用MCP Server**：能够独立启动、配置和测试mcpserver服务
4. **理解AI工具调用流程**：掌握AI模型如何通过MCP协议调用外部工具
5. **应用场景分析**：了解mcpserver在实际项目中的应用价值

## 二、课程结构

| 章节 | 主题 | 时长 | 教学方式 |
|------|------|------|----------|
| 1 | MCP协议与mcpserver概述 | 15分钟 | 理论讲解 |
| 2 | mcpserver代码结构分析 | 20分钟 | 代码讲解 |
| 3 | 服务启动与配置 | 15分钟 | 演示操作 |
| 4 | API功能测试 | 20分钟 | 实战演示 |
| 5 | 工具集成与扩展 | 15分钟 | 代码讲解 |
| 6 | 应用场景与架构设计 | 15分钟 | 案例分析 |

## 三、详细教学内容

### 1. MCP协议与mcpserver概述

#### 1.1 MCP协议介绍
- **定义**：Model Communication Protocol (MCP)是AI模型与外部工具通信的标准协议
- **核心功能**：允许AI模型调用外部工具，扩展模型能力
- **协议特点**：基于JSON-RPC 2.0，支持HTTP和SSE两种通信方式

#### 1.2 mcpserver定位
- **服务类型**：MCP协议的服务端实现
- **核心角色**：AI模型与外部工具之间的通信桥梁
- **主要功能**：工具管理、协议处理、API提供

#### 1.3 应用场景
- 大语言模型扩展工具调用能力
- AI系统与外部服务集成
- 构建可扩展的AI工具生态

### 2. mcpserver代码结构分析

#### 2.1 目录结构
```
mcpserver/
├── internal/
│   ├── config/        # 配置管理
│   ├── protocol/      # 协议处理（JSON-RPC、SSE）
│   ├── server/        # 核心服务实现
│   └── tools/         # 工具实现
├── config.yaml        # 配置文件
├── go.mod             # Go模块定义
└── main.go            # 服务入口
```

#### 2.2 核心模块详解

| 模块 | 主要职责 | 关键文件 |
|------|----------|----------|
| 配置管理 | 加载和解析配置文件 | `config/config.go` |
| 协议处理 | 处理JSON-RPC和SSE请求 | `protocol/jsonrpc.go`, `protocol/sse.go` |
| 服务实现 | HTTP服务器和MCP协议处理 | `server/http.go`, `server/mcp.go` |
| 工具管理 | 注册和提供工具 | `server/registry.go` |
| 默认工具 | 内置工具实现 | `tools/*.go` |

#### 2.3 核心流程
1. 服务启动：`main.go`加载配置，初始化服务器
2. 工具注册：注册默认工具和可选工具
3. 服务监听：启动HTTP服务器，监听请求
4. 请求处理：接收MCP请求，调用对应处理函数
5. 工具调用：执行工具逻辑，返回结果

### 3. 服务启动与配置

#### 3.1 启动服务
```bash
# 进入mcpserver目录
cd /Users/wangzhongyang/go/code/agent-interview/mcpserver

# 启动服务（默认配置）
go run main.go

# 使用指定配置文件启动
go run main.go -config config.yaml
```

#### 3.2 配置文件解析
- **配置文件位置**：`config.yaml`
- **主要配置项**：
  - 服务器地址和端口
  - 服务名称和版本
  - API密钥管理

#### 3.3 预期启动日志
```
MCP Server starting on :8080
Server name: MCP Server v0.0.1
Health check: http://:8080/health
MCP endpoint: http://:8080/mcp
Registered tool: calculator
Registered tool: echo
Registered tool: text_processor
Registered tool: time
Registered tool: weather
Weather API key not configured, skipping weather tool registration
```

### 4. API功能测试

#### 4.1 健康检查
- **接口**：`GET /health`
- **功能**：检查服务状态和已注册工具数量
- **测试命令**：
  ```bash
  curl http://localhost:8080/health
  ```
- **预期响应**：
  ```json
  {"status":"healthy","tools":5,"version":"0.0.1"}
  ```

#### 4.2 工具列表查询
- **接口**：`POST /mcp`（method: tools/list）
- **功能**：获取所有已注册的工具信息
- **测试命令**：
  ```bash
  curl -X POST http://localhost:8080/mcp \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "method": "tools/list",
      "id": "1"
    }'
  ```
- **预期响应**：包含工具名称、描述和输入schema的列表

#### 4.3 工具调用演示

##### 4.3.1 计算器工具
- **功能**：执行数学表达式计算
- **测试命令**：
  ```bash
  curl -X POST http://localhost:8080/mcp \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "method": "tools/call",
      "id": "2",
      "params": {
        "name": "calculator",
        "arguments": {
          "expression": "1 + 2 * 3"
        }
      }
    }'
  ```
- **预期响应**：
  ```json
  {"jsonrpc":"2.0","id":"2","result":{"content":"7"}}
  ```

##### 4.3.2 文本处理工具
- **功能**：对文本执行各种操作（大小写转换等）
- **测试命令**：
  ```bash
  curl -X POST http://localhost:8080/mcp \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "method": "tools/call",
      "id": "3",
      "params": {
        "name": "text_processor",
        "arguments": {
          "text": "HELLO WORLD",
          "operation": "lowercase"
        }
      }
    }'
  ```
- **预期响应**：
  ```json
  {"jsonrpc":"2.0","id":"3","result":{"content":"hello world"}}
  ```

##### 4.3.3 时间工具
- **功能**：获取当前时间或格式化时间
- **测试命令**：
  ```bash
  curl -X POST http://localhost:8080/mcp \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "method": "tools/call",
      "id": "4",
      "params": {
        "name": "time",
        "arguments": {
          "format": "2006-01-02 15:04:05"
        }
      }
    }'
  ```
- **预期响应**：当前时间的格式化输出

### 5. 工具集成与扩展

#### 5.1 默认工具类型
- **calculator**：数学计算工具
- **echo**：回声工具（返回输入内容）
- **text_processor**：文本处理工具
- **time**：时间工具
- **weather**：天气工具（需要API密钥）

#### 5.2 工具扩展方式
1. **创建工具结构体**：实现`Tool`接口
2. **实现接口方法**：
   - `Name()`：返回工具名称
   - `Description()`：返回工具描述
   - `InputSchema()`：返回输入参数schema
   - `Execute()`：执行工具逻辑
3. **注册工具**：调用`server.RegisterTool()`方法

#### 5.3 天气工具配置
- **配置项**：`weather_api_key`
- **配置方式**：在`config.yaml`中添加API密钥
- **注册逻辑**：启动时自动检查并注册

### 6. 应用场景与架构设计

#### 6.1 系统架构位置
```
┌─────────────────┐     ┌───────────────┐     ┌─────────────────┐
│   AI 模型       │────▶│  MCP Server   │────▶│  外部工具/服务  │
│  (如LLM)        │◀────│               │◀────│                 │
└─────────────────┘     └───────────────┘     └─────────────────┘
```

#### 6.2 实际应用案例
- **智能助手**：AI助手通过MCP调用各种工具（计算器、天气、日历等）
- **数据分析平台**：模型调用数据处理工具进行数据分析
- **自动化工作流**：模型通过工具调用实现业务流程自动化

#### 6.3 架构优势
- **松耦合设计**：AI模型与工具解耦，便于扩展
- **标准化协议**：基于MCP标准，便于集成不同模型和工具
- **安全可控**：集中管理工具调用，便于监控和审计

## 四、演示录制脚本

### 场景1：服务启动演示
1. 打开终端，进入mcpserver目录
2. 执行启动命令：`go run main.go`
3. 展示启动日志，讲解关键信息
4. 演示配置文件修改和重新启动

### 场景2：API测试演示
1. 使用curl测试健康检查接口
2. 测试工具列表查询
3. 测试计算器工具调用
4. 测试文本处理工具调用
5. 测试时间工具调用

### 场景3：代码结构讲解
1. 打开IDE，展示mcpserver目录结构
2. 讲解核心文件功能
3. 重点分析`main.go`的启动流程
4. 分析工具注册和调用逻辑

### 场景4：应用场景演示
1. 绘制系统架构图，讲解mcpserver的角色
2. 演示AI模型如何通过MCP调用工具
3. 讨论实际项目中的集成方案

## 五、学员实践任务

1. **启动服务**：独立启动mcpserver服务
2. **API测试**：使用curl测试所有API端点
3. **工具扩展**：尝试创建一个自定义工具（如翻译工具）
4. **配置修改**：修改配置文件，添加天气API密钥
5. **集成设计**：设计一个基于mcpserver的AI应用架构

## 六、教学资源

- **代码仓库**：/Users/wangzhongyang/go/code/agent-interview/mcpserver
- **参考文档**：MCP协议规范
- **测试工具**：curl、Postman
- **开发环境**：Go 1.18+

## 七、常见问题与解决方案

| 问题 | 解决方案 |
|------|----------|
| 服务启动失败 | 检查端口是否被占用，配置文件是否正确 |
| 工具调用出错 | 检查参数格式是否符合schema，工具是否正确注册 |
| 天气工具无法使用 | 检查API密钥是否正确配置 |
| 服务无法访问 | 检查防火墙设置，确认服务监听地址正确 |

## 八、总结与扩展

### 8.1 课程总结
- 掌握了MCP协议的基本概念
- 理解了mcpserver的架构和功能
- 学会了启动、配置和测试mcpserver服务
- 了解了工具扩展和集成方法
- 掌握了mcpserver的应用场景

### 8.2 扩展学习方向
- 深入学习MCP协议规范
- 研究大语言模型与工具的集成方式
- 探索mcpserver的高可用设计
- 学习AI工具生态系统的构建

## 九、附录

### A. 工具API参考

#### calculator工具
- **功能**：执行数学表达式计算
- **参数**：
  - `expression`：字符串，数学表达式
- **返回**：计算结果

#### text_processor工具
- **功能**：处理文本内容
- **参数**：
  - `text`：字符串，待处理文本
  - `operation`：字符串，操作类型（lowercase、uppercase、trim等）
- **返回**：处理后的文本

#### time工具
- **功能**：获取或格式化时间
- **参数**：
  - `format`：字符串，时间格式
- **返回**：格式化的时间字符串

### B. 配置文件示例
```yaml
server:
  address: :8080
  name: MCP Server
  version: 0.0.1

api_keys:
  weather_api_key: "your_weather_api_key_here"
```

---

**教学方案作者**：AI Assistant
**创建时间**：2025-12-22
**适用对象**：AI开发工程师、系统架构师、技术爱好者
**前置知识**：Go语言基础、HTTP协议、JSON-RPC基础
