# RAG 平台测试与使用指南

## 1. 环境信息

| 服务 | 地址 | 说明 |
|------|------|------|
| Admin 前端 | http://localhost:3003 | 管理控制台 |
| 后端 API | http://localhost:8899 | 直接调用 |
| MySQL | localhost:3308 | 数据库 |
| Redis | localhost:6380 | 缓存 |
| Milvus | localhost:19531 | 向量库 |
| Attu | http://localhost:8001 | Milvus 管理界面 |

---

## 2. 测试账户

| 项目 | 值 |
|------|-----|
| 邮箱 | `testuser1@ragtest.com` |
| 密码 | `TestPass12345` |
| 角色 | owner |
| tenant_id | 8 |

---

## 3. 多租户隔离说明

### 3.1 核心规则

- 每个用户注册后自动创建独立租户
- 租户之间的知识库、文档、日志完全隔离
- API Key 绑定到创建它的租户
- 跨租户访问会被拒绝（返回 404，不泄露资源存在性）

### 3.2 角色权限

| 角色 | 权限 |
|------|------|
| owner | 租户所有者，拥有全部权限 |
| admin | 管理员，可管理知识库、策略、评估 |
| member | 成员，可使用已授权的知识库 |
| viewer | 只读用户 |

---

## 4. 前端功能测试

### 4.1 登录

1. 打开 http://localhost:3003/login
2. 输入邮箱 `testuser1@ragtest.com` 和密码 `TestPass12345`
3. 点击登录
4. 应跳转到 /dashboard

### 4.2 概览 Dashboard

- 应显示知识库数量、文档数量、处理中的任务数
- 应显示检索指标图表

### 4.3 知识库管理

**创建知识库：**
1. 进入 /knowledge-bases
2. 点击"创建知识库"
3. 输入名称和描述
4. 点击确认
5. 新知识库应出现在列表中，tenant_id 应为当前租户

**上传文档：**
1. 进入知识库详情页
2. 点击"上传文档"
3. 选择文件（支持 txt、pdf、md）
4. 等待处理完成

**检索测试：**
1. 进入 /retrieval-lab
2. 选择知识库
3. 输入查询内容
4. 点击检索
5. 应返回相关结果

### 4.4 API Key 管理

1. 进入 /api-keys
2. 点击"创建 API Key"
3. 输入名称
4. 复制生成的 Key（只显示一次）
5. Key 格式：`rag_xxxxxxxxxxxx`

### 4.5 租户设置

1. 进入 /tenant/settings
2. 应显示租户信息（名称、计划、状态）
3. 进入 /tenant/usage
4. 应显示配额使用情况

### 4.6 策略中心

1. 进入 /strategy-center
2. 应显示策略标志列表
3. 可启用/禁用策略

### 4.7 评估中心

1. 进入 /evaluation/datasets
2. 可创建评估数据集
3. 进入 /evaluation/runs
4. 可创建评估运行

### 4.8 日志追踪

1. 进入 /trace-logs/retrieval
2. 应显示检索日志列表
3. 进入 /trace-logs/ingest
4. 应显示入库日志列表

---

## 5. 后端 API 测试

### 5.1 认证 API

**注册：**
```bash
curl -X POST http://localhost:8899/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"newuser","password":"TestPass12345","email":"new@test.com"}'
```

**登录：**
```bash
curl -X POST http://localhost:8899/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"testuser1@ragtest.com","password":"TestPass12345"}'
```

**获取用户信息：**
```bash
curl http://localhost:8899/v1/auth/me \
  -H "Authorization: Bearer <token>"
```

### 5.2 知识库 API

**创建知识库：**
```bash
curl -X POST http://localhost:8899/api/admin/kb/bases \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-kb","description":"我的知识库"}'
```

**列出知识库：**
```bash
curl http://localhost:8899/api/admin/kb/bases \
  -H "Authorization: Bearer <token>"
```

**删除知识库：**
```bash
curl -X DELETE http://localhost:8899/api/admin/kb/bases/<kb_id> \
  -H "Authorization: Bearer <token>"
```

### 5.3 文档 API

**上传文档：**
```bash
curl -X POST http://localhost:8899/api/admin/kb/documents/upload \
  -H "Authorization: Bearer <token>" \
  -F "kb_id=<kb_id>" \
  -F "file=@/path/to/document.txt"
```

