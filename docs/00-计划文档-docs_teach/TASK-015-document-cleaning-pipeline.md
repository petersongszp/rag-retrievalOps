# TASK-015: 文档清洗与切片质量治理开发教程

> 🎯 **任务 ID**: TASK-015
>
> **功能名称**: 文档清洗与切片质量治理
>
> **预估工时**: 14h
>
> **难度**: ⭐⭐⭐ (中级)
>
> **技术栈**: 文本清洗、切片策略、质量评估
>
> **推荐人数**: 1-2 人

---

## 📋 目录

- [一、需求是什么？](#一需求是什么)
- [二、为什么要做这个？](#二为什么要做这个)
- [三、技术原理](#三技术原理)
- [四、实现步骤](#四实现步骤)
- [五、验收标准](#五验收标准)
- [六、代码提交流程](#六代码提交流程)

---

## 一、需求是什么？

### 1.1 问题背景

RAG 效果差，很多时候不是检索算法不行，而是入库数据质量差：

- PDF 解析后有大量脏文本
- 标题、页眉、页脚重复污染检索
- 切片过长或过短影响召回
- 表格、代码块、列表结构丢失

### 1.2 功能需求

| 功能点 | 说明 |
|--------|------|
| 文本清洗 | 去除页眉页脚、乱码、重复空行 |
| 结构保留 | 保留标题层级、列表、代码块、表格边界 |
| 切片策略 | 支持按标题、段落、长度、语义切片 |
| 质量评分 | 对切片质量进行打分 |
| 可配置规则 | 不同文档类型支持不同清洗规则 |

---

## 二、为什么要做这个？

### 2.1 业务价值

- 检索召回率提升 15%-30%
- 降低无效 Chunk 数量
- 提高重排序和最终答案质量

### 2.2 教学价值

- 学习数据治理在 RAG 中的重要性
- 理解 chunking 对整体链路的影响
- 适合独立模块开发，不易冲突

---

## 三、技术原理

### 3.1 质量治理流程

```text
原始文档
   ↓
文档解析
   ↓
文本清洗
   ↓
结构识别
   ↓
切片生成
   ↓
质量评分
   ↓
入库
```

### 3.2 常见问题示例

```text
问题 1：页眉页脚重复
问题 2：一个 chunk 混入多个主题
问题 3：标题与正文被错误拆分
问题 4：代码片段被截断
```

---

## 四、实现步骤

### Step 1: 设计清洗配置

```go
type CleaningRule struct {
	RemoveHeaderFooter bool
	RemoveExtraSpaces  bool
	NormalizeNewlines  bool
	MergeBrokenLines   bool
	PreserveCodeBlock  bool
	PreserveTableBlock bool
}

type ChunkingRule struct {
	MaxTokens      int
	MinTokens      int
	OverlapTokens  int
	SplitByTitle   bool
	SplitBySection bool
}
```

### Step 2: 实现文本清洗

```go
func CleanText(input string, rule CleaningRule) string {
	text := input
	if rule.NormalizeNewlines {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	if rule.RemoveExtraSpaces {
		text = regexp.MustCompile(` +`).ReplaceAllString(text, " ")
	}
	if rule.MergeBrokenLines {
		text = mergeBrokenLines(text)
	}
	return text
}
```

### Step 3: 实现切片质量评分

```go
type ChunkQualityScore struct {
	LengthScore      float64
	StructureScore   float64
	CompletenessScore float64
	NoiseScore       float64
	FinalScore       float64
}

func ScoreChunk(content string) ChunkQualityScore {
	return ChunkQualityScore{
		LengthScore:      0.9,
		StructureScore:   0.8,
		CompletenessScore: 0.85,
		NoiseScore:       0.95,
		FinalScore:       0.875,
	}
}
```

### Step 4: 增加可观测指标

```go
var (
	ChunkTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "rag_chunk_total", Help: "Total chunks generated"},
		[]string{"doc_type"},
	)
	ChunkQuality = prometheus.NewHistogram(
		prometheus.HistogramOpts{Name: "rag_chunk_quality_score", Help: "Chunk quality score"},
	)
)
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 清洗效果 | 脏数据明显减少 |
| 切片效果 | 标题与正文关系保留正确 |
| 检索提升 | 召回质量提升 15%+ |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-015-document-cleaning-pipeline

git add .

git commit -m "feat: TASK-015 实现文档清洗与切片质量治理

- 文本清洗规则
- 切片策略优化
- chunk 质量评分
- 数据治理指标"

git push origin feature/TASK-015-document-cleaning-pipeline
```
