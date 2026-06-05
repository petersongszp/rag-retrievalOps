# TASK-017: 权限感知检索开发教程

> 🎯 **任务 ID**: TASK-017
>
> **功能名称**: 权限感知检索
>
> **预估工时**: 16h
>
> **难度**: ⭐⭐⭐⭐ (高级)
>
> **技术栈**: ACL、RBAC、多租户权限
>
> **推荐人数**: 2 人

---

## 一、需求是什么？

企业 RAG 中台不能只“能搜”，还必须“只能搜到该看的内容”。

### 1.2 功能需求

| 功能点 | 说明 |
|--------|------|
| 知识库权限继承 | 文档继承 KB 权限 |
| 文档级权限 | 单文档单独授权 |
| 片段级保护 | 敏感 Chunk 不参与返回 |
| 查询前校验 | 检索前过滤无权范围 |

---

## 二、为什么要做这个？

- ToB 场景中是是否可上线的关键项
- 也是企业招聘中非常看重的中台能力

---

## 三、技术原理

```text
身份认证
   ↓
权限解析
   ↓
构造可访问资源范围
   ↓
在可访问范围内检索
   ↓
返回脱敏后的结果
```

---

## 四、实现步骤

### Step 1: 设计权限模型

```go
type ResourcePermission struct {
	TenantID   uint64
	UserID     uint64
	ResourceID string
	ResourceType string
	Action     string
	Allow      bool
}
```

### Step 2: 查询前权限过滤

```go
func FilterAccessibleKBIDs(userID uint64, kbIDs []uint64) ([]uint64, error) {
	accessible := make([]uint64, 0)
	for _, kbID := range kbIDs {
		if hasReadPermission(userID, kbID) {
			accessible = append(accessible, kbID)
		}
	}
	return accessible, nil
}
```

### Step 3: 敏感片段脱敏

```go
func MaskSensitiveContent(content string) string {
	content = strings.ReplaceAll(content, "身份证", "[敏感字段]")
	return content
}
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 权限正确性 | 无权限用户无法检索到数据 |
| 脱敏效果 | 敏感内容可正确处理 |
| 租户隔离 | 不允许跨租户泄漏 |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-017-permission-aware-retrieval
git add .
git commit -m "feat: TASK-017 实现权限感知检索"
git push origin feature/TASK-017-permission-aware-retrieval
```
