# TASK-018: 索引生命周期与重建运维开发教程

> 🎯 **任务 ID**: TASK-018
>
> **功能名称**: 索引生命周期与重建运维
>
> **预估工时**: 14h
>
> **难度**: ⭐⭐⭐ (中级)
>
> **技术栈**: 索引管理、任务调度、Milvus
>
> **推荐人数**: 1-2 人

---

## 一、需求是什么？

RAG 中台上线后，文档不断变更，索引也需要可管理、可重建、可追踪。

### 1.2 功能需求

| 功能点 | 说明 |
|--------|------|
| 索引状态管理 | 创建中、可用、失败、重建中 |
| 增量更新 | 仅更新变更文档 |
| 全量重建 | 支持手动触发 |
| 任务进度 | 展示重建进度 |
| 失败恢复 | 支持断点续跑 |

---

## 二、为什么要做这个？

- 企业系统不能只建一次索引
- 索引运维能力是中台成熟度体现

---

## 三、技术原理

```text
文档变更事件
   ↓
变更检测
   ↓
增量索引 / 全量重建
   ↓
状态持久化
   ↓
监控告警
```

---

## 四、实现步骤

### Step 1: 定义索引任务模型

```go
type IndexTask struct {
	ID         string
	KBID       uint64
	TaskType   string
	Status     string
	Progress   int
	ErrorMsg   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
```

### Step 2: 实现状态流转

```go
const (
	TaskPending   = "pending"
	TaskRunning   = "running"
	TaskSucceeded = "succeeded"
	TaskFailed    = "failed"
)
```

### Step 3: 暴露运维接口

```go
POST /api/index/rebuild
GET  /api/index/tasks/:id
POST /api/index/tasks/:id/retry
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 状态可见 | 可查看每次任务进度 |
| 重建能力 | 支持手动重建 |
| 故障恢复 | 失败任务可重试 |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-018-index-lifecycle-ops
git add .
git commit -m "feat: TASK-018 实现索引生命周期与重建运维"
git push origin feature/TASK-018-index-lifecycle-ops
```
