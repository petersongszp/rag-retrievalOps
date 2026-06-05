# TASK-011: 自适应 RAG（根据查询动态选择策略）开发教程

&gt; 🎯 **任务 ID**: TASK-011
&gt;
&gt; **功能名称**: 自适应 RAG
&gt;
&gt; **预估工时**: 18h
&gt;
&gt; **难度**: ⭐⭐⭐⭐ (高级)
&gt;
&gt; **技术栈**: 机器学习、分类器、策略选择
&gt;
&gt; **推荐人数**: 2-3 人

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

不同查询类型需要不同的检索策略：

- 简单事实查询：快速检索 + 直接回答
- 复杂问题：需要查询分解 + 多步检索
- 创意写作：需要多样化结果 + 宽松阈值
- 代码问题：需要精确匹配 + 严格阈值

当前系统对所有查询使用相同策略，效率和效果都不是最优。

### 1.2 功能需求

| 功能点 | 说明 |
|--------|------|
| 查询意图分类 | 识别查询类型 |
| 策略路由 | 根据类型选择对应策略 |
| 策略配置 | 可配置各种策略参数 |
| 效果反馈 | 收集反馈优化策略 |

---

## 二、为什么要做这个？

### 2.1 市场趋势

- **Adaptive RAG** 成为 2024 年热点
- **Self-RAG**（反思检索）开始流行
- **Corrective RAG**（纠错检索）提升准确率

### 2.2 业务价值

- 不同查询类型准确率提升 20%-40%
- Token 成本降低 30%（简单查询用小模型
- 用户满意度提升

---

## 三、技术原理

### 3.1 查询类型分类

| 查询类型 | 特征 | 推荐策略 |
|---------|------|---------|
| 事实查询 | "谁"、"什么"、"何时"、"何地" | 快速检索 + TopK=3 |
| 复杂问题 | "如何"、"为什么"、"对比" | 查询分解 + 多步检索 |
| 创意写作 | "写"、"创作"、"生成" | 多样化检索 + 宽松阈值 |
| 代码问题 | 代码片段、函数名 | 精确匹配 + BM25 优先 |

### 3.2 自适应 RAG 流程

```
用户查询
    ↓
[查询分类器]
    ↓
    ├─→ 事实类 ─→ 策略 A (快速检索)
    ├─→ 复杂类 ─→ 策略 B (查询分解)
    ├─→ 创意类 ─→ 策略 C (多样化检索)
    └─→ 代码类 ─→ 策略 D (精确匹配)
             ↓
         [执行策略]
             ↓
         [结果返回]
```

---

## 四、实现步骤

### Step 1: 训练查询分类器

```python
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
import joblib

# 训练数据
training_data = [
    ("什么是 RAG？", "factual"),
    ("如何实现 RAG？", "complex"),
    ("写一篇关于 AI 的文章", "creative"),
    ("如何实现 Python 排序？", "code"),
    # ... 更多样本
]

# 特征提取
texts = [x[0] for x in training_data]
labels = [x[1] for x in training_data]

vectorizer = TfidfVectorizer()
X = vectorizer.fit_transform(texts)

# 训练分类器
classifier = LogisticRegression()
classifier.fit(X, labels)

# 保存模型
joblib.dump(vectorizer, "query_vectorizer.pkl")
joblib.dump(classifier, "query_classifier.pkl")
```

### Step 2: 定义策略配置

```go
// RAGStrategy RAG 策略配置
type RAGStrategy struct {
    Name           string
    TopK           int
    UseSemantic    bool
    UseBM25        bool
    UseQueryDecomp bool
    UseRerank      bool
    SimilarityTh   float64
}

// 策略映射
var StrategyMap = map[string]RAGStrategy{
    "factual": {
        Name:           "快速事实检索",
        TopK:           3,
        UseSemantic:    true,
        UseBM25:        false,
        UseQueryDecomp: false,
        UseRerank:      false,
        SimilarityTh:   0.85,
    },
    "complex": {
        Name:           "复杂问题处理",
        TopK:           10,
        UseSemantic:    true,
        UseBM25:        true,
        UseQueryDecomp: true,
        UseRerank:      true,
        SimilarityTh:   0.75,
    },
    // ... 其他策略
}
```

### Step 3: 实现自适应路由

```python
def adaptive_rag(query: str, kb_ids: list[int]):
    # 1. 分类查询
    query_type = classify_query(query)

    # 2. 获取对应策略
    strategy = StrategyMap[query_type]

    # 3. 执行策略
    if strategy.UseQueryDecomp:
        results = multi_step_retrieve(query, kb_ids, strategy)
    else:
        results = standard_retrieve(query, kb_ids, strategy)

    return {
        "query_type": query_type,
        "strategy_used": strategy.Name,
        "results": results
    }
```

### Step 4: 实现反馈收集

```python
# 收集用户反馈，用于优化策略
def collect_feedback(
    query_id: str,
    query_type: str,
    strategy_used: str,
    is_useful: bool,
    comment: str = ""
):
    feedback = {
        "query_id": query_id,
        "query_type": query_type,
        "strategy_used": strategy_used,
        "is_useful": is_useful,
        "comment": comment,
        "timestamp": datetime.now().isoformat()
    }

    # 保存到数据库
    save_feedback(feedback)

    # 定期重新训练分类器
    if should_retrain():
        retrain_classifier()
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 分类准确率 | 查询类型分类准确率 &gt; 90% |
| 效果提升 | 整体检索准确率提升 25%+ |
| 成本优化 | Token 成本降低 30%+ |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-011-adaptive-rag

git add .

git commit -m "feat: TASK-011 实现自适应 RAG

- 查询类型分类器
- 策略配置与路由
- 反馈收集系统
- 策略效果分析"

git push origin feature/TASK-011-adaptive-rag
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 理解自适应 RAG 原理
- ✅ 掌握查询分类技术
- ✅ 学会策略优化方法
- ✅ 成为 RAG 优化专家
