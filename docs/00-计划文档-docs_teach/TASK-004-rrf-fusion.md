# TASK-004: RRF（Reciprocal Rank Fusion）融合算法详细实现教程

> 🎯 **任务 ID**: TASK-004
>
> **功能名称**: RRF 融合算法
>
> **预估工时**: 6 小时
>
> **难度**: ⭐⭐ (入门级)
>
> **技术栈**: Go、算法
>
> **推荐人数**: 1 人

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

目前的融合是**简单加权融合**，有问题：

| Chunk ID | Dense 分数 | Sparse 分数 | 加权总分 (0.7+0.3) |
|----------|-----------|------------|-------------------|
| A        | 0.9 (第1) | 0.1 (第10) | 0.66 |
| B        | 0.6 (第5) | 0.8 (第2)  | 0.66 |

**问题**: A 在 Dense 排第1，B 在 Sparse 排第2，但总分一样！

### 1.2 解决方案

用 **RRF (Reciprocal Rank Fusion)**：
- 不看原始分数，看**排名**
- 公式：`score = sum(1 / (k + rank))`
- k 是常数（通常取 60）

### 1.3 功能需求

| 功能点 | 说明 |
|--------|------|
| RRF 算法实现 | 正确实现 RRF 公式 |
| k 值可配置 | 支持调整 k 参数 |
| 去重 | 同一个 Chunk 在多个召回源出现时去重 |
| 可切换 | 支持在 WeightedFusion 和 RRFFusion 之间切换 |

---

## 二、为什么要做这个？

### 2.1 业务价值

| 指标 | 预期提升 |
|------|---------|
| 融合质量 | NDCG@10 提升 5-10% |

### 2.2 技术价值

- 学习**信息检索融合算法**
- 理解**RRF vs 加权融合**
- 实践**算法工程化**

---

## 三、技术原理

### 3.1 RRF 公式

```
score(d) = Σ (1 / (k + r_i(d)))

其中：
- d 是文档
- r_i(d) 是文档 d 在第 i 个召回源中的排名
- k 是常数（通常取 60）
```

### 3.2 例子

假设有两个召回源：

```
召回源1 (Dense):
1. Chunk A
2. Chunk B
3. Chunk C

召回源2 (Sparse):
1. Chunk B
2. Chunk C
3. Chunk A

k = 60

计算：
Chunk A: 1/(60+1) + 1/(60+3) = 0.0164 + 0.0161 = 0.0325
Chunk B: 1/(60+2) + 1/(60+1) = 0.0161 + 0.0164 = 0.0325
Chunk C: 1/(60+3) + 1/(60+2) = 0.0161 + 0.0161 = 0.0322

等等，这样 A 和 B 还是一样？
解决：再加一个 tie-breaker（比如用原始分数）
```

### 3.3 为什么 k=60？

- k 太小：前面的排名权重太大
- k 太大：所有排名权重差不多
- 60 是经验值（来自 MS MARCO 竞赛）

---

## 四、实现步骤

### Step 0: 现有代码位置

```
backend/
└── internal/
    └── milvus/
        └── retrieval/
            ├── fusion.go          ← 现有 WeightedFusion
            └── fusion_test.go     ← 测试文件
```

### Step 1: 先看现有代码

**文件**: `backend/internal/milvus/retrieval/fusion.go`

```go
package retrieval

// FusionStrategy 融合策略接口
type FusionStrategy interface {
	Fuse(ctx context.Context, results1, results2 []*Chunk) ([]*Chunk, error)
}

// WeightedFusion 加权融合（现有）
type WeightedFusion struct {
	weight1 float64
	weight2 float64
}

func (w *WeightedFusion) Fuse(ctx context.Context, results1, results2 []*Chunk) ([]*Chunk, error) {
	// ... 现有实现 ...
}
```

### Step 2: 实现 RRFFusion

**新建/修改文件**: `backend/internal/milvus/retrieval/fusion.go`

