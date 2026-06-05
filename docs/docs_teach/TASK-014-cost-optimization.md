# TASK-014: RAG 成本优化与计费系统开发教程

&gt; 🎯 **任务 ID**: TASK-014
&gt;
&gt; **功能名称**: 成本优化与计费
&gt;
&gt; **预估工时**: 14h
&gt;
&gt; **难度**: ⭐⭐⭐ (中级)
&gt;
&gt; **技术栈**: 成本分析、智能缓存、计费系统
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

RAG 系统成本主要来自：

- Embedding API 调用
- LLM 推理
- 向量数据库查询
- 其他服务开销

需要：

- **成本追踪**: 精确记录每项成本
- **成本优化**: 智能缓存、模型路由
- **计费系统**: 按使用量计费
- **成本看板**: 可视化展示成本趋势

### 1.2 功能需求

| 功能点 | 说明 |
|--------|------|
| 成本追踪 | 记录每次调用的成本 |
| 智能缓存 | 预测性缓存热门查询 |
| 模型路由 | 根据需求选择合适模型 |
| 计费系统 | 按 API Key 计费 |
| 成本告警 | 超预算时告警 |

---

## 二、为什么要做这个？

### 2.1 成本压力

- Embedding 和 LLM 成本持续上升
- 企业需要控制预算
- 按使用量计费是 SaaS 标配

---

## 三、技术原理

### 3.1 成本构成

```
总成本 = Embedding 成本 + LLM 成本 + 其他成本

Embedding 成本 = 输入 Token 数 × 单价
LLM 成本 = (输入 Token 数 + 输出 Token 数) × 单价
```

### 3.2 成本优化策略

| 策略 | 效果 | 实现难度 |
|------|------|---------|
| 语义缓存 | 降低 20%-40% 成本 | 中 |
| 预测性缓存 | 降低 30%-50% 成本 | 高 |
| 模型路由 | 降低 30%-60% 成本 | 中 |
| Context 压缩 | 降低 20%-30% 成本 | 中 |

---

## 四、实现步骤

### Step 1: 实现成本追踪

```go
package cost

import (
    "time"
)

// CostRecord 成本记录
type CostRecord struct {
    ID          string    `json:"id"`
    TenantID    uint64    `json:"tenant_id"`
    APIKeyID    uint64    `json:"api_key_id"`
    Feature     string    `json:"feature"` // "embedding", "llm", "retrieve"
    TokenCount  int       `json:"token_count"`
    Cost        float64   `json:"cost"`
    CreatedAt   time.Time `json:"created_at"`
}

// Pricing 定价配置
type Pricing struct {
    EmbeddingPerToken float64
    LLMInputPerToken  float64
    LLMOutputPerToken float64
    RetrievePerQuery  float64
}

// CostTracker 成本追踪器
type CostTracker struct {
    pricing Pricing
    db      *Database
}

func (ct *CostTracker) RecordEmbeddingCost(
    tenantID, apiKeyID uint64,
    tokenCount int,
) error {
    cost := float64(tokenCount) * ct.pricing.EmbeddingPerToken
    record := CostRecord{
        TenantID:   tenantID,
        APIKeyID:   apiKeyID,
        Feature:    "embedding",
        TokenCount: tokenCount,
        Cost:       cost,
        CreatedAt:  time.Now(),
    }
    return ct.db.Create(&amp;record).Error
}

func (ct *CostTracker) RecordLLMCost(
    tenantID, apiKeyID uint64,
    inputTokens, outputTokens int,
) error {
    cost := float64(inputTokens)*ct.pricing.LLMInputPerToken +
        float64(outputTokens)*ct.pricing.LLMOutputPerToken
    record := CostRecord{
        TenantID:   tenantID,
        APIKeyID:   apiKeyID,
        Feature:    "llm",
        TokenCount: inputTokens + outputTokens,
        Cost:       cost,
        CreatedAt:  time.Now(),
    }
    return ct.db.Create(&amp;record).Error
}
```

### Step 2: 实现预测性缓存

