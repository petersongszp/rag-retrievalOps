# 社招简历 Agent 接入 RAG 需求实现路线（试运行版）

## 1. 文档定位

本文档是“把已完成的 Milvus/RAG 能力接入面试吧社招简历 Agent”的执行手册，目标是让 `comprehensive_social` 在生成面试问题时，可以受控使用 `get_resume_info`、`get_milvus_retriever` 与现有 RAG 检索链路。

它有两个用途：

1. 作为第一版社招简历 Agent 接入 RAG 的统一需求与实现文档。
2. 作为后续“面试 Agent 检索增强、题目质量评估、RAG 证据回放”的基线文档。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `社招简历 Agent` 固定指 `backend/internal/agents/interview/comprehensive/social_comprehensive_agent.go` 中的 `SocialComprehensiveAgent`。
2. `RAG 增强提问` 固定指 Agent 生成问题前，可根据简历、岗位、当前话题、最近问答历史检索 Milvus 知识库，并用检索证据校准提问方向、难度和追问深度。
3. `最终输出契约` 固定指 SSE 面试问题仍只输出一条自然语言问题，不输出 JSON、Markdown、引用列表或调试字段。
4. `检索证据` 固定指 `get_milvus_retriever` 返回的 `documents/content/metadata/score`，以及底层 RAG 的 `source/route/retriever_version/rerank_score/parent_id/child_id` 等字段。
5. `安全降级` 固定指 Milvus/RAG 不可用、超时、空结果、证据不足时，Agent 必须退回“简历 + 历史问答”的原有提问路径。
6. `结构化链路日志` 固定至少包含：`session_id/record_id/question_index/agent_type/resume_id/user_id/query/request_id/retrieve_count/retrieve_status/rag_used/fallback_reason/duration_ms`。
7. `试运行` 固定指默认只对 `comprehensive_social` 生效，不影响校招综合、专项面试、群面、多 Agent 编排。

---

## 2. 当前项目现状

## 2.1 已有基础能力（可复用）

1. 社招综合 Agent 已存在：`NewSocialComprehensiveAgent(userId, needResumeTool)`。
2. 简历工具已存在：`GetResumeInfoTool()`，工具名为 `get_resume_info`。
3. Milvus 检索工具已存在：`GetMilvusRetrieverTool()`，工具名为 `get_milvus_retriever`。
4. 专项面试 Agent 已经接入 Milvus 工具，可作为社招综合 Agent 的最小接入参考。
5. RAG 检索链路已具备 Phase 2/Phase 3 能力：
   - hybrid retrieval
   - query rewrite
   - dynamic/strategic TopK
   - parent-child retrieval
   - evidence refusal
   - citation consistency
6. 面试主流程已使用 Graph 编排：`RunInterviewLoopWithGraph`。
7. 面试状态中已有 `SessionID/RecordID/ResumeId/Domain/Difficulty/QuestionIndex/RecentHistory/CurrentTopic`，可用于构造检索上下文。
8. RAG 侧已有检索日志模型：`KBRetrieveLog`。
9. RAG 侧已有 Prometheus 指标：`rag_retrieve_*`。

## 2.2 当前关键缺口（本阶段需要补齐）

1. `SocialComprehensiveAgent` 当前只在 `needResumeTool=true` 时挂载 `get_resume_info`，尚未挂载 `get_milvus_retriever`。
2. Agent 提示词没有明确“什么时候检索、检索什么、如何使用证据、什么时候降级”的规则。
3. `get_milvus_retriever` 输入目前只有 `query`，缺少面试场景所需的 `session_id/question_index/domain/position/resume_id/request_id/top_k` 等上下文。
4. 面试 Graph 还没有把 RAG 使用情况与 `session_id/question_index` 串起来。
5. 面试最终问题没有记录“是否使用 RAG、命中哪些证据、为什么降级”。
6. 缺少面向社招简历 Agent 的离线评测集与题目质量验收标准。

---

## 3. 范围边界

## 3.1 本阶段必须完成

