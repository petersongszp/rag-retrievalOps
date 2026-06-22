# TASK-016: 元数据过滤与分层召回开发教程

> 🎯 **任务 ID**: TASK-016
>
> **功能名称**: 元数据过滤与分层召回
>
> **预估工时**: 12h
>
> **难度**: ⭐⭐⭐ (中级)
>
> **技术栈**: Metadata Filter、召回路由、Milvus
>
> **推荐人数**: 1-2 人

---

## 一、需求是什么？

企业知识库往往包含大量不同来源、不同时间、不同部门的数据，需要支持：

- 按文档类型过滤
- 按时间范围过滤
- 按标签、来源、作者过滤
- 先过滤再召回，降低噪声

### 1.2 功能需求

| 功能点 | 说明 |
|--------|------|
| 元数据模型 | 统一字段定义 |
| 过滤表达式 | 支持 and/or/in/range |
| 分层召回 | 先过滤范围，再执行语义检索 |
| 调试能力 | 输出过滤前后样本数 |

---

## 二、为什么要做这个？

- 企业客户非常关注“只搜某个范围”
- 可明显提升精确率
- 是 ToB RAG 的高频刚需

---

## 三、技术原理

```text
用户查询
   ↓
提取 metadata filter
   ↓
筛选候选文档集合
   ↓
在候选集合中做向量检索
   ↓
返回结果
```

---

## 四、实现步骤

### Step 1: 定义元数据结构

```go
type RetrievalMetadata struct {
	DocType    string
	Source     string
	Department string
	Author     string
	Tags       []string
	CreatedAt  time.Time
}
```

### Step 2: 定义过滤条件

```go
type MetadataFilter struct {
	DocTypes    []string
	Sources     []string
	Departments []string
	Authors     []string
	Tags        []string
	StartTime   *time.Time
	EndTime     *time.Time
}
```

### Step 3: 构造过滤表达式

```go
func BuildFilterExpression(filter MetadataFilter) string {
	parts := make([]string, 0)
	if len(filter.DocTypes) > 0 {
		parts = append(parts, buildInExpr("doc_type", filter.DocTypes))
	}
	if len(filter.Departments) > 0 {
		parts = append(parts, buildInExpr("department", filter.Departments))
	}
	return strings.Join(parts, " and ")
}
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 过滤正确性 | 返回结果全部满足过滤条件 |
| 检索精度 | 精确率提升 10%+ |
| 可调试性 | 过滤日志清晰 |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-016-metadata-filtering
git add .
git commit -m "feat: TASK-016 实现元数据过滤与分层召回"
git push origin feature/TASK-016-metadata-filtering
```