```go
package retrieval

import (
	"context"
	"fmt"
	"math"
)

// RRFFusion RRF 融合
type RRFFusion struct {
	k float64
}

// NewRRFFusion 创建 RRF 融合器
func NewRRFFusion(k float64) *RRFFusion {
	if k <= 0 {
		k = 60 // 默认值
	}
	return &RRFFusion{k: k}
}

// Fuse 执行 RRF 融合
func (r *RRFFusion) Fuse(ctx context.Context, results1, results2 []*Chunk) ([]*Chunk, error) {
	// 1. 构建排名映射
	rankMap1 := buildRankMap(results1)
	rankMap2 := buildRankMap(results2)

	// 2. 合并所有文档 ID
	allIDs := make(map[string]bool)
	for _, c := range results1 {
		allIDs[c.ID] = true
	}
	for _, c := range results2 {
		allIDs[c.ID] = true
	}

	// 3. 计算每个文档的 RRF 分数
	scoreMap := make(map[string]float64)
	chunkMap := make(map[string]*Chunk)

	for _, c := range results1 {
		chunkMap[c.ID] = c
		rank := rankMap1[c.ID]
		scoreMap[c.ID] += 1.0 / (r.k + float64(rank))
	}

	for _, c := range results2 {
		if existing, ok := chunkMap[c.ID]; ok {
			// 已存在，累加分数，并保留原始分数用于 tie-breaker
			scoreMap[c.ID] += 1.0 / (r.k + float64(rankMap2[c.ID]))
		} else {
			chunkMap[c.ID] = c
			scoreMap[c.ID] = 1.0 / (r.k + float64(rankMap2[c.ID]))
		}
	}

	// 4. 构建结果列表
	type scoredChunk struct {
		chunk *Chunk
		score float64
		// tie-breaker: 用最大原始分数
		maxRawScore float64
	}

	var scoredList []scoredChunk
	for id, chunk := range chunkMap {
		maxRawScore := getMaxRawScore(chunk, results1, results2)
		scoredList = append(scoredList, scoredChunk{
			chunk:       chunk,
			score:       scoreMap[id],
			maxRawScore: maxRawScore,
		})
	}

	// 5. 排序：先按 RRF 分数，再按 maxRawScore
	for i := range scoredList {
		for j := i + 1; j < len(scoredList); j++ {
			if compare(scoredList[i], scoredList[j]) < 0 {
				scoredList[i], scoredList[j] = scoredList[j], scoredList[i]
			}
		}
	}

	// 6. 提取结果
	var finalResults []*Chunk
	for _, sc := range scoredList {
		finalResults = append(finalResults, sc.chunk)
	}

	return finalResults, nil
}

// buildRankMap 构建 ID -> 排名的映射
func buildRankMap(results []*Chunk) map[string]int {
	rankMap := make(map[string]int)
	for i, c := range results {
		rankMap[c.ID] = i + 1 // 排名从 1 开始
	}
	return rankMap
}

// getMaxRawScore 获取最大原始分数（用于 tie-breaker）
func getMaxRawScore(chunk *Chunk, results1, results2 []*Chunk) float64 {
	maxScore := 0.0
	for _, c := range results1 {
		if c.ID == chunk.ID && c.Score > maxScore {
			maxScore = c.Score
		}
	}
	for _, c := range results2 {
		if c.ID == chunk.ID && c.Score > maxScore {
			maxScore = c.Score
		}
	}
	return maxScore
}

// compare 比较两个 scoredChunk
func compare(a, b scoredChunk) int {
	if math.Abs(a.score-b.score) > 1e-9 {
		if a.score > b.score {
			return 1
		}
		return -1
	}
	// 分数相同，用 maxRawScore 当 tie-breaker
	if a.maxRawScore > b.maxRawScore {
		return 1
	}
	if a.maxRawScore < b.maxRawScore {
		return -1
	}
	return 0
}
```

### Step 3: 支持策略切换

**修改文件**: `backend/internal/milvus/retrieval/service.go`

```go
// Service 检索服务
type Service struct {
	// ... 其他字段 ...
	fusion FusionStrategy
}

// Config 配置
type Config struct {
	// ...
	FusionStrategy string  // "weighted" or "rrf"
	FusionK        float64 // RRF 的 k 参数
	FusionWeight1  float64 // 加权融合的 weight1
	FusionWeight2  float64 // 加权融合的 weight2
}

// NewService 创建检索服务
func NewService(cfg Config) (*Service, error) {
	var fusion FusionStrategy

	switch cfg.FusionStrategy {
	case "rrf":
		fusion = NewRRFFusion(cfg.FusionK)
	case "weighted":
		fallthrough
	default:
		fusion = NewWeightedFusion(cfg.FusionWeight1, cfg.FusionWeight2)
	}

	return &Service{
		// ...
		fusion: fusion,
	}, nil
}
```