1. 为 `SocialComprehensiveAgent` 接入 Milvus/RAG 检索工具，并受 feature flag 控制。
2. 明确社招 Agent 的 RAG 使用策略：优先围绕简历项目、技术栈、岗位方向、当前话题检索面试知识库。
3. 保持最终问题输出契约不变：只输出一个问题，不暴露检索 JSON 或 citation。
4. 打通最小链路日志：一次问题生成能追踪到是否调用 RAG、检索 query、命中数量和降级原因。
5. 建立安全降级路径：RAG 失败不影响面试继续。
6. 建立小规模试运行验收集，验证 RAG 增强后问题更贴近简历、岗位和技术深度。
7. 明确灰度、回滚和后续扩展路线。

## 3.2 本阶段明确不做

1. 不重做 RAG 检索底层能力。
2. 不要求所有 Agent 全量接入 RAG。
3. 不在第一版把 citation 展示给面试用户。
4. 不让大模型自由决定“证据是否足够”，第一版以工具返回、分数、空结果和规则降级为主。
5. 不把 RAG 命中内容直接拼成大段背景灌给用户。
6. 不改变现有 SSE 输出协议。
7. 不在第一版引入自动 AB 平台，只做可控灰度和人工验收。

---

## 4. 目标与通过标准（Gate）

本阶段通过标准（全满足）：

1. `comprehensive_social` 在开关开启时可同时使用 `get_resume_info` 和 `get_milvus_retriever`。
2. 开关关闭后行为回到当前纯简历提问路径，不影响校招、专项、群面 Agent。
3. 首题和后续题都能根据简历项目、岗位方向、当前话题构造检索 query，并把检索结果用于提问方向选择。
4. RAG 失败、超时、空结果时不阻断面试，最终仍能生成自然语言问题。
5. 最终输出仍满足原契约：只输出一个问题，不输出 JSON、Markdown、工具结果或引用列表。
6. 至少 30 条社招简历场景离线样例中，RAG 增强问题的“简历相关性”和“技术追问深度”优于当前基线。
7. 单题生成 P95 延迟退化在可接受范围内，建议门槛：相比纯简历路径不超过 30%。
8. 一次问题生成可通过日志还原：`resume_id -> query -> retrieve result -> question -> fallback reason`。

---

## 5. 实现路线总览（L0 -> L8）

本阶段按 9 条路线推进，按门禁顺序合流：

1. L0：接入基线冻结、开关与契约定义
2. L1：社招综合 Agent 工具挂载与最小可用链路
3. L2：社招 RAG 提问提示词协议
4. L3：检索 query 构造与上下文选择策略
5. L4：检索结果压缩、证据使用与问题生成约束
6. L5：Graph/session 链路追踪与日志落点
7. L6：降级、超时、空结果与安全控制
8. L7：离线评测集、题目质量回归与验收样例
9. L8：灰度试运行、回滚预案与验收收口

建议顺序：`L0 -> L1 + L2 -> L3 + L4 -> L5 + L6 -> L7 -> L8`

---

## 6. 详细路线拆解

## 6.1 L0 接入基线冻结、开关与契约定义

### 目标
在接入前冻结当前社招面试提问行为，新增独立开关，确保第一版接入可灰度、可回滚、可对比。

### 功能任务

1. 固定当前基线：
   - `SocialComprehensiveAgent` 当前工具列表
   - 当前 `SocialComprehensiveAgentInstruction`
   - 当前 Graph 首题/后续题 prompt
   - 当前首题、追问、切换话题样例
2. 新增或约定 Feature Flag：
   - `INTERVIEW_ENABLE_SOCIAL_RAG`
   - `INTERVIEW_SOCIAL_RAG_SHADOW_ONLY`
   - `INTERVIEW_SOCIAL_RAG_MAX_TOOL_CALLS`
   - `INTERVIEW_SOCIAL_RAG_TIMEOUT_MS`
