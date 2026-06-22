# TASK-012: 限流、熔断、降级与灰度发布开发教程

&gt; 🎯 **任务 ID**: TASK-012
&gt;
&gt; **功能名称**: 生产化高可用增强
&gt;
&gt; **预估工时**: 16h
&gt;
&gt; **难度**: ⭐⭐⭐ (中级)
&gt;
&gt; **技术栈**: 令牌桶、熔断器、灰度发布
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

生产环境需要：

- **限流**: 防止恶意请求和系统过载
- **熔断**: 故障时快速失败，防止雪崩
- **降级**: 核心功能优先，非核心降级
- **灰度发布**: 新功能渐进式上线

### 1.2 功能需求

| 功能点 | 说明 |
|--------|------|
| API 限流 | 基于 Token Bucket 或漏桶算法 |
| 租户限流 | 按租户/API Key 限流 |
| 熔断器 | Hystrix 或 Sentinel 模式 |
| 服务降级 | 缓存结果或默认结果 |
| 灰度发布 | 按流量比例或用户分组 |

---

## 二、为什么要做这个？

### 2.1 生产级必备

- 保证系统稳定性
- 防止服务雪崩
- 安全发布新功能

---

## 三、技术原理

### 3.1 令牌桶算法

```
         [令牌生成器] → 以恒定速率生成令牌
              ↓
         [令牌桶] → 存储令牌，有最大容量
              ↓
请求 → [取令牌] → 有令牌则通过，无令牌则拒绝
```

### 3.2 熔断器状态机

```
    ┌─────────────┐
    │   关闭状态   │ ←─ 正常服务
    └──────┬──────┘
           │ 失败率超过阈值
           ↓
    ┌─────────────┐
    │   打开状态   │ ←─ 快速失败
    └──────┬──────┘
           │ 经过冷却时间
           ↓
    ┌─────────────┐
    │  半开状态    │ ←─ 尝试少量请求
    └─────────────┘
           │
    ┌──────┴──────┐
    ↓             ↓
成功           失败
    │             │
[回到关闭]   [回到打开]
```

---

## 四、实现步骤

### Step 1: 实现令牌桶限流

```go
package ratelimit

import (
    "sync"
    "time"
)

type TokenBucket struct {
    capacity   int           // 桶容量
    tokens     int           // 当前令牌数
    rate       time.Duration // 令牌生成速率
    lastRefill time.Time     // 上次补充时间
    mu         sync.Mutex
}

func NewTokenBucket(capacity int, rate time.Duration) *TokenBucket {
    return &amp;TokenBucket{
        capacity:   capacity,
        tokens:     capacity,
        rate:       rate,
        lastRefill: time.Now(),
    }
}

func (tb *TokenBucket) Allow() bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    // 补充令牌
    now := time.Now()
    elapsed := now.Sub(tb.lastRefill)
    newTokens := int(elapsed / tb.rate)

    if newTokens &gt; 0 {
        tb.tokens = min(tb.capacity, tb.tokens+newTokens)
        tb.lastRefill = now
    }

    // 取令牌
    if tb.tokens &gt; 0 {
        tb.tokens--
        return true
    }
    return false
}

func min(a, b int) int {
    if a &lt; b {
        return a
    }
    return b
}
```

### Step 2: 实现熔断器