```python
from collections import defaultdict
import time

class PredictiveCache:
    def __init__(self, ttl=3600):
        self.cache = {}
        self.query_frequency = defaultdict(int)
        self.last_access = {}
        self.ttl = ttl

    def record_query(self, query):
        """记录查询频率"""
        self.query_frequency[query] += 1
        self.last_access[query] = time.time()

    def should_cache(self, query):
        """判断是否应该缓存"""
        # 高频查询（超过阈值）
        if self.query_frequency.get(query, 0) &gt;= 5:
            return True

        # 最近访问过
        if time.time() - self.last_access.get(query, 0) &lt; 300:
            return True

        return False

    def get(self, query):
        """获取缓存"""
        if query in self.cache:
            if time.time() - self.last_access[query] &lt; self.ttl:
                return self.cache[query]
            else:
                del self.cache[query]
        return None

    def set(self, query, results):
        """设置缓存"""
        if self.should_cache(query):
            self.cache[query] = results
            self.last_access[query] = time.time()
```

### Step 3: 实现模型路由

```python
class ModelRouter:
    def __init__(self):
        self.models = {
            "gpt-4": {
                "quality": 5,
                "speed": 2,
                "cost": 5,
                "capabilities": ["complex", "creative", "code"]
            },
            "gpt-3.5-turbo": {
                "quality": 4,
                "speed": 4,
                "cost": 2,
                "capabilities": ["simple", "qa"]
            },
            "claude-3-sonnet": {
                "quality": 4.5,
                "speed": 3,
                "cost": 3,
                "capabilities": ["complex", "creative"]
            }
        }

    def select_model(self, query_type, requirements):
        """
        根据需求选择最优模型

        Args:
            query_type: 查询类型
            requirements: {"quality": 0-5, "speed": 0-5, "cost": 0-5}
        """
        candidates = []

        for model_name, model_info in self.models.items():
            # 检查能力匹配
            if query_type not in model_info["capabilities"]:
                continue

            # 计算匹配分数
            score = 0
            if "quality" in requirements:
                score += (5 - abs(model_info["quality"] - requirements["quality"])) * 2
            if "speed" in requirements:
                score += (5 - abs(model_info["speed"] - requirements["speed"])) * 2
            if "cost" in requirements:
                score += (5 - abs(model_info["cost"] - requirements["cost"])) * 3

            candidates.append((model_name, score))

        # 返回分数最高的模型
        if candidates:
            candidates.sort(key=lambda x: -x[1])
            return candidates[0][0]

        return "gpt-3.5-turbo"  # 默认模型
```

### Step 4: 实现计费系统

```go
package billing

import (
    "time"
)

// Invoice 账单
type Invoice struct {
    ID          string    `json:"id"`
    TenantID    uint64    `json:"tenant_id"`
    Month       string    `json:"month"` // "2024-06"
    TotalCost   float64   `json:"total_cost"`
    Breakdown   map[string]float64 `json:"breakdown"`
    Status      string    `json:"status"` // "pending", "paid", "overdue"
    CreatedAt   time.Time `json:"created_at"`
}

// BillingManager 计费管理器
type BillingManager struct {
    db *Database
}

func (bm *BillingManager) GenerateMonthlyInvoice(tenantID uint64, month string) (*Invoice, error) {
    // 查询该月成本记录
    var records []CostRecord
    start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
    end := start.AddDate(0, 1, 0)

    bm.db.Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, start, end).Find(&amp;records)

    // 计算总成本
    breakdown := make(map[string]float64)
    totalCost := 0.0

    for _, record := range records {
        breakdown[record.Feature] += record.Cost
        totalCost += record.Cost
    }

    // 创建账单
    invoice := &amp;Invoice{
        TenantID:  tenantID,
        Month:     month,
        TotalCost: totalCost,
        Breakdown: breakdown,
        Status:    "pending",
        CreatedAt: time.Now(),
    }

    return invoice, bm.db.Create(invoice).Error
}

func (bm *BillingManager) CheckBudgetAlert(tenantID uint64, budget float64) (bool, error) {
    // 检查本月是否超预算
    month := time.Now().Format("2006-01")
    invoice, err := bm.GetInvoice(tenantID, month)
    if err != nil {
        return false, err
    }

    if invoice.TotalCost &gt;= budget {
        return true, nil
    }

    return false, nil
}
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 成本追踪 | 准确记录每次调用成本 |
| 成本优化 | 总体成本降低 30%+ |
| 计费系统 | 正确生成月度账单 |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-014-cost-optimization

git add .

git commit -m "feat: TASK-014 实现成本优化与计费系统

- 成本追踪系统
- 预测性缓存
- 模型路由
- 计费系统"

git push origin feature/TASK-014-cost-optimization
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 理解 RAG 成本构成
- ✅ 掌握成本优化策略
- ✅ 学会计费系统设计
