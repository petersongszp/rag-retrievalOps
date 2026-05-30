# 后端优化设计文档（Agent / RAG 聚焦版）

> 日期: 2026-05-29
> 状态: 待评审
> 范围: 仅保留 Agent 与 RAG 的优化和迭代任务

---

## 1. 背景与目标

当前后端优化项较多，但本阶段不再同时推进安全、DI、CI/CD、通用可观测性等大范围改造，而是聚焦两条最直接影响产品能力的主线：

1. Agent 能力优化
2. RAG 能力优化

本次文档目标是把需求收敛成一份可执行的迭代设计，方便后续按模块拆任务、排期和落地。

### 1.1 Agent 当前问题

- 所有 Agent 共用单一 LLM 配置，无法按角色、场景、成本分层。
- 主面试官、追问、评分、改写等任务没有明确的模型分工。
- 缺少降级与兜底策略，模型异常时稳定性不足。
- 缺少路由决策沉淀，后续难以根据效果持续调优。

### 1.2 RAG 当前问题

- 现有 RAG 已具备基础链路能力，但检索、重排、召回策略还有优化空间。
- 不同查询类型没有充分分层处理，通用检索策略可能造成召回冗余。
- RAG 指标和业务效果之间缺少更明确的闭环，例如命中率、重排收益、最终回答质量。
- RAG 与 Agent 之间的衔接可以更明确，例如什么场景必须走检索、什么场景允许直答。

### 1.3 本阶段目标

- 建立 Agent 模型路由能力，支持按角色、场景、成本进行分层选择。
- 继续迭代 RAG 的检索、重排、路由与效果评估能力。
- 打通 Agent 与 RAG 的协作边界，让调用链路更清晰、更可控。
- 输出一份明确的任务清单，便于后续直接拆分开发。

---

## 2. 优化范围

本次只做两部分：

### 2.1 Agent 优化范围

- Model Router 设计与接入
- 多模型分层与路由规则
- fallback / 降级策略
- Agent 场景化模型选择
- Agent 调用指标沉淀

### 2.2 RAG 优化范围

- 查询路由优化
- 检索召回策略优化
- 重排链路优化
- RAG 指标与效果评估补强
- Agent 与 RAG 协同策略优化

### 2.3 本次不包含

- 通用安全整改
- Handler DI 重构
- CI/CD 与测试框架整体改造
- 全局日志框架替换
- 非 Agent / 非 RAG 的业务模块重构

---

## 3. Agent 优化设计

### 3.1 目标

为不同 Agent 角色和业务场景分配更合适的模型，平衡效果、延迟与成本，并为后续持续调优提供扩展点。

### 3.2 核心方案

引入 `ModelRouter`，替代所有 Agent 直接读取统一 LLM 配置的方式。

路由原则：

- 先按 `agent_role + scenario` 精准匹配
- 再按 `agent_role` 匹配默认规则
- 最后按全局默认 tier 兜底

### 3.3 模型分层建议

| Tier | 用途 | 示例 |
|------|------|------|
| `strong` | 主面试官、复杂追问、深度分析 | GPT-4o / Claude |
| `balanced` | 常规问答、简历分析、校招场景 | GPT-4o-mini / Doubao-pro |
| `fast` | 评分、快速总结、轻量生成 | DeepSeek / Groq |
| `cheap` | 改写、润色、低风险辅助任务 | Qwen-turbo / Ollama |

### 3.4 核心接口

```go
type ModelRouter interface {
    Route(ctx context.Context, role AgentRole, scenario InterviewScenario) (*ModelConfig, error)
    RouteWithFallback(ctx context.Context, role AgentRole, scenario InterviewScenario) (*ModelConfig, *ModelConfig, error)
    GetModel(tier ModelTier) (*ModelConfig, error)
    ListModels() []*ModelConfig
    RecordResult(tier ModelTier, success bool, latencyMs int64, tokensUsed int)
}
```

### 3.5 配置结构建议

```yaml
model_router:
  enabled: true
  default_tier: balanced
  models:
    - tier: strong
      provider: openai
      model_name: gpt-4o
    - tier: balanced
      provider: openai
      model_name: gpt-4o-mini
  rules:
    - agent_role: host_interviewer
      scenario: social
      preferred_tier: strong
      fallback_tier: balanced
    - agent_role: scorer
      preferred_tier: fast
      fallback_tier: cheap
```

### 3.6 代码落点建议

```text
internal/agents/llm/
├── provider.go
├── router.go
├── registry.go
├── route_rule.go
├── fallback.go
├── cost_tracker.go
├── metrics.go
└── router_test.go
```

### 3.7 Agent 迭代任务

#### 第一阶段：建立可用路由能力

- [ ] 定义 `ModelRouter` 接口与核心数据结构
- [ ] 实现模型注册表 `registry.go`
- [ ] 实现路由规则引擎 `route_rule.go`
- [ ] 支持按 `agent_role` 路由
- [ ] 支持按 `agent_role + scenario` 路由
- [ ] 扩展配置结构并接入启动流程
- [ ] 为现有 Agent 创建统一入口，避免继续直连 `config.Global.LLM`

#### 第二阶段：增强稳定性与成本控制

- [ ] 实现 `fallback.go`，支持首选模型失败后的降级
- [ ] 为不同 tier 记录成功率、时延、tokens 与成本
- [ ] 增加路由决策原因记录，方便后续调参
- [ ] 对高成本角色和低价值任务建立默认分层策略