```go
package circuitbreaker

import (
    "sync"
    "time"
)

type State int

const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

type CircuitBreaker struct {
    state           State
    failureCount    int
    successCount    int
    failureThreshold int
    recoveryTime    time.Duration
    lastFailureTime time.Time
    mu              sync.Mutex
}

func NewCircuitBreaker(failureThreshold int, recoveryTime time.Duration) *CircuitBreaker {
    return &amp;CircuitBreaker{
        state:           StateClosed,
        failureThreshold: failureThreshold,
        recoveryTime:    recoveryTime,
    }
}

func (cb *CircuitBreaker) Allow() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    switch cb.state {
    case StateClosed:
        return true
    case StateOpen:
        // 检查是否进入半开状态
        if time.Since(cb.lastFailureTime) &gt; cb.recoveryTime {
            cb.state = StateHalfOpen
            cb.successCount = 0
            return true
        }
        return false
    case StateHalfOpen:
        // 半开状态只允许一个请求
        return cb.successCount == 0
    }
    return false
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if cb.state == StateHalfOpen {
        cb.successCount++
        if cb.successCount &gt;= 3 {
            cb.state = StateClosed
            cb.failureCount = 0
        }
    }
}

func (cb *CircuitBreaker) RecordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failureCount++

    if cb.state == StateClosed &amp;&amp; cb.failureCount &gt;= cb.failureThreshold {
        cb.state = StateOpen
        cb.lastFailureTime = time.Now()
    } else if cb.state == StateHalfOpen {
        cb.state = StateOpen
        cb.lastFailureTime = time.Now()
    }
}
```

### Step 3: 实现灰度发布

```go
package canary

import (
    "math/rand"
    "sync"
)

type CanaryRelease struct {
    percentage int // 灰度流量百分比 0-100
    whitelist  map[string]bool // 白名单用户
    mu         sync.Mutex
}

func NewCanaryRelease(percentage int, whitelist []string) *CanaryRelease {
    wl := make(map[string]bool)
    for _, user := range whitelist {
        wl[user] = true
    }
    return &amp;CanaryRelease{
        percentage: percentage,
        whitelist:  wl,
    }
}

func (cr *CanaryRelease) ShouldUseNewFeature(userID string) bool {
    cr.mu.Lock()
    defer cr.mu.Unlock()

    // 白名单用户直接使用新功能
    if cr.whitelist[userID] {
        return true
    }

    // 按概率判断
    return rand.Intn(100) &lt; cr.percentage
}

func (cr *CanaryRelease) SetPercentage(p int) {
    cr.mu.Lock()
    defer cr.mu.Unlock()
    cr.percentage = p
}
```

### Step 4: 集成到 API 中间件

```go
package middleware

import (
    "your-project/ratelimit"
    "your-project/circuitbreaker"
    "your-project/canary"
    "github.com/gin-gonic/gin"
)

func RateLimiter(tb *ratelimit.TokenBucket) gin.HandlerFunc {
    return func(c *gin.Context) {
        if !tb.Allow() {
            c.JSON(429, gin.H{
                "error": "Too many requests",
                "code":  "RATE_LIMIT_EXCEEDED",
            })
            c.Abort()
            return
        }
        c.Next()
    }
}

func CircuitBreaker(cb *circuitbreaker.CircuitBreaker) gin.HandlerFunc {
    return func(c *gin.Context) {
        if !cb.Allow() {
            c.JSON(503, gin.H{
                "error": "Service unavailable",
                "code":  "CIRCUIT_OPEN",
            })
            c.Abort()
            return
        }

        c.Next()

        // 记录结果
        if c.Writer.Status() &gt;= 500 {
            cb.RecordFailure()
        } else {
            cb.RecordSuccess()
        }
    }
}
```

---

## 五、验收标准

| 验收项 | 标准 |
|--------|------|
| 限流 | 超过阈值时正确返回 429 |
| 熔断 | 故障时快速失败，恢复时自动闭合 |
| 灰度 | 按配置比例分配流量 |

---

## 六、代码提交流程

```bash
git checkout -b feature/TASK-012-rate-limiting

git add .

git commit -m "feat: TASK-012 实现限流、熔断、降级与灰度发布

- 令牌桶限流算法
- 熔断器模式
- 灰度发布支持
- 中间件集成"

git push origin feature/TASK-012-rate-limiting
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 掌握限流算法
- ✅ 理解熔断器模式
- ✅ 学会灰度发布
- ✅ 成为生产化专家
