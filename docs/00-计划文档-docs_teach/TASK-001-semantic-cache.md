# TASK-001: Semantic Cache（语义缓存）详细实现教程

> 🎯 **任务 ID**: TASK-001
>
> **功能名称**: 语义缓存
>
> **预估工时**: 8 小时
>
> **难度**: ⭐⭐ (入门级)
>
> **技术栈**: Go、Redis、Embedding
>
> **推荐人数**: 1-2 人

---

## 📋 目录

- [一、需求是什么？](#一需求是什么)
- [二、为什么要做这个？](#二为什么要做这个)
- [三、技术原理](#三技术原理)
- [四、实现步骤](#四实现步骤)
- [五、如何验证？](#五如何验证)
- [六、代码提交流程](#六代码提交流程)

---

## 一、需求是什么？

### 1.1 问题背景

在 RAG 系统中，经常会遇到**重复或相似的查询**，例如：
- "什么是 RAG？"
- "RAG 是什么意思？"
- "能解释一下 RAG 吗？"

这些查询本质上是同一个问题，但每次都要：
1. 计算 Embedding（花钱）
2. 调用 Milvus 检索（花时间）
3. 可能还要调用 LLM（花更多钱）

### 1.2 解决方案

实现一个**语义缓存层**，当新查询进来时：
1. 先计算查询的 Embedding
2. 在 Redis 中查找**相似度超过阈值**的历史查询
3. 如果命中，直接返回缓存的结果
4. 如果没命中，正常走检索流程，并把结果缓存起来

### 1.3 功能需求

| 功能点 | 说明 |
|--------|------|
| 缓存存储 | 使用 Redis Hash 存储，包含：query_embedding、results、timestamp |
| 相似度计算 | 余弦相似度，阈值默认 0.95 |
| TTL | 缓存过期时间，默认 24 小时 |
| 命中率监控 | Prometheus 指标：cache_hit_rate、cache_total_requests |
| 可配置 | 支持开关、阈值、TTL 配置 |

---

## 二、为什么要做这个？

### 2.1 业务价值

| 指标 | 预期提升 |
|------|---------|
| 检索响应速度 | 命中缓存时从 2s 降到 50ms (提升 40x) |
| Token 成本 | 降低 25%+ |
| 向量数据库负载 | 降低 30%+ |

### 2.2 技术价值

- 学习**缓存设计模式**
- 掌握**相似度计算**
- 理解**Embedding 应用**
- 实践**Prometheus 埋点**

---

## 三、技术原理

### 3.1 系统架构

```
┌─────────────┐
│   用户查询   │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│  计算 Embedding │ ← (1) 对查询向量化
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  语义缓存查找   │ ← (2) 在 Redis 中查找相似查询
└──────┬──────────┘
       │
   ┌───┴───┐
   │       │
  命中    未命中
   │       │
   ▼       ▼
┌─────┐  ┌─────────────┐
│返回 │  │ 正常检索流程 │
│缓存 │  └──────┬──────┘
└─────┘         │
                ▼
         ┌─────────────┐
         │  写入缓存   │ ← (3) 缓存新结果
         └─────────────┘
```

### 3.2 核心算法：余弦相似度

```
cos_sim(a, b) = (a · b) / (||a|| * ||b||)

- 范围：[-1, 1]
- 值越大越相似
- 阈值建议：0.90 ~ 0.98
```

### 3.3 Redis 数据结构设计

```
Key: rag:cache:{kb_id}:{query_embedding_hash}
Type: Hash
Fields:
  - query_text: 原始查询文本
  - query_embedding: 查询向量 (JSON)
  - results: 检索结果 (JSON)
  - created_at: 缓存时间
  - hit_count: 命中次数
TTL: 86400 (24小时)
```

---

## 四、实现步骤

### Step 0: 先看现有代码结构

```
backend/
├── internal/
│   ├── milvus/
│   │   └── retrieval/          ← 检索相关代码在这里
│   │       ├── retrieval.go
│   │       ├── fusion.go
│   │       └── rerank.go
│   └── observability/
│       └── metrics/            ← Prometheus 埋点
│           └── rag_metrics.go
└── pkg/
    └── cache/                  ← 我们在这里创建新代码
        └── semantic_cache.go   ← (新建)
```

### Step 1: 创建语义缓存结构体

**文件**: `backend/pkg/cache/semantic_cache.go`

```go
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SemanticCache 语义缓存
type SemanticCache struct {
	rdb          *redis.Client
	embeddingDim int
	similarityTh float64
	ttl          time.Duration
	enabled      bool
}

// CacheItem 缓存项
type CacheItem struct {
	QueryText     string          `json:"query_text"`
	QueryEmbedding []float32      `json:"query_embedding"`
	Results       json.RawMessage `json:"results"`
	CreatedAt     time.Time       `json:"created_at"`
	HitCount      int             `json:"hit_count"`
}

// NewSemanticCache 创建语义缓存
func NewSemanticCache(rdb *redis.Client, embeddingDim int, similarityTh float64, ttl time.Duration, enabled bool) *SemanticCache {
	return &SemanticCache{
		rdb:          rdb,
		embeddingDim: embeddingDim,
		similarityTh: similarityTh,
		ttl:          ttl,
		enabled:      enabled,
	}
}

// Get 查找缓存
func (c *SemanticCache) Get(ctx context.Context, kbID string, queryEmbedding []float32) (*CacheItem, bool, error) {
	// TODO: 实现查找逻辑
	return nil, false, nil
}

// Set 写入缓存
func (c *SemanticCache) Set(ctx context.Context, kbID string, queryText string, queryEmbedding []float32, results interface{}) error {
	// TODO: 实现写入逻辑
	return nil
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float64 {
	// TODO: 实现余弦相似度计算
	return 0.0
}
```

### Step 2: 实现余弦相似度

**完善 `cosineSimilarity` 函数**:

```go
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct float64
	var normA float64
	var normB float64

	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

### Step 3: 实现缓存查找逻辑

**完善 `Get` 方法**:

```go
func (c *SemanticCache) Get(ctx context.Context, kbID string, queryEmbedding []float32) (*CacheItem, bool, error) {
	if !c.enabled {
		return nil, false, nil
	}

	// 1. 获取该知识库下的所有缓存 key
	pattern := fmt.Sprintf("rag:cache:%s:*", kbID)
	keys, err := c.rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, false, err
	}

	// 2. 遍历查找相似度最高的
	var bestItem *CacheItem
	var bestSim float64 = c.similarityTh

	for _, key := range keys {
		data, err := c.rdb.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}

		item := &CacheItem{}
		if err := json.Unmarshal([]byte(data["query_embedding"]), &item.QueryEmbedding); err != nil {
			continue
		}

		sim := cosineSimilarity(queryEmbedding, item.QueryEmbedding)
		if sim > bestSim {
			bestSim = sim
			item.QueryText = data["query_text"]
			item.Results = json.RawMessage(data["results"])
			item.CreatedAt, _ = time.Parse(time.RFC3339, data["created_at"])
			item.HitCount, _ = strconv.Atoi(data["hit_count"])
			bestItem = item
		}
	}

	// 3. 更新命中次数
	if bestItem != nil {
		key := fmt.Sprintf("rag:cache:%s:%x", kbID, hashEmbedding(queryEmbedding))
		c.rdb.HIncrBy(ctx, key, "hit_count", 1)
		return bestItem, true, nil
	}

	return nil, false, nil
}
```

### Step 4: 实现缓存写入逻辑

**完善 `Set` 方法**:

```go
func (c *SemanticCache) Set(ctx context.Context, kbID string, queryText string, queryEmbedding []float32, results interface{}) error {
	if !c.enabled {
		return nil
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return err
	}

	embeddingJSON, err := json.Marshal(queryEmbedding)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("rag:cache:%s:%x", kbID, hashEmbedding(queryEmbedding))

	item := map[string]interface{}{
		"query_text":      queryText,
		"query_embedding": embeddingJSON,
		"results":         resultsJSON,
		"created_at":      time.Now().Format(time.RFC3339),
		"hit_count":       0,
	}

	if err := c.rdb.HSet(ctx, key, item).Err(); err != nil {
		return err
	}

	return c.rdb.Expire(ctx, key, c.ttl).Err()
}

// hashEmbedding 简单的 Embedding 哈希（用于生成 key）
func hashEmbedding(embedding []float32) []byte {
	// 简单的哈希：取前 8 个 float32 的字节
	var b []byte
	for i := 0; i < min(8, len(embedding)); i++ {
		bits := math.Float32bits(embedding[i])
		b = append(b, byte(bits>>24), byte(bits>>16), byte(bits>>8), byte(bits))
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

### Step 5: 在检索流程中集成缓存

**文件**: `backend/internal/milvus/retrieval/retrieval.go`

找到检索入口函数，在最前面加上缓存逻辑：

```go
// Retrieve 检索入口
func (s *Service) Retrieve(ctx context.Context, kbID string, query string) ([]*Chunk, error) {
	// 1. 计算查询 Embedding
	queryEmbedding, err := s.embeddingModel.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// 2. 先查缓存
	if s.semanticCache != nil {
		cached, hit, err := s.semanticCache.Get(ctx, kbID, queryEmbedding)
		if err == nil && hit {
			// 命中缓存！
			metrics.CacheHit.Inc()
			var results []*Chunk
			json.Unmarshal(cached.Results, &results)
			return results, nil
		}
		metrics.CacheMiss.Inc()
	}

	// 3. 没命中，正常检索
	results, err := s.doRetrieve(ctx, kbID, query, queryEmbedding)
	if err != nil {
		return nil, err
	}

	// 4. 写入缓存
	if s.semanticCache != nil {
		s.semanticCache.Set(ctx, kbID, query, queryEmbedding, results)
	}

	return results, nil
}
```

### Step 6: 添加 Prometheus 指标

**文件**: `backend/internal/observability/metrics/rag_metrics.go`

添加缓存相关指标：

```go
var (
	// CacheHit 缓存命中次数
	CacheHit = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rag_cache_hit_total",
			Help: "Total number of cache hits",
		},
	)

	// CacheMiss 缓存未命中次数
	CacheMiss = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rag_cache_miss_total",
			Help: "Total number of cache misses",
		},
	)

	// CacheHitRate 缓存命中率
	CacheHitRate = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rag_cache_hit_rate",
			Help: "Cache hit rate (0-1)",
		},
	)
)

func init() {
	prometheus.MustRegister(CacheHit)
	prometheus.MustRegister(CacheMiss)
	prometheus.MustRegister(CacheHitRate)
}
```

---

## 五、如何验证？

### 5.1 单元测试

**文件**: `backend/pkg/cache/semantic_cache_test.go`

```go
package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestSemanticCache(t *testing.T) {
	// 1. 启动 miniredis（内存 Redis，用于测试）
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// 2. 创建缓存
	cache := NewSemanticCache(rdb, 1536, 0.95, 24*time.Hour, true)

	ctx := context.Background()
	kbID := "test-kb"

	// 3. 写入测试数据
	query1 := "什么是 RAG？"
	embedding1 := make([]float32, 1536)
	for i := 0; i < 1536; i++ {
		embedding1[i] = float32(i) / 1536
	}
	results1 := []map[string]interface{}{{"id": "1", "text": "RAG 是检索增强生成"}}

	err = cache.Set(ctx, kbID, query1, embedding1, results1)
	assert.NoError(t, err)

	// 4. 测试查找 - 相同 Embedding 应该命中
	item, hit, err := cache.Get(ctx, kbID, embedding1)
	assert.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, query1, item.QueryText)

	// 5. 测试查找 - 不同 Embedding 应该不命中
	embedding2 := make([]float32, 1536)
	for i := 0; i < 1536; i++ {
		embedding2[i] = float32(1536-i) / 1536
	}
	item, hit, err = cache.Get(ctx, kbID, embedding2)
	assert.NoError(t, err)
	assert.False(t, hit)
}

func TestCosineSimilarity(t *testing.T) {
	// 相同向量
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	assert.InDelta(t, 1.0, cosineSimilarity(a, b), 0.001)

	// 相反向量
	c := []float32{-1, 0, 0}
	assert.InDelta(t, -1.0, cosineSimilarity(a, c), 0.001)

	// 正交向量
	d := []float32{0, 1, 0}
	assert.InDelta(t, 0.0, cosineSimilarity(a, d), 0.001)
}
```

**运行测试**:

```bash
cd backend
go test ./pkg/cache -v
```

### 5.2 手动测试

#### 步骤 1: 启动服务

```bash
make run
```

#### 步骤 2: 第一次查询（未命中）

```bash
curl -X POST http://localhost:8080/api/v1/retrieve \
  -H "Content-Type: application/json" \
  -d '{
    "kb_id": "test-kb",
    "query": "什么是 RAG？"
  }'
```

**预期**: 正常返回结果，Prometheus 指标 `rag_cache_miss_total` +1

#### 步骤 3: 第二次查询（相似查询，应该命中）

```bash
curl -X POST http://localhost:8080/api/v1/retrieve \
  -H "Content-Type: application/json" \
  -d '{
    "kb_id": "test-kb",
    "query": "RAG 是什么意思？"
  }'
```

**预期**: 更快返回结果，Prometheus 指标 `rag_cache_hit_total` +1

#### 步骤 4: 查看 Grafana 监控

访问 Grafana，查看缓存命中率曲线

### 5.3 验收标准

| 验收项 | 标准 |
|--------|------|
| 单元测试 | 100% 通过 |
| 缓存命中率 | 相同查询命中率 100%，相似查询命中率 > 80% |
| 性能提升 | 命中缓存时响应时间 < 100ms |
| 监控指标 | Prometheus 指标正常上报 |

---

## 六、代码提交流程

### 6.1 提交代码

```bash
# 1. 创建分支
git checkout -b feature/TASK-001-semantic-cache

# 2. 查看修改
git status
git diff

# 3. 提交
git add backend/pkg/cache/semantic_cache.go
git add backend/pkg/cache/semantic_cache_test.go
git add backend/internal/milvus/retrieval/retrieval.go
git add backend/internal/observability/metrics/rag_metrics.go

git commit -m "feat: TASK-001 实现语义缓存功能

- 实现基于余弦相似度的语义缓存
- 集成 Redis 存储
- 添加 Prometheus 监控指标
- 完整单元测试覆盖"

# 4. 推送
git push origin feature/TASK-001-semantic-cache
```

### 6.2 创建 Pull Request

在 GitHub/GitLab 创建 PR，填写：

**标题**: `feat: TASK-001 实现语义缓存`

**内容**:

```markdown
## 任务说明
- 任务 ID：TASK-001
- 功能：语义缓存（Semantic Cache）
- 实现人：[你的名字]

## 实现方案
- 使用 Redis Hash 存储缓存
- 余弦相似度计算，阈值 0.95
- TTL：24 小时
- 集成 Prometheus 监控

## 验证结果
- [x] 单元测试通过（覆盖率 90%）
- [x] 本地手动测试通过
- [x] 缓存命中率 > 80%
- [x] 命中时响应时间 < 100ms

## 相关文档
- 教程文档：./docs/TASK-001-semantic-cache.md

## 截图（可选）
[如果有 Grafana 截图可以贴在这里]
```

### 6.3 等待评审

- 导师会在 24 小时内评审
- 有修改意见及时修改
- 通过后合并到主分支

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 理解语义缓存的原理
- ✅ 掌握余弦相似度计算
- ✅ 学会 Redis 高级用法
- ✅ 实践 Prometheus 监控
- ✅ 完成团队协作开发

**下一步**: 去做 TASK-004（RRF 融合），难度差不多！
