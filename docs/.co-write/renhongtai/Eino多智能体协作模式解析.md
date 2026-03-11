# Eino 多智能体协作模式深度解析

在构建复杂的 AI 应用（如智能面试系统）时，单个智能（Single Agent）往往难以平衡“流程控制”与“专业深度”。Eino 框架通过其 Agent Development Kit (ADK) 提供了灵活的多智能体协作模式。以下是三种核心协作模式的深度解析及其在面试场景中的应用。

---

## 1. 主从模式 (Host-Specialist / Orchestrator)

### 核心原理
这是最常用的模式，类似于“主面试官 + 专项面试官”的组合。一个 **Host Agent (主控)** 负责整体流程和意图识别，它拥有多个“工具”，而这些工具的背后实际上是其他的 **Specialist Agents (专家)**。

- **Host**: 负责开场、总结、并在合适的时间点调用对应的专家。
- **Specialists**: 专注于某一垂直领域（如算法、系统设计、HR 常识）。
- **Eino 实现关键**: 使用 `adk.NewAgentTool` 将 Agent 包装为 `Tool`，供 Host Agent 调用。

### 面试场景应用
**“多对一”专家面试**：
- **Host**: 负责控制面试时长，在候选人回答完后决定下一步是考算法还是考架构。
- **Algorithm Agent**: 专门负责从题目库中抽题并进行深度追问。
- **System Design Agent**: 专门负责考察高并发和架构设计。

---

## 2. 移交模式 (Handover / Routing)

### 核心原理
控制权在 Agent 之间直接流转，不一定存在一个永久的“中心化”主控。当 A Agent 完成其阶段性任务，或者发现任务超纲时，直接将上下文移交给 B Agent，后续由 B 接手。

- **流转逻辑**: 基于状态机（State Machine）或意图识别（Router）。
- **Eino 实现关键**: 在 `compose.Graph` 中连接不同 Agent 节点，或使用 `adk.NewTransferToAgentAction` 实现显式移交。

### 面试场景应用
**“接力式”面试环节**：
1. **Resume Agent**: 首先分析简历，提取关键词。然后“移交”给...
2. **Technical Agent**: 开始技术面试。面试结束后“移交”给...
3. **Evaluation Agent**: 进行最后的评分和总结。
*特点：每个阶段 Agent 全权负责，直到触发移交条件。*

---

## 3. 工作流/编排模式 (Workflow / Direct Graph)

### 核心原理
这是一种确定性更强的协作模式。将复杂的业务逻辑拆解为一系列串行（Sequential）、并行（Parallel）或循环（Loop）的节点。

- **确定性**: 开发者精准控制 Agent 之间的信息流向。
- **Eino 实现关键**: 利用 `adk.NewSequentialAgent`、`adk.NewParallelAgent` 或最灵活的 `compose.NewGraph()`。

### 面试场景应用
**“背对背”面试评估**：
- **并行执行**: 针对同一个面试片段，同时让 **Standardization Agent**（核对答案准确性）和 **Sentiment Agent**（分析候选人情绪/自信度）进行分析。
- **聚合汇总**: 两个 Agent 运行完后，由一个聚合节点生成最终的综合报告。

---

## 模式选择指南

| 维度 | 主从模式 (Host-Specialist) | 移交模式 (Handover) | 工作流模式 (Workflow) |
| :--- | :--- | :--- | :--- |
| **控制中心** | 有（Host） | 无/动态 | 静态拓扑 |
| **灵活性** | 极高（由 LLM 决定调谁） | 高 | 中（流程相对固定） |
| **适用场景** | 复杂的动态对话、专家调度 | 流程分段明确、长路径任务 | 高性能并行分析、确定性流程 |
| **Eino 组件** | `AgentAsTool` | `TransferToAgent` | `Graph` / `Chain` |

---
*本文档由 Antigravity 整理，旨在为「面试吧」项目提供多智能体架构设计的理论支撑。*
