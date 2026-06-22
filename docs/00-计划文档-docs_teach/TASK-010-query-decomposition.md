# TASK-010: 查询分解与多步检索开发教程

&gt; 🎯 **任务 ID**: TASK-010
&gt;
&gt; **功能名称**: 查询分解与多步检索
&gt;
&gt; **预估工时**: 16h
&gt;
&gt; **难度**: ⭐⭐⭐⭐ (高级)
&gt;
&gt; **技术栈**: LLM、查询重写、思维链
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

复杂查询（如"对比 A 和 B 在 C 方面的差异"）无法通过单次检索获得完整信息，需要：

- 将复杂查询分解为多个子问题
- 对每个子问题独立检索
- 综合所有检索结果回答原始问题

### 1.2 功能需求

| 功能点 | 说明 |
|--------|------|
| 查询复杂度识别 | 判断是否需要分解 |
| 查询分解 | 使用 LLM 将复杂查询拆分为子查询 |
| 多步检索 | 并行或串行执行子查询检索 |
| 结果聚合 | 合并多个检索结果 |

---

## 二、为什么要做这个？

### 2.1 技术价值

- 提升复杂查询的准确率 30%+
- 支持多跳推理
- 类似 RAPTOR 的递归检索架构

---

## 三、技术原理

### 3.1 查询分解示例

```
原始查询: "对比 A 和 B 在 C 方面的差异"
分解为:
1. "A 在 C 方面是什么？"
2. "B 在 C 方面是什么？"
3. "A 和 B 在 C 方面的主要差异是什么？"
```

### 3.2 系统流程

```
原始查询
    ↓
[复杂度判断]
    ↓
[LLM 查询分解]
    ↓
    ├─→ 子查询 1 ─→ 检索 1 ──┐
    ├─→ 子查询 2 ─→ 检索 2 ──┤
    └─→ 子查询 3 ─→ 检索 3 ──┘
                            ↓
                      [结果聚合]
                            ↓
                      最终答案
```

---

## 四、实现步骤

### Step 1: 设计查询分解 Prompt

```python
QUERY_DECOMPOSITION_PROMPT = """你是一个查询分解专家。
请将用户的复杂查询分解为 2-5 个简单的子查询，
每个子查询都可以独立进行知识检索。

原始查询: {query}

请以 JSON 数组格式输出子查询:
["子查询1", "子查询2", "子查询3"]
"""
```

### Step 2: 实现查询分解服务

```python
from openai import OpenAI

client = OpenAI()

def decompose_query(query: str) -> list[str]:
    response = client.chat.completions.create(
        model="gpt-4",
        messages=[
            {"role": "system", "content": QUERY_DECOMPOSITION_PROMPT},
            {"role": "user", "content": query}
        ],
        temperature=0
    )

    # 解析 JSON
    import json
    sub_queries = json.loads(response.choices[0].message.content)
    return sub_queries
```

### Step 3: 实现多步检索

```python
async def multi_step_retrieve(
    rag_client,
    query: str,
    kb_ids: list[int]
) -> list:
    # 1. 分解查询
    sub_queries = decompose_query(query)

    # 2. 并行检索
    tasks = []
    for sub_q in sub_queries:
        task = rag_client.retrieve(
            query=sub_q,
            kb_ids=kb_ids,
            top_k=5
        )
        tasks.append(task)

    results_list = await asyncio.gather(*tasks)

    # 3. 聚合结果
    aggregated = []
    seen = set()
    for results in results_list:
        for item in results:
            if item.id not in seen:
                seen.add(item.id)
                aggregated.append(item)

    # 4. 重新排序
    return sorted(aggregated, key=lambda x: -x.score)[:10]
```

### Step 4: 实现复杂度判断

```python
def is_complex_query(query: str) -> bool:
    # 关键词判断
    complex_keywords = [
        "对比", "差异", "区别", "如何", "为什么",
        "步骤", "流程", "方法", "策略", "综合"
    ]

    # 长度判断
    if len(query) &gt; 50:
        return True

    # 关键词匹配
    for keyword in complex_keywords:
        if keyword in query:
            return True

    return False
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 查询分解 | 能够正确分解复杂查询 |
| 准确率提升 | 复杂查询准确率提升 30%+ |
| 性能 | 总延迟 &lt; 3s |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-010-query-decomposition

git add .

git commit -m "feat: TASK-010 实现查询分解与多步检索

- 查询复杂度识别
- LLM 查询分解
- 并行多步检索
- 结果聚合与重排序"

git push origin feature/TASK-010-query-decomposition
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 理解查询分解技术
- ✅ 掌握 LLM 思维链应用
- ✅ 提升复杂问题处理能力