**列出文档：**
```bash
curl "http://localhost:8899/api/admin/kb/documents?kb_id=<kb_id>" \
  -H "Authorization: Bearer <token>"
```

### 5.4 检索 API

**检索：**
```bash
curl -X POST http://localhost:8899/v1/retrieve \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query":"搜索内容","kb_ids":[<kb_id>],"top_k":5}'
```

**用 API Key 检索：**
```bash
curl -X POST http://localhost:8899/v1/retrieve \
  -H "Authorization: Bearer rag_xxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"query":"搜索内容","kb_ids":[<kb_id>],"top_k":5}'
```

### 5.5 API Key API

**创建 API Key：**
```bash
curl -X POST http://localhost:8899/v1/api-keys \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-key"}'
```

**列出 API Key：**
```bash
curl http://localhost:8899/v1/api-keys \
  -H "Authorization: Bearer <token>"
```

**删除 API Key：**
```bash
curl -X DELETE http://localhost:8899/v1/api-keys/<key_id> \
  -H "Authorization: Bearer <token>"
```

### 5.6 租户 API

**获取租户信息：**
```bash
curl http://localhost:8899/v1/tenant \
  -H "Authorization: Bearer <token>"
```

**获取租户用量：**
```bash
curl http://localhost:8899/v1/tenant/usage \
  -H "Authorization: Bearer <token>"
```

---

## 6. 租户隔离验证

### 6.1 验证跨租户访问被拒绝

1. 注册两个用户 A 和 B
2. 用户 A 创建知识库 KB-A
3. 用户 B 尝试访问 KB-A
4. 应返回 404（不泄露 KB-A 存在）

### 6.2 验证 API Key 隔离

1. 用户 A 创建 API Key-A
2. 用 API Key-A 尝试访问用户 B 的知识库
3. 应返回 404

### 6.3 验证日志隔离

1. 用户 A 执行检索
2. 用户 B 查看检索日志
3. 用户 B 不应看到用户 A 的日志

---

## 7. 配额测试

### 7.1 查看配额

```bash
curl http://localhost:8899/v1/tenant/usage \
  -H "Authorization: Bearer <token>"
```

返回示例：
```json
{
  "api_calls_today": 0,
  "kb_count": 0,
  "doc_count": 0,
  "storage_mb": 0,
  "limits": {
    "max_kb_count": 5,
    "max_doc_count": 100,
    "max_storage_mb": 1024,
    "max_api_calls_per_day": 10000
  }
}
```

### 7.2 配额超限测试

默认配额：
- 知识库数量：5 个
- 文档数量：100 个
- 存储空间：1024 MB
- 每日 API 调用：10000 次

---

## 8. 常见问题

### Q: 登录后页面 404

A: 检查 Docker 容器是否正常运行：`docker compose ps`

### Q: 策略中心报 "admin role required"

A: 确保使用 owner 或 admin 角色的账户登录

### Q: 检索返回 500

A: 知识库需要先上传文档，Milvus 集合在首次入库时创建

### Q: 跨租户访问返回 404

A: 这是正常行为，表示隔离生效

### Q: API Key 创建后无法查看

A: API Key 只在创建时显示一次，请妥善保管

---

## 9. 数据库查看

### 9.1 连接数据库

```bash
mysql -h localhost -P 3308 -u root -proot interview_agent
```

### 9.2 查看租户

```sql
SELECT id, name, slug, plan, status FROM rag_tenant;
```

### 9.3 查看用户

```sql
SELECT id, tenant_id, email, name, role FROM rag_user;
```

### 9.4 查看知识库

```sql
SELECT id, tenant_id, user_id, name, status FROM kb_knowledge_base;
```

### 9.5 查看 API Key

```sql
SELECT id, tenant_id, user_id, name, key_prefix, status FROM rag_api_key;
```

### 9.6 查看权限

```sql
SELECT id, tenant_id, kb_id, permission FROM rag_tenant_kb_permission;
```

---

## 10. Milvus 查看

打开 http://localhost:8001 连接 Milvus：
- 地址：milvus:19530（容器内）或 localhost:19531（宿主机）

查看集合和数据。
