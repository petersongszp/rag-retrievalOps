# TASK-013: RAG 评估体系增强与用户反馈收集开发教程

&gt; 🎯 **任务 ID**: TASK-013
&gt;
&gt; **功能名称**: 评估体系增强
&gt;
&gt; **预估工时**: 12h
&gt;
&gt; **难度**: ⭐⭐⭐ (中级)
&gt;
&gt; **技术栈**: RAGAS、TRULENS、A/B 测试
&gt;
&gt; **推荐人数**: 2 人

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

需要更完善的评估体系：

- **自动评估**: 使用 LLM-as-Judge 自动评估答案质量
- **用户反馈**: 收集点赞/点踩反馈
- **A/B 测试**: 对比不同策略的效果
- **评估看板**: 可视化展示评估指标

### 1.2 功能需求

| 功能点 | 说明 |
|--------|------|
| RAGAS 评估 | Faithfulness、Answer Relevance 等 |
| LLM-as-Judge | 使用 GPT-4 自动评估答案 |
| 用户反馈 | 点赞、点踩、评论 |
| 评估看板 | Grafana 展示指标 |

---

## 二、为什么要做这个？

### 2.1 市场标准

- RAGAS 成为 RAG 评估事实标准
- TruLens 提供可观测性
- 持续评估和优化闭环

---

## 三、技术原理

### 3.1 RAGAS 评估指标

| 指标 | 说明 | 计算方式 |
|------|------|---------|
| **Faithfulness** | 答案忠实度 | 答案与上下文的一致性 |
| **Answer Relevance** | 答案相关性 | 答案与问题的相关性 |
| **Context Precision** | 上下文准确率 | 相关 Chunk 排在前面 |
| **Context Recall** | 上下文召回率 | 所有相关 Chunk 都被检索 |

---

## 四、实现步骤

### Step 1: 实现 RAGAS 评估

```python
from ragas import evaluate
from ragas.metrics import (
    faithfulness,
    answer_relevancy,
    context_precision,
    context_recall
)
from datasets import Dataset

def evaluate_rag_results(
    questions: list[str],
    answers: list[str],
    contexts: list[list[str]],
    ground_truths: list[str]
):
    # 构建数据集
    dataset = Dataset.from_dict({
        "question": questions,
        "answer": answers,
        "contexts": contexts,
        "ground_truth": ground_truths
    })

    # 执行评估
    result = evaluate(
        dataset,
        metrics=[
            faithfulness,
            answer_relevancy,
            context_precision,
            context_recall
        ]
    )

    return result
```

### Step 2: 实现 LLM-as-Judge

```python
from openai import OpenAI

client = OpenAI()

JUDGE_PROMPT = """你是一个 RAG 答案评估专家。
请从以下几个维度评估答案质量：

1. 准确性 (1-5 分)
2. 完整性 (1-5 分)
3. 相关性 (1-5 分)
4. 清晰度 (1-5 分)

问题: {question}
上下文: {context}
答案: {answer}

请以 JSON 格式输出评估结果:
{{
    "accuracy": score,
    "completeness": score,
    "relevance": score,
    "clarity": score,
    "overall": score,
    "reasoning": "简短说明"
}}
"""

def llm_judge(question: str, context: str, answer: str):
    response = client.chat.completions.create(
        model="gpt-4",
        messages=[
            {"role": "system", "content": JUDGE_PROMPT},
            {"role": "user", "content": f"问题: {question}\n上下文: {context}\n答案: {answer}"}
        ],
        temperature=0
    )

    import json
    return json.loads(response.choices[0].message.content)
```

### Step 3: 实现用户反馈收集

```go
// Feedback 用户反馈
type Feedback struct {
    ID         string    `json:"id"`
    QueryID    string    `json:"query_id"`
    Query      string    `json:"query"`
    Answer     string    `json:"answer"`
    IsUseful   bool      `json:"is_useful"`
    Rating     int       `json:"rating"` // 1-5
    Comment    string    `json:"comment"`
    CreatedAt  time.Time `json:"created_at"`
}

// CollectFeedback 收集反馈
func CollectFeedback(feedback Feedback) error {
    // 保存到数据库
    return db.Create(&amp;feedback).Error
}

// GetFeedbackStats 获取反馈统计
func GetFeedbackStats(start, end time.Time) (map[string]interface{}, error) {
    var total int64
    var useful int64
    var avgRating float64

    db.Model(&amp;Feedback{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&amp;total)
    db.Model(&amp;Feedback{}).Where("created_at BETWEEN ? AND ? AND is_useful = ?", start, end, true).Count(&amp;useful)
    db.Model(&amp;Feedback{}).Where("created_at BETWEEN ? AND ?", start, end).Select("AVG(rating)").Scan(&amp;avgRating)

    return map[string]interface{}{
        "total":       total,
        "useful":      useful,
        "useful_rate": float64(useful) / float64(total),
        "avg_rating":  avgRating,
    }, nil
}
```

### Step 4: 实现 A/B 测试分析

```python
import pandas as pd
from scipy import stats

def analyze_ab_test(
    group_a_results: list[dict],
    group_b_results: list[dict],
    metric: str
):
    """
    分析 A/B 测试结果

    Args:
        group_a_results: A 组结果列表
        group_b_results: B 组结果列表
        metric: 评估指标

    Returns:
        统计分析结果
    """
    a_scores = [r[metric] for r in group_a_results]
    b_scores = [r[metric] for r in group_b_results]

    # 计算平均值
    mean_a = sum(a_scores) / len(a_scores)
    mean_b = sum(b_scores) / len(b_scores)

    # 统计显著性检验
    t_stat, p_value = stats.ttest_ind(a_scores, b_scores)

    return {
        "metric": metric,
        "mean_a": mean_a,
        "mean_b": mean_b,
        "improvement": (mean_b - mean_a) / mean_a,
        "p_value": p_value,
        "is_significant": p_value &lt; 0.05
    }
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 自动评估 | RAGAS 指标正确计算 |
| 用户反馈 | 反馈正确保存和统计 |
| A/B 测试 | 统计显著性分析正确 |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-013-evaluation-enhancement

git add .

git commit -m "feat: TASK-013 实现评估体系增强

- RAGAS 自动评估
- LLM-as-Judge
- 用户反馈收集
- A/B 测试分析"

git push origin feature/TASK-013-evaluation-enhancement
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 掌握 RAG 评估方法
- ✅ 理解 A/B 测试
- ✅ 学会评估指标分析
