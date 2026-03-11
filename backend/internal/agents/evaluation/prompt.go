package evaluation

import "fmt"

// EvaluatorInstruction Evaluator Agent 的系统提示词
const EvaluatorInstruction = `你是一位资深的技术面试评估专家，负责对候选人的回答进行客观、专业的评分。

你的任务是：
1. 根据面试领域和当前话题，评估候选人回答的质量
2. 从多个维度给出 1-10 分的评分
3. 识别回答中涉及的技术知识点
4. 建议下一步的面试策略

评分标准：
- 1-3分：回答错误或完全不了解
- 4-5分：基本了解但不够深入
- 6-7分：理解正确，有一定深度
- 8-9分：理解深入，有实践经验
- 10分：专家级理解，见解独到

重要：
- 保持客观公正，不要因为回答简短就给低分
- 关注回答的质量而非长度
- 只返回 JSON 格式，不要有任何其他文字`

// BuildEvaluationPrompt 构建评分请求的 Prompt
func BuildEvaluationPrompt(req *EvaluationRequest) string {
	return fmt.Sprintf(`请对以下面试对话进行评分：

【面试领域】%s
【当前话题】%s

【面试官问题】
%s

【候选人回答】
%s

请从以下维度评分（1-10分）：
1. 正确性(correctness)：技术概念是否准确
2. 深度(depth)：是否展现对原理的深入理解
3. 完整性(completeness)：是否覆盖了问题的关键点
4. 实践性(practicality)：是否体现实际项目经验

请识别回答中覆盖的技术知识点，并建议下一步动作：
- "deepen": 候选人回答优秀(总分>=8)，应深入追问相关话题
- "continue": 候选人回答合格(4<=总分<8)，继续当前话题的其他问题
- "lower": 候选人回答较差(总分<4)，应降低难度或给予提示
- "switch": 当前话题已充分评估或者回答不上来，可切换到新话题。

返回JSON格式（只返回JSON，不要任何其他内容）：
{
  "scores": {
    "correctness": <1-10>,
    "depth": <1-10>,
    "completeness": <1-10>,
    "practicality": <1-10>
  },
  "overall": <加权总分1-10>,
  "covered_topics": ["知识点1", "知识点2"],
  "next_action": "<deepen|continue|lower|switch>",
  "reason": "<简短的评分理由，20字以内>"
}`,
		req.Domain,
		req.CurrentTopic,
		req.Question,
		req.Answer,
	)
}

// DomainTopicHints 各领域的知识点提示（帮助 LLM 更准确地识别知识点）
var DomainTopicHints = map[string]string{
	"Go": `Go 领域常见知识点包括：
- 并发编程：Goroutine、Channel、sync包、Context、并发模式、调度器
- 内存管理：GC机制、内存分配、逃逸分析、pprof、性能调优
- 标准库：net/http、io、encoding/json、database/sql、reflect
- 工程实践：项目结构、错误处理、测试、依赖管理`,

	"Java": `Java 领域常见知识点包括：
- JVM：内存模型、GC算法、类加载、JIT编译、调优
- 并发：线程池、锁机制、原子类、并发容器、volatile
- 框架：Spring IoC/AOP、MyBatis、SpringBoot、Spring Cloud
- 性能：调优方法、监控工具、故障排查`,

	"MySQL": `MySQL 领域常见知识点包括：
- 索引：B+树、覆盖索引、索引下推、索引优化
- 事务：ACID、隔离级别、MVCC、锁机制
- 架构：主从复制、分库分表、读写分离
- 优化：慢查询分析、执行计划、SQL优化`,

	"Redis": `Redis 领域常见知识点包括：
- 数据结构：String、Hash、List、Set、ZSet、Stream
- 持久化：RDB、AOF、混合持久化
- 集群：主从、哨兵、Cluster
- 应用：缓存穿透/击穿/雪崩、分布式锁、消息队列`,

	"MQ": `MQ 领域常见知识点包括：
- 基础：生产者、消费者、Topic、队列、消息模型
- 可靠性：消息确认、重试机制、幂等性、事务消息
- 架构：集群、分区、副本、负载均衡
- 产品对比：Kafka、RabbitMQ、RocketMQ 特性差异`,
}
