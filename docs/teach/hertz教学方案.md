**目标陈述**
- 在你“新创建的项目”中，按模块逐步复刻当前后端的能力，从注册/登录开始，后续覆盖简历、面试流程、预测、消息队列、向量检索等
- 保持 Hertz + Thrift 的“IDL 驱动”开发范式与目录结构一致，便于教学与维护

**总体原则**
- 先定义 IDL，再代码生成，再实现 handler 逻辑，最后通过中间件与仓库层串起来
- 所有路由、模型来源于 Thrift 注解；不在生成目录里手改路由（变更通过 IDL + `hz update`）
- 配置分离：关键密钥与连接串来自环境/配置文件，业务逻辑不直接硬编码

**项目初始化**
- 在新项目根目录：
  - `go mod init <你的模块名>`
  - `go get github.com/cloudwego/hertz@latest`
  - `go get github.com/apache/thrift@v0.13.0`
  - 如需固定版本：`go mod edit -replace github.com/apache/thrift=github.com/apache/thrift@v0.13.0`，再 `go mod tidy`
- Windows 环境：确保 `hz` 在 PATH，安装 `go install github.com/cloudwego/hertz/cmd/hz@latest`

**IDL 规划**
- 按当前仓库的结构拆分 IDL（保持可迭代）：
  - `idl/user/user.thrift`：注册、登录、资料接口
    - 参考现有字段设计与注解映射：`backend/idl/user/user.thrift:131-162,235-247,249-261,270-281`
  - `idl/interviews/interviews.thrift`：面试记录与列表
  - `idl/mianshi/mianshi.thrift`：面试过程、会话、流式接口
  - `idl/prediction/prediction.thrift`：预测相关接口
  - 汇总入口 `idl/api.thrift`：
    - `include "./user/user.thrift"` 等（参考 `backend/idl/api.thrift:1-4`）
    - `service UserService extends user.UserService {}`（参考 `backend/idl/api.thrift:9`）
- 注解约定：
  - `api.post="/api/user/register"`、`api.post="/api/user/login"`
  - `api.form="field"`、`api.query="field"`、`api.path="field"`

**代码生成工作流**
- 首次生成：
  - `hz new -module <你的模块名> -idl idl\api.thrift`
- 变更 IDL 后：
  - `hz update -idl idl\api.thrift`
- 生成结果与现有仓库映射：
  - 路由入口：`api/router/register.go`（参考现有 `backend/api/router/register.go:11-15`）
  - 用户模块路由：`api/router/user/api.go`（布局风格参考 `backend/api/router/interview/api.go:65-70`）

**注册/登录实现方案**
- 存储层先用 MySQL（正式项目），演示时可先用内存仓库
- 密码安全：
  - `bcrypt` 哈希保存，登录时 `CompareHashAndPassword`
- 用户表设计（最小字段集）：
  - `id`、`username`、`email(唯一索引)`、`password_hash`、`role`、`created_at`、`updated_at`
- 注册流程：
  - 校验必填（IDL + handler 二次校验）
  - 检查邮箱唯一
  - 写入用户并默认 `role=user`
  - 签发 JWT，返回 `LoginResponse`（你当前仓库的响应风格参考 `backend/idl/user/user.thrift:158-162`）
- 登录流程：
  - 通过邮箱取用户
  - 校验密码哈希
  - 签发 JWT，返回 `LoginResponse`

**JWT 与中间件**
- 使用 `HS256`，Claims 包含 `user_id/username/role/RegisteredClaims`
- 配置项：
  - `JWT_SECRET`（环境变量/配置文件）
  - `JWT_EXP`（如 `24h`）
- 中间件策略：
  - 全局 JWT 中间件，设置 skipper 跳过 `/api/user/register`、`/api/user/login` 与 `OPTIONS`
  - Token 提取顺序：`Authorization: Bearer`、`X-Auth-Token`、`?token=`、`Cookie: token`
- 参考现有实现风格：
  - 生成/解析与上下文设置：`backend/internal/middleware/jwt.go:80-113,116-134,71-78`
  - 提取多来源 token：`backend/internal/middleware/jwt.go:186-214`

**配置与环境**
- `.env` + `config.yaml` 双轨：
  - 读取 `.env`（参考 `backend/main.go:31-36`）
  - 加载 `config.yaml` 并展开 `${ENV}`（参考 `backend/main.go:41-49`）
- 关键配置项：
  - `server.host`、`server.port`
  - `database.dsn`（MySQL 连接）
  - `redis.addr`（如后续要接入 MQ）
  - `security.jwt_secret`、`security.jwt_expiration`