3. 复用已有 RAG 开关作为底层能力控制：
   - `RAG_ENABLE_HYBRID_RETRIEVAL`
   - `RAG_ENABLE_QUERY_REWRITE`
   - `RAG_ENABLE_DYNAMIC_TOPK`
   - `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
   - `RAG_ENABLE_EVIDENCE_REFUSAL`
4. 固定最终输出契约：
   - 只输出一条问题
   - 不输出 JSON
   - 不输出 Markdown
   - 不输出 citation
   - 不输出“我检索到”
5. 固定日志字段：
   - `session_id`
   - `record_id`
   - `question_index`
   - `agent_type`
   - `resume_id`
   - `rag_enabled`
   - `rag_used`
   - `rag_query`
   - `rag_request_id`
   - `rag_result_count`
   - `fallback_reason`

### 验收

1. 关闭 `INTERVIEW_ENABLE_SOCIAL_RAG` 后，社招提问行为与当前基线一致。
2. 开关只影响 `comprehensive_social`，不影响专项面试 Agent 已有 Milvus 工具行为。
3. 最终输出契约有单测或快照样例覆盖。
4. 新增开关缺省值安全：默认关闭或仅开发环境开启。

---

## 6.2 L1 社招综合 Agent 工具挂载与最小可用链路

### 目标
让 `SocialComprehensiveAgent` 在开关开启后具备调用 Milvus 检索工具的能力，并保留原有简历工具。

### 功能任务

1. 修改 `backend/internal/agents/interview/comprehensive/social_comprehensive_agent.go`：
   - 创建 `milvusTool := tool2.GetMilvusRetrieverTool()`
   - 组装 `tools := []componenttool.BaseTool{}`
   - 当 `needResumeTool=true` 时追加 `GetResumeInfoTool()`
   - 当 `INTERVIEW_ENABLE_SOCIAL_RAG=true` 时追加 `milvusTool`
2. 保持工具初始化失败可控：
   - RAG 开关关闭时不初始化 Milvus 工具
   - RAG 开关开启但工具创建失败时返回可读错误或降级为无 RAG Agent
3. 明确第一版工具调用约束：
   - 每轮最多调用 1 次 `get_resume_info`
   - 每轮最多调用 1 次 `get_milvus_retriever`
   - 首题必须先读取简历再决定是否检索
   - 后续题优先结合最近问答历史再检索
4. 补充最小单测：
   - 开关关闭时工具列表不包含 Milvus
   - 开关开启时工具列表包含 Milvus
   - `needResumeTool=false` 时不强行调用简历工具

### 验收

1. 本地启动后社招 Agent 可成功创建。
2. 开关开启时 Agent 可以调用 `get_milvus_retriever`。
3. 开关关闭时 Agent 仍只使用原有简历工具。
4. Milvus 工具不可用时面试流程不会崩溃。

---

## 6.3 L2 社招 RAG 提问提示词协议

### 目标
让模型知道什么时候检索、检索什么、如何使用检索结果，以及如何保持最终问题输出干净。

### 功能任务

1. 扩展 `SocialComprehensiveAgentInstruction`，新增“RAG 使用规则”：
   - 当问题涉及候选人简历中的技术栈、项目架构、性能优化、故障处理、分布式系统、中间件、数据库时，优先检索知识库。
   - 检索 query 应由“简历关键词 + 当前话题 + 岗位方向 + 面试难度”组成。
   - 检索结果只用于选择提问角度，不要直接复述知识库内容。
   - 如果检索为空或证据弱，退回简历和历史问答，不要编造知识库内容。
2. 增加工具调用顺序规则：
   - 首题：`get_resume_info -> get_milvus_retriever -> 生成问题`
   - 后续题：`最近回答分析 -> 必要时 get_milvus_retriever -> 生成追问`
3. 增加问题生成约束：
   - 问题必须指向候选人真实项目或技术栈
   - 问题不能变成知识问答背诵
   - 问题要能引导候选人讲实际决策、取舍、排障和结果
4. 增加语言约束：
   - 仍复用 `common.LanguageAdaptiveInstruction`
   - 候选人使用中文则中文提问，候选人使用英文则英文提问

### 验收

1. Agent 不会把检索 JSON、citation、工具名称输出给用户。
2. Agent 生成的问题能体现简历项目和检索知识的结合。
3. RAG 空结果时，问题仍自然且不暴露“未检索到”。
4. 首题不会只问泛泛的“介绍项目”，而是能基于简历技术栈形成切入点。

---

## 6.4 L3 检索 query 构造与上下文选择策略

### 目标
避免把整份简历直接丢给检索工具，改为生成短而精准的检索 query，提高命中率与稳定性。

### 功能任务

1. 定义 query 构造输入：
   - `resume_id`
   - `position_name`
   - `company_name`
   - `difficulty`
   - `domain`
   - `current_topic`
   - `question_index`
   - 最近 1 到 2 轮问答
   - 简历中的核心技术栈与项目关键词
2. 定义首题 query 模板：
   - `社招 {岗位} {核心技术栈} {核心项目场景} 面试考察点 架构 取舍`
3. 定义追问 query 模板：
   - `{当前话题} {候选人回答关键词} 深入追问 原理 边界 性能 故障处理`
4. 定义切换话题 query 模板：
   - `{岗位方向} {未覆盖技术栈} 社招面试 高频问题 实战场景`
5. 工具输入演进建议：
   - 第一版可继续只传 `query`
   - 第二版扩展 `MilvusRetrieverInput`，增加 `scene/session_id/question_index/top_k/request_id`
6. 检索范围策略：
   - 第一版默认使用当前 Milvus collection 的全局面试知识库
   - 后续支持按 `language/category/document_tag/kb_scope/active_global_kb_id` 过滤

### 验收

1. Query 长度可控，避免整段简历进入检索。
2. Query 能覆盖技术栈、项目场景和当前话题。
3. 对 Go/Java/MySQL/Redis/MQ 等技术方向能形成不同 query。
4. 日志可看到每轮实际使用的 `rag_query`。

---

## 6.5 L4 检索结果压缩、证据使用与问题生成约束

### 目标
让 RAG 结果成为“提问依据”，而不是让模型把知识库内容直接塞进问题里。

### 功能任务

1. 定义证据压缩规则：
   - 保留 Top 3 到 Top 5 个片段
   - 每个片段只抽取：技术点、考察方向、常见追问、风险点
   - 超长内容截断，不超过单轮上下文预算
2. 定义证据使用方式：
   - 用于选择问题角度
   - 用于校准难度
   - 用于补齐候选人回答中的薄弱点
   - 不用于直接给答案
3. 定义问题风格：
   - 从候选人项目进入
   - 问决策依据、边界情况、性能瓶颈、故障恢复、团队协作
   - 避免纯概念题
4. 定义证据不足处理：
   - 检索为空：按简历继续问
   - 分数低：只作为弱参考，不强行提及
   - 内容与简历不相关：丢弃
5. 增加 prompt 内部规则：
   - “不要说根据知识库”
   - “不要引用文档标题”
   - “不要输出多个问题”

### 验收

1. 生成的问题不会泄露知识库原文。
2. 生成的问题明显比纯简历问题更贴近真实社招考察。
3. 低质量检索结果不会导致问题跑偏。
4. 候选人听到的是自然面试问题，而不是检索摘要。

---

## 6.6 L5 Graph/session 链路追踪与日志落点

### 目标
让一次社招提问是否使用 RAG 可以被复盘，便于调试问题质量、延迟和回滚。

### 功能任务

1. 在 Graph 问题生成上下文中生成 `rag_request_id`：
   - 建议格式：`interview-{session_id}-q{question_index}`
2. 将 `session_id/question_index/agent_type` 注入 context 或 prompt 元信息。
3. 在工具层补充日志：
   - `scene=interview_social`
   - `request_id`
   - `user_id`
   - `resume_id`
   - `question_index`
   - `query`
   - `result_count`
   - `duration_ms`
4. 优先复用 `KBRetrieveLog`：
   - 若 `request_id` 可控，则检索日志与面试轮次可关联。
   - 若暂时无法注入，则先在应用日志中记录映射关系。
5. 后续可新增轻量表：
   - `interview_question_rag_trace`
   - 字段：`session_id/record_id/question_index/request_id/rag_query/rag_result_count/rag_used/fallback_reason`
6. SSE 不暴露调试字段，调试信息只进入后端日志或 Admin 检索日志页面。

### 验收

1. 通过 `session_id + question_index` 能找到对应 RAG 检索记录。
2. 空结果、超时、降级都有明确原因。
3. 题目质量回放时能看到“模型为什么问了这个方向”的检索依据。
4. 日志不包含敏感简历全文，只记录必要关键词和 ID。

---

## 6.7 L6 降级、超时、空结果与安全控制

### 目标
把 RAG 作为增强能力，而不是面试主链路的单点风险。

### 功能任务

1. 超时控制：
   - 默认复用 `RAG_RETRIEVE_TIMEOUT_MS`
   - 社招 Agent 可单独使用 `INTERVIEW_SOCIAL_RAG_TIMEOUT_MS`
   - 超时后继续生成问题
2. 空结果分类：
   - `No-Retrieval-Hit`
   - `Low-Score`
   - `Tool-Error`
   - `Timeout`
   - `Irrelevant-Evidence`
3. 调用次数限制：
   - 每轮最多 1 次检索
   - 单场面试可设置最大检索次数
4. 内容安全：
   - 不把用户隐私信息作为检索 query 的主体
   - query 只保留技术栈、项目类型、岗位方向、回答关键词
5. 回滚顺序：
   - 关闭 `INTERVIEW_SOCIAL_RAG_SHADOW_ONLY`
   - 关闭 `INTERVIEW_ENABLE_SOCIAL_RAG`
   - 保留 `get_resume_info`
   - 回退到当前社招简历提问路径

### 验收

1. Milvus 未初始化时社招面试仍可继续。
2. 检索超时时最终问题仍能输出。
3. RAG 异常不会导致 SSE 中断或 Graph 失败。
4. 关闭开关后 10 分钟内可回到当前稳定路径。

---

## 6.8 L7 离线评测集、题目质量回归与验收样例

### 目标
用小而准的评测集判断 RAG 接入是否真的提升社招面试问题质量，而不是只验证“工具能调用”。

### 功能任务

1. 构建社招简历评测集：
   - Go 后端 3 年经验
   - Java 微服务 5 年经验
   - MySQL 调优经验
   - Redis 缓存与高可用经验
   - MQ/Kafka 经验
   - 分布式系统与故障处理经验
2. 每条样例包含：
   - `resume_summary`
   - `position_name`
   - `difficulty`
   - `recent_history`
   - `expected_focus`
   - `bad_question_examples`
   - `golden_question_traits`
3. 对比实验：
   - `baseline_resume_only`
   - `candidate_resume_rag_tool`
   - `candidate_resume_rag_shadow`
4. 质量指标：
   - 简历相关性
   - 技术深度
   - 实战导向
   - 非重复性
   - 语言自然度
   - 输出契约合规率
5. 工程指标：
   - 工具调用成功率
   - RAG 使用率
   - RAG 空结果率
   - 单题生成 P95 延迟
   - fallback rate

### 验收

1. 至少 30 条样例完成 baseline/candidate 对比。
2. Candidate 的简历相关性和技术深度有可观察提升。
3. 输出契约合规率保持 100%。
4. P95 延迟退化不超过预设阈值。
5. fallback 场景问题质量不低于基线。

---

## 6.9 L8 灰度试运行、回滚预案与验收收口

### 目标
用最小风险把社招 RAG 提问能力接入真实流程，并确保可回滚、可复盘、可继续扩展。

### 功能任务

1. 灰度顺序：
   - 本地开发环境手工验证
   - 内部账号开启
   - `INTERVIEW_SOCIAL_RAG_SHADOW_ONLY=true` 影子检索
   - 小流量真实社招简历面试开启
   - 稳定后扩大到全部社招简历面试
2. 观测指标：
   - `rag_used_rate`
   - `rag_fallback_rate`
   - `rag_empty_rate`
   - `question_generation_p95`
   - `tool_error_rate`
   - `output_contract_violation_count`
3. 告警规则：
   - RAG 工具错误率异常升高
   - 社招问题生成失败率升高
   - P95 延迟超过阈值
   - 输出中出现 JSON/citation/工具结果
4. 回滚预案：
   - 关闭 `INTERVIEW_ENABLE_SOCIAL_RAG`
   - 保留 `RAG_*` 底层能力供其他模块使用
   - 若专项 Agent 受影响，再按 RAG Phase 2/3 回滚开关逐项关闭
5. 输出验收报告：
   - 功能完成情况
   - 质量对比
   - 延迟与失败率
   - 典型好案例
   - 典型坏案例
   - 是否进入下一阶段

### 验收

1. 灰度期间社招面试主流程稳定。
2. RAG 增强题目有正向样例支撑。
3. 回滚演练成功。
4. 验收报告可支撑下一阶段扩展到校招、专项或群面。

---

## 7. 推荐实施节奏

## 7.1 阶段推进建议

1. 先完成 `L0`，冻结当前基线、开关和输出契约。
2. 再完成 `L1 + L2`，让社招 Agent 能用工具，并让模型知道如何正确使用工具。
3. 然后完成 `L3 + L4`，让检索 query 和证据使用变得稳定可控。
4. 再完成 `L5 + L6`，补齐可观测、降级和安全控制。
5. 最后完成 `L7 + L8`，用评测集和灰度数据决定是否扩大范围。

## 7.2 并行与合流规则

1. 可并行：`L2` 提示词协议与 `L7` 评测集准备。
2. 可并行：`L5` 日志字段设计与 `L6` 降级策略设计。
3. 必须串行：`L1` 工具挂载必须先于真实灰度。
4. 必须串行：`L8` 必须等 `L0~L7` 全部通过后执行。
5. 合流规则：任何输出契约破坏都必须阻断灰度。

---

## 8. 角色分工（建议）

1. 后端 Agent：L1 + L2，负责 `SocialComprehensiveAgent` 工具接入与提示词协议。
2. 后端 Graph：L3 + L5，负责面试状态、query 构造、request_id 和日志串联。
3. RAG/检索：L4 + L6，负责检索结果压缩、证据筛选、超时与降级策略。
4. QA/产品：L7，负责社招简历样例、题目质量标准和人工验收。
5. SRE/后端：L8，负责灰度开关、指标、告警和回滚演练。

补充协作约束：

1. Agent 和 RAG 先冻结“最终问题只输出一条自然语言问题”的契约。
2. Graph 和 RAG 先冻结 `request_id` 规则，避免日志无法关联。
3. QA 先提供至少 10 条坏问题样例，防止 RAG 接入后问题变成概念背诵。

---

## 9. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0~L8）
2. 开关配置快照：
   - `INTERVIEW_ENABLE_SOCIAL_RAG`
   - `INTERVIEW_SOCIAL_RAG_SHADOW_ONLY`
   - `INTERVIEW_SOCIAL_RAG_TIMEOUT_MS`
   - 关键 `RAG_*` 开关
3. 对比结果：
   - `baseline_resume_only`
   - `candidate_resume_rag_tool`
   - `candidate_resume_rag_shadow`
4. 题目质量指标：
   - 简历相关性
   - 技术深度
   - 实战导向
   - 非重复性
   - 语言自然度
   - 输出契约合规率
5. 工程指标：
   - RAG 使用率
   - RAG fallback rate
   - RAG 空结果率
   - 工具调用错误率
   - 单题生成 P95 延迟
6. 典型好案例：
   - 简历背景
   - 检索 query
   - 命中方向
   - 生成问题
7. 典型坏案例：
   - 问题现象
   - 检索结果
   - 失败原因
   - 修复建议
8. 灰度与回滚演练结果（成功/失败 + 原因）
9. 遗留问题与负责人
10. 是否进入下一阶段（是/否）

---

## 10. 本阶段完成后下一步

完成本阶段 Gate 后，下一阶段建议按以下顺序推进：

1. 把 `MilvusRetrieverInput` 从单字段 `query` 升级为结构化输入。
2. 把 `interview_question_rag_trace` 或等价日志落库做成可检索回放。
3. 将 RAG 增强从社招综合扩展到校招综合。
4. 将专项 Agent 的现有 Milvus 工具调用纳入同一套日志和评测口径。
5. 在 Admin 侧增加“面试题 RAG 调试视图”。
6. 评估是否把 citation 用于后台质检，而不是直接展示给面试用户。

---

## 11. 文档维护规则

1. 任何社招 RAG 接入范围变更，先更新本文档再改代码。
2. 新增开关、工具字段、日志字段必须同步更新 L0/L3/L5/L8。
3. 提示词协议变更必须同步补充至少 3 条验收样例。
4. 灰度或回滚后必须补充“阶段验收模板”。
5. 后续实现时，以本文档作为社招简历 Agent 接入 RAG 的第一版参考基线。