### Step 4: 写单元测试

**文件**: `backend/internal/milvus/retrieval/fusion_test.go`

```go
package retrieval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRRFFusion(t *testing.T) {
	// 测试数据
	results1 := []*Chunk{
		{ID: "A", Score: 0.9},
		{ID: "B", Score: 0.6},
		{ID: "C", Score: 0.3},
	}
	results2 := []*Chunk{
		{ID: "B", Score: 0.8},
		{ID: "C", Score: 0.5},
		{ID: "A", Score: 0.2},
	}

	// 创建 RRF 融合器
	fusion := NewRRFFusion(60)

	// 执行融合
	result, err := fusion.Fuse(context.Background(), results1, results2)
	assert.NoError(t, err)
	assert.Len(t, result, 3)

	// 验证结果顺序（A 和 B 分数一样，看 tie-breaker）
	ids := make([]string, len(result))
	for i, c := range result {
		ids[i] = c.ID
	}

	// A 和 B 的 RRF 分数应该一样，A 的 maxRawScore 更高 (0.9 vs 0.8)
	assert.Equal(t, []string{"A", "B", "C"}, ids)
}

func TestRRFFusion_OnlyOneSource(t *testing.T) {
	results1 := []*Chunk{{ID: "A", Score: 0.9}, {ID: "B", Score: 0.6}}
	results2 := []*Chunk{}

	fusion := NewRRFFusion(60)
	result, err := fusion.Fuse(context.Background(), results1, results2)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "A", result[0].ID)
}
```

---

## 五、如何验证？

### 5.1 运行单元测试

```bash
cd backend
go test ./internal/milvus/retrieval -v -run TestRRFFusion
```

**预期**: 所有测试通过 ✅

### 5.2 本地测试

修改配置，切换到 RRF：

```yaml
# config.yaml
fusion:
  strategy: "rrf"
  k: 60
```

启动服务，测试检索：

```bash
curl -X POST http://localhost:8080/api/v1/retrieve ...
```

### 5.3 评估对比

用评估框架对比两种策略：

```bash
# 测试 WeightedFusion
go run cmd/evaluate/main.go --fusion weighted --output weighted.json

# 测试 RRFFusion
go run cmd/evaluate/main.go --fusion rrf --output rrf.json

# 对比结果
diff weighted.json rrf.json
```

**验收标准**:

| 指标 | 要求 |
|------|------|
| 单元测试 | 100% 通过 |
| NDCG@10 | 不低于加权融合，最好提升 >= 5% |

---

## 六、代码提交流程

### 6.1 提交代码

```bash
git checkout -b feature/TASK-004-rrf-fusion

git add backend/internal/milvus/retrieval/fusion.go
git add backend/internal/milvus/retrieval/fusion_test.go
git add backend/internal/milvus/retrieval/service.go

git commit -m "feat: TASK-004 实现 RRF 融合算法

- 实现 Reciprocal Rank Fusion 算法
- 支持 k 参数配置
- 支持 tie-breaker（原始分数）
- 支持策略切换（weighted/rrf）
- 完整单元测试"

git push origin feature/TASK-004-rrf-fusion
```

### 6.2 创建 PR

**标题**: `feat: TASK-004 实现 RRF 融合算法`

**内容**:

```markdown
## 任务说明
- 任务 ID：TASK-004
- 功能：RRF（Reciprocal Rank Fusion）融合算法
- 实现人：[你的名字]

## 实现方案
- 实现标准 RRF 公式
- k 值可配置（默认 60）
- tie-breaker 使用 max raw score
- 支持 weighted/rrf 策略切换

## 验证结果
- [x] 单元测试通过（覆盖率 95%）
- [x] 本地测试通过
- [x] 评估：NDCG@10 从 0.70 提升到 0.75

## 相关文档
- 教程：./docs/TASK-004-rrf-fusion.md
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 理解 RRF 算法原理
- ✅ 掌握信息检索融合技术
- ✅ 学会算法工程化
- ✅ 完成第 4 个任务！

**全部完成！** 现在你可以去挑战 TASK-005 到 TASK-008（进阶任务）了！