**服务启动与中间件**
- `server.Default(server.WithHostPorts("<host>:<port>"))`
- `Recovery` 中间件优先注册，统一错误响应（参考你仓库路由层的 `recovery`）
- `CORS`：
  - 设置 `Access-Control-Allow-*`，拦截 `OPTIONS 204`（参考 `backend/main.go:110-125`）
- 注册由 `hz` 生成的路由入口：
  - `router.GeneratedRegister(s)`（参考 `backend/main.go:128-129`）

**验证与演示**
- 启动：`go run .`
- `curl` 测试：
  - 注册：`POST /api/user/register`，表单 `username/email/password`
  - 登录：`POST /api/user/login`，表单 `email/password`
- 验证返回结构与 JWT 可用性（带 `Authorization: Bearer <token>` 请求受保护接口）

**实施顺序（新项目分阶段复刻）**
- 阶段 A：基础与认证
  - 完成项目初始化、IDL 定义、生成路由与模型
  - 实现注册/登录，接入 MySQL/GORM，JWT 中间件与 CORS
- 阶段 B：用户资料与模型配置
  - `GetProfile/UpdateProfile`，字段映射与鉴权
  - 用户默认模型配置（对标你仓库的 `CheckUserModelConfigured`：`backend/idl/user/user.thrift:194-196,276-281`）
- 阶段 C：简历模块
  - 上传、设置默认、查询详情/列表、删除、更新
  - 路由与注解风格对齐（参考 `backend/api/router/interview/api.go:55-63` 的路径布局）
- 阶段 D：面试流程（mianshi）
  - 会话信息、记录列表、提交答案、结束面试、流式开始
  - 路由分组层次对齐（参考 `backend/api/router/interview/api.go:27-47`）
- 阶段 E：预测模块
  - 详情、列表、启动预测（参考 `backend/api/router/interview/api.go:49-53`）
- 阶段 F：消息队列与向量检索
  - MQ：Redis 队列初始化与消费者（参考 `backend/internal/mq/*.go`、`backend/main.go:89-100`）
  - Milvus：文档导入、检索器与索引器（参考 `backend/internal/eino/milvus/*`）
  - 分离到可选模块，教学时可在后半段启用

**错误处理与响应规范**
- 统一响应格式：成功返回数据；失败返回错误消息与 code
- Recovery 捕获 `panic`，避免服务崩溃；在中间件里落日志与标准错误响应
- 路由分组中可按模块加中间件链

**安全与健壮性**
- 密码只存哈希，不回显
- JWT 秘钥只来源配置，不写死
- 邮箱唯一索引与请求限流（后续可讲解）
- 生产环境 `CORS` 收敛域名，`OPTIONS` 放行但不暴露多余头

**Windows 命令速览**
- 生成项目：`hz new -module <你的模块名> -idl idl\api.thrift`
- 更新生成：`hz update -idl idl\api.thrift`
- 固定 Thrift：`go mod edit -replace github.com/apache/thrift=github.com/apache/thrift@v0.13.0 && go mod tidy`

**对齐现有仓库的关键参考**
- IDL 汇总与服务继承：`backend/idl/api.thrift:1-4,9-12`
- 用户模块的接口与注解：`backend/idl/user/user.thrift:131-162,235-247,249-261,270-281`
- 自动路由注册入口：`backend/api/router/register.go:11-15`
- 用户相关路由分组风格：`backend/api/router/interview/api.go:65-70`
- JWT 中间件实现策略：`backend/internal/middleware/jwt.go:32-78,80-113,116-134,186-214`
- 服务启动与 CORS：`backend/main.go:101-129`

如果你希望，我可以把这份“新项目从0到全功能复刻”的教学方案按章节格式化为一个文档文本，包含每阶段的课堂讲解要点与实操清单，并附上所有 `hz` 命令与 `curl` 验证示例。告诉我目标文件名与存放路径，我会直接写入并保持与上述方案完全一致的结构。

# 上传简历端到端教学方案

## 课程目标
- 完成“上传简历 → 解析简历 → 入库 → 查询”的端到端闭环
- 覆盖 Hertz 接口设计、IDL 驱动路由、文件上传与校验、持久化层、Eino Agent 编排、大模型对接
- 通过模块化实战，让学员能独立实现与扩展上传简历能力

## 架构总览
- 路由与接口
  - IDL 定义驱动生成路由：`backend/idl/interviews/interviews.thrift:204-216`
  - 路由注册：`backend/api/router/interview/api.go:55-63`
- Handler 层
  - 上传入口：`backend/api/handler/interview/interviews_service.go:56`
  - 文件校验与保存：同文件内 `handleResumeUpload(...)`
  - 解析服务调用：`service.ParseResumeAndSave(...)`，位置 `backend/chatApp/agent/service/resume_service.go:55`
