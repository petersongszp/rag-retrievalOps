package topic

// TopicTrees 各领域的话题树定义
// 结构: Domain -> MainTopic -> SubTopics
var TopicTrees = map[string]map[string][]string{
	"Go": {
		"并发编程": {
			"Goroutine原理",
			"Channel机制",
			"sync包使用",
			"Context机制",
			"并发模式",
			"调度器GMP",
			"协程泄露与排查",
		},
		"内存管理": {
			"GC机制",
			"内存分配器",
			"逃逸分析",
			"pprof工具",
			"内存优化",
			"对象池sync.Pool",
		},
		"标准库": {
			"net/http",
			"io接口",
			"encoding/json",
			"database/sql",
			"reflect反射",
			"time定时器",
		},
		"工程实践": {
			"项目结构",
			"错误处理",
			"单元测试",
			"基准测试",
			"依赖管理go mod",
			"代码规范",
		},
		"网络编程": {
			"TCP编程",
			"HTTP服务",
			"gRPC",
			"WebSocket",
			"连接池",
		},
	},

	"Java": {
		"JVM": {
			"内存模型",
			"GC算法",
			"类加载机制",
			"JIT编译",
			"JVM调优",
			"内存泄露排查",
		},
		"并发编程": {
			"线程池",
			"锁机制synchronized",
			"Lock接口",
			"原子类",
			"并发容器",
			"volatile关键字",
			"ThreadLocal",
		},
		"Spring框架": {
			"IoC容器",
			"AOP原理",
			"Bean生命周期",
			"事务管理",
			"SpringBoot自动配置",
			"Spring Cloud组件",
		},
		"性能优化": {
			"JVM调优",
			"数据库优化",
			"缓存策略",
			"监控工具",
			"故障排查",
			"压测方法",
		},
		"设计模式": {
			"单例模式",
			"工厂模式",
			"代理模式",
			"观察者模式",
			"策略模式",
		},
	},

	"MySQL": {
		"索引机制": {
			"B+树原理",
			"索引类型",
			"覆盖索引",
			"索引下推",
			"索引优化",
			"索引失效场景",
		},
		"事务机制": {
			"ACID特性",
			"隔离级别",
			"MVCC原理",
			"锁机制",
			"死锁检测",
			"事务日志",
		},
		"查询优化": {
			"执行计划分析",
			"慢查询优化",
			"SQL改写",
			"Join优化",
			"分页优化",
		},
		"架构设计": {
			"主从复制",
			"读写分离",
			"分库分表",
			"数据迁移",
			"备份恢复",
		},
		"存储引擎": {
			"InnoDB特性",
			"MyISAM特性",
			"存储引擎选型",
			"缓冲池",
		},
	},

	"Redis": {
		"数据结构": {
			"String",
			"Hash",
			"List",
			"Set",
			"ZSet",
			"Stream",
			"底层实现",
		},
		"持久化": {
			"RDB机制",
			"AOF机制",
			"混合持久化",
			"持久化选型",
		},
		"高可用": {
			"主从复制",
			"哨兵模式",
			"Cluster集群",
			"故障转移",
		},
		"应用场景": {
			"缓存穿透",
			"缓存击穿",
			"缓存雪崩",
			"分布式锁",
			"延迟队列",
			"限流计数",
		},
		"性能优化": {
			"内存优化",
			"大Key处理",
			"热Key处理",
			"Pipeline批量",
			"连接池",
		},
	},

	"MQ": {
		"基础概念": {
			"消息模型",
			"生产者消费者",
			"Topic与Queue",
			"分区机制",
		},
		"可靠性": {
			"消息确认机制",
			"重试策略",
			"幂等性设计",
			"事务消息",
			"死信队列",
		},
		"高可用": {
			"集群架构",
			"副本机制",
			"故障恢复",
			"数据一致性",
		},
		"性能调优": {
			"吞吐量优化",
			"延迟优化",
			"批量发送",
			"消费者并行度",
		},
		"产品对比": {
			"Kafka特性",
			"RabbitMQ特性",
			"RocketMQ特性",
			"选型建议",
		},
	},
}

// GetTopicTree 获取指定领域的话题树
func GetTopicTree(domain string) map[string][]string {
	if tree, ok := TopicTrees[domain]; ok {
		return tree
	}
	return nil
}

// GetMainTopics 获取指定领域的主话题列表
func GetMainTopics(domain string) []string {
	tree := GetTopicTree(domain)
	if tree == nil {
		return nil
	}

	topics := make([]string, 0, len(tree))
	for topic := range tree {
		topics = append(topics, topic)
	}
	return topics
}

// GetSubTopics 获取指定领域和主话题下的子话题
func GetSubTopics(domain, mainTopic string) []string {
	tree := GetTopicTree(domain)
	if tree == nil {
		return nil
	}

	if subs, ok := tree[mainTopic]; ok {
		return subs
	}
	return nil
}

// GetAllTopics 获取指定领域的所有话题（扁平化）
func GetAllTopics(domain string) []string {
	tree := GetTopicTree(domain)
	if tree == nil {
		return nil
	}

	var allTopics []string
	for mainTopic, subTopics := range tree {
		allTopics = append(allTopics, mainTopic)
		allTopics = append(allTopics, subTopics...)
	}
	return allTopics
}