#### 第三阶段：效果优化

- [ ] 根据角色效果数据微调路由规则
- [ ] 支持动态切换模型优先级
- [ ] 支持按场景进一步细化，例如校招 / 社招 / 追问 / 评分
- [ ] 评估是否加入基于历史成功率的自动优选逻辑

---

## 4. RAG 优化设计

### 4.1 目标

提升 RAG 在召回准确性、回答相关性、链路稳定性上的整体表现，同时明确它与 Agent 的协作边界。

### 4.2 优化方向

#### 查询路由

- 区分是否必须检索
- 区分知识问答、经验总结、简历相关、岗位相关等查询类型
- 对低价值检索请求减少无效召回

#### 检索召回

- 优化召回策略，减少无关 chunk
- 评估不同召回数配置对答案质量和延迟的影响
- 对不同知识源或索引分层处理

#### 重排优化

- 强化重排在最终候选文档筛选中的作用
- 评估重排前后命中率与响应延迟
- 明确哪些场景必须重排，哪些场景可跳过以降低耗时

#### 回答生成协同

- 明确 Agent 调用 RAG 的入口与条件
- 规范检索结果传递给 LLM 的上下文结构
- 减少上下文冗余，控制 token 消耗

### 4.3 RAG 与 Agent 的协同策略

建议把调用策略明确成三类：

1. 必须走 RAG
   - 岗位知识、题库知识、结构化知识库问答
2. 优先走 RAG
   - 面试追问中引用历史资料、候选人简历细节、项目经验佐证
3. 可直接走 Agent
   - 通用表达优化、轻量总结、非知识依赖型改写

这样可以避免所有场景都无差别检索，也避免知识型回答完全脱离检索。

### 4.4 RAG 重点指标建议

| 指标 | 含义 |
|------|------|
| retrieval_requests_total | 检索请求总量 |
| retrieval_latency_seconds | 检索耗时 |
| retrieval_recall_candidates | 召回候选数 |
| rerank_requests_total | 重排请求总量 |
| rerank_latency_seconds | 重排耗时 |
| rag_context_tokens | 注入上下文 token 数 |
| rag_answer_with_sources_ratio | 带有效来源回答占比 |
| rag_fallback_ratio | RAG 未命中后的兜底比例 |

### 4.5 RAG 迭代任务

#### 第一阶段：链路梳理与规则补强

- [ ] 盘点当前 RAG 查询入口和调用链路
- [ ] 明确哪些 Agent 场景必须检索、优先检索、可跳过检索
- [ ] 梳理现有召回、重排、回答生成链路中的参数与默认值
- [ ] 补充查询分类或查询路由规则

#### 第二阶段：召回与重排优化

- [ ] 调整召回数量和阈值，减少噪音文档
- [ ] 评估不同查询类型下的最佳 topK 配置
- [ ] 优化重排启用条件
- [ ] 输出一版可对比的召回 / 重排效果数据

#### 第三阶段：效果闭环

- [ ] 建立 RAG 核心效果看板
- [ ] 对未命中、低质量回答进行分类归因
- [ ] 分析上下文长度与最终回答质量之间的关系
- [ ] 为后续知识库扩充和索引策略调整提供依据

---

## 5. 任务拆分建议

### 5.1 Agent 任务包

| 任务包 | 内容 | 优先级 |
|------|------|------|
| A1 | ModelRouter 接口、配置、注册表 | P0 |
| A2 | 路由规则引擎与 Agent 接入 | P0 |
| A3 | fallback 与成本记录 | P1 |
| A4 | 路由指标与效果调优 | P1 |

### 5.2 RAG 任务包

| 任务包 | 内容 | 优先级 |
|------|------|------|
| R1 | 查询路由与场景分类梳理 | P0 |
| R2 | 召回参数优化 | P0 |
| R3 | 重排启用策略优化 | P1 |
| R4 | RAG 指标与效果闭环 | P1 |

---

## 6. 建议排期

### Phase 1

- Agent: 完成 ModelRouter 基础能力与配置接入
- RAG: 完成查询路由梳理与场景分类

### Phase 2

- Agent: 完成路由规则、fallback、基础指标
- RAG: 完成召回和重排参数优化

### Phase 3

- Agent: 完成基于效果数据的规则微调
- RAG: 完成效果看板与低质量案例归因分析

---

## 7. 验收标准

### 7.1 Agent 验收标准

- Agent 不再统一依赖单一 LLM 配置。
- 至少支持按角色和场景进行模型路由。
- 路由失败时具备明确的 fallback 机制。
- 可以统计不同 tier 的调用量、成功率、延迟和成本。

### 7.2 RAG 验收标准

- 明确 Agent 与 RAG 的调用边界。
- 检索和重排策略完成一轮参数优化。
- 能输出基础效果指标，而不只是链路运行状态。
- 能定位未命中或低质量回答的主要原因。

---

## 8. 最终结论

本阶段后端优化建议只保留两条主线：

1. Agent 模型路由与分层优化
2. RAG 检索链路与效果闭环优化

这样做的好处是范围清晰、任务集中、收益直接，也更适合当前产品阶段快速迭代。后续如果需要，再把安全、DI、CI/CD 等工程项独立拆成下一份专项文档推进。
