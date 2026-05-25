# Admin 管理后台测试指南

## 当前状态总结

✅ **已完成的工作：**
1. 创建了完整的 `admin/` 项目（Next.js + Ant Design + Tailwind）
2. 配置了 API 客户端和类型定义
3. 实现了知识库管理、文档上传、任务监控、检索测试等完整功能
4. 更新了 `docker-compose.yml`、`nginx.conf`、`.env.example`，添加了 Admin 服务配置
5. 为 Admin 服务创建了 `Dockerfile`

⚠️ **需要后续完成的工作：**
需要启动基础设施服务（MySQL、Redis、Milvus），然后才能完整测试

---

## 完整测试步骤

### 1. 启动基础设施（Docker）

```bash
# 先确保 Docker 正在运行
# 然后启动所有服务
docker-compose up -d
```

这会启动：
- MySQL (端口 3307)
- Redis (端口 6379)  
- Milvus (端口 19530)
- 后端 (端口 8899)
- 前端用户版 (端口 3000)
- **Admin 管理后台 (端口 3001)**
- Nginx (端口 81)

### 2. 或者本地开发模式（分步启动）

#### 2.1 先启动基础设施

```bash
# 只启动数据库和中间件
docker-compose up -d mysql redis milvus
```

#### 2.2 启动后端服务

```bash
cd backend

# 如果是 Windows，直接运行编译好的 exe
.\server.exe

# 或者如果需要重新编译
go build -o server.exe cmd/server/main.go
.\server.exe
```

后端会在 `http://localhost:8899` 启动

#### 2.3 启动 Admin 前端（需要 Node.js）

```bash
cd admin

# 安装依赖（首次）
npm install

# 开发模式启动（端口 3001）
npm run dev

# 或者生产构建和启动
npm run build
npm start
```

Admin 会在 `http://localhost:3001` 启动

---

## 测试清单

### 基础功能测试

- [ ] **访问管理后台**：`http://localhost:3001`
- [ ] **创建知识库**：点击"新建知识库"，填写名称和描述
- [ ] **选择知识库**：在侧边栏点击选择刚创建的知识库
- [ ] **上传文档**：
  - 点击"上传文档"按钮
  - 选择 PDF/Markdown/Text 文件
  - 点击上传
- [ ] **查看任务状态**：切换到"任务列表"标签，查看上传的文档处理状态
- [ ] **重试失败任务**：如果有失败任务，点击"重试"
- [ ] **删除文档**：在文档列表中删除一个文档
- [ ] **测试检索**：
  - 点击"测试检索"
  - 输入查询内容
  - 查看检索结果和相关度分数

### API 接口测试（直接测试）

#### 创建知识库
```bash
curl -X POST http://localhost:8899/api/admin/kb/bases \
  -H "Content-Type: application/json" \
  -d '{"name": "测试知识库", "description": "测试使用"}'
```

#### 获取知识库列表
```bash
curl http://localhost:8899/api/admin/kb/bases
```

#### 上传文档（使用文件）
```bash
curl -X POST http://localhost:8899/api/admin/kb/documents/upload \
  -F "kb_id=1" \
  -F "file=@test.md"
```

#### 测试检索
```bash
curl -X POST http://localhost:8899/api/admin/kb/retrieve \
  -H "Content-Type: application/json" \
  -d '{"query": "测试问题", "kb_id": 1, "top_k": 5}'
```

---

## 访问地址汇总

| 服务 | 本地地址 | Nginx 代理地址 | 说明 |
|------|---------|--------------|------|
| 用户前端 | http://localhost:3000 | http://localhost:81/ | 用户使用 |
| **Admin 后台** | http://localhost:3001 | http://localhost:81/admin/ | 业务人员管理 |
| 后端 API | http://localhost:8899/api | http://localhost:81/api | API 服务 |

---

## 项目文件结构

```
admin/
├── src/
│   ├── app/
│   │   ├── layout.tsx      # 根布局
│   │   └── page.tsx        # 主页面（完整功能）
│   ├── config/
│   │   └── api.ts          # API 配置
│   ├── services/
│   │   └── api/
│   │       └── client.ts   # API 客户端
│   ├── types/
│   │   └── kb.ts           # 类型定义
│   └── styles/
│       └── globals.css     # 全局样式
├── package.json
├── next.config.js
├── Dockerfile
└── README.md
```

---

## 常见问题

### 1. 后端无法启动
**问题**：`Failed to initialize database: dial tcp [::1]:3307: connectex: No connection...`

**解决方案**：
- 检查 Docker 是否运行
- 运行 `docker-compose up -d mysql redis milvus` 启动基础设施
- 确认 `.env` 文件中的数据库配置正确

### 2. 前端依赖安装失败
**问题**：`npm : The term 'npm' is not recognized...`

**解决方案**：
- 安装 Node.js (v18+)
- 确保 npm 命令可以在终端中使用
- 重新运行 `npm install`

### 3. RAG 功能不工作
**问题**：检索或文档上传失败

**解决方案**：
- 检查 Milvus 是否正常运行
- 检查 `.env` 中的 Embedding API 配置
- 确认 Milvus 集合已创建

---

## 下一步

1. 确认 Docker 和 Node.js 环境已就绪
2. 按照上述步骤启动基础设施和服务
3. 进行完整的功能测试
4. 根据测试结果进行调整