- Agent 与模型
  - Agent 构建：`backend/chatApp/agent/resume/resume.go:16`
  - 模型创建：`chat.CreatOpenAiChatModel(ctx, userId)`（项目内封装，密钥走环境）
  - 工具注入：`tool2.CreatePDFToTextTool()`，先工具解析再总结
- 持久化层
  - 模型与 DAO：`backend/internal/model/resume.go`
  - 创建记录：`backend/internal/model/resume.go:33`
  - 业务服务：`backend/internal/service/interviews/impl/resume_manager_impl.go`

## 开发顺序（课堂路线）
- 第 1 步：IDL → 路由 → Handler 生成与关系
  - 上传接口定义：`backend/idl/interviews/interviews.thrift:204-216`
  - 路由注册对照：`backend/api/router/interview/api.go:55-63`
- 第 2 步：上传 Handler 最小正确性
  - 入口：`backend/api/handler/interview/interviews_service.go:56`
  - 只允许 PDF、10MB 限制，保存到 `uploads/resumes`（使用 `PWD`）
  - 用 Postman/curl 验证接收与落盘
- 第 3 步：持久化层与数据模型
  - 字段解释：`backend/internal/model/resume.go:14-23`
  - `getDB` 初始化与常见坑
  - 先用占位内容写入，跑通 DB 流程
- 第 4 步：引入 Eino Agent 编排（先讲流程）
  - Agent 构造：`backend/chatApp/agent/resume/resume.go:16`
  - 必须先调用 `pdf_to_text` 工具
  - Runner 与超时：`backend/chatApp/agent/service/resume_service.go:55` 顶层 `context.WithTimeout(..., 120*time.Second)`
  - Query 的结构化提示与 JSON 输出约束
- 第 5 步：对接大模型并联通 Agent
  - 模型创建：`chat.CreatOpenAiChatModel(ctx, userId)`，密钥环境管理
  - 跑通“pdf_to_text → LLM 总结 → JSON → 入库”：`backend/chatApp/agent/service/resume_service.go:243`
  - 将解析 JSON 存入 `Resume.Content`，文件名保留原 PDF 名、`FileType="pdf"`
- 第 6 步：查询与默认简历
  - 查询接口：`GetResume` 入口在 `backend/api/handler/interview/interviews_service.go`（函数名定位）
  - 默认简历设置与读取：`backend/internal/model/resume.go:112-129`、`impl.ResumeServer.GetDefaultResumeInfo(...)`
- 第 7 步：质量与扩展
  - 错误处理与用户态提示（401、文件无效、模型失败）
  - 性能与健壮性：超时、文件清理、并发上传、异步解析（讲思路）
  - 安全：不记录密钥、不暴露路径、控制上传大小与类型

## 课程分段（2.5–3 小时）
- 模块 1（20 分钟）：架构与 IDL 驱动
- 模块 2（40 分钟）：文件上传与校验
- 模块 3（30 分钟）：数据模型与入库
- 模块 4（45–60 分钟）：Eino Agent 与模型对接
- 模块 5（20–30 分钟）：查询、默认简历与收尾

## 演示与练习
- 演示命令
  - 上传：`curl -X POST -H "Authorization: Bearer <token>" -F "resume=@/path/resume.pdf" http://localhost:<port>/api/resume/upload`
  - 查询：`GET /api/resume/<resume_id>`、`GET /api/resume/default`、`GET /api/resume/list`
- 练习题
  - 将 10MB 限制改为 5MB 并返回友好错误
  - 增加 `.docx` 支持（扩展工具链与类型校验）
  - 为 `ParseResumeAndSave` 增加失败重试一次

## 关键讲解点与易错项
- IDL 变更后需更新生成代码，否则路由与 Handler 不匹配
- `getDB` 未初始化会 panic，开课前检查数据库初始化
- 文件保存路径依赖 `PWD`，需确认环境
- Agent 工具必须先调用，避免 LLM 凭空总结
- 超时控制与错误上报清晰，避免课堂卡住
- 不把密钥写入代码或日志

## 代码定位参考
- 路由注册：`backend/api/router/interview/api.go:55-63`
- 上传入口：`backend/api/handler/interview/interviews_service.go:56`
- 解析服务：`backend/chatApp/agent/service/resume_service.go:55`
- Agent 构建：`backend/chatApp/agent/resume/resume.go:16`
- 创建简历记录：`backend/internal/model/resume.go:33`
- 服务层上传：`backend/internal/service/interviews/impl/resume_manager_impl.go:81`
- 默认简历设置：`backend/internal/model/resume.go:112-129`

## 作业与扩展
- 支持异步解析并在前端展示解析状态
- 增加“简历内容预览”接口，返回核心字段摘要
- 设置默认时取消其他默认（业务事件化）
