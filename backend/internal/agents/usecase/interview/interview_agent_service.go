package interview

import (
	"context"
	"fmt"
	"interview-agents/internal/agents/interview/comprehensive"
	"interview-agents/internal/agents/interview/specialized"
	"interview-agents/internal/agents/multiagent"
	"io"
	"log"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	mycallbacks "interview-agents/pkg/eino/callbacks"
)

// InterviewAgentType 面试智能体类型
type InterviewAgentType string

const (
	// Comprehensive 综合面试类型
	ComprehensiveSchool InterviewAgentType = "comprehensive_school" // 综合校招
	ComprehensiveSocial InterviewAgentType = "comprehensive_social" // 综合社招

	// Specialized 专项面试类型
	SpecializedGo    InterviewAgentType = "specialized_go"    // Go 专项
	SpecializedJava  InterviewAgentType = "specialized_java"  // Java 专项
	SpecializedMQ    InterviewAgentType = "specialized_mq"    // MQ 专项
	SpecializedMySQL InterviewAgentType = "specialized_mysql" // MySQL 专项
	SpecializedRedis InterviewAgentType = "specialized_redis" // Redis 专项

	// MultiAgent 多智能体面试类型
	GroupInterview InterviewAgentType = "group_interview" // 群组面试
)

// InterviewAgentService 面试智能体服务
type InterviewAgentService struct {
	userId uint
}

// NewInterviewAgentService 创建面试智能体服务
func NewInterviewAgentService(userId uint) *InterviewAgentService {
	return &InterviewAgentService{
		userId: userId,
	}
}

// GetInterviewAgent 根据类型获取对应的面试智能体
// 参数:
//   - agentType: 智能体类型
//   - needResumeTool: 是否需要简历工具
//
// 返回:
//   - adk.Agent: 面试智能体
//   - error: 错误信息
func (s *InterviewAgentService) GetInterviewAgent(agentType InterviewAgentType, needResumeTool bool) (adk.Agent, error) {
	switch agentType {
	// Comprehensive 综合面试智能体
	case ComprehensiveSchool:
		return comprehensive.NewSchoolComprehensiveAgent(s.userId, needResumeTool)
	case ComprehensiveSocial:
		return comprehensive.NewSocialComprehensiveAgent(s.userId, needResumeTool)

	// Specialized 专项面试智能体
	case SpecializedGo:
		return specialized.NewGoSpecializedAgent(s.userId, needResumeTool)
	case SpecializedJava:
		return specialized.NewJavaSpecializedAgent(s.userId, needResumeTool)
	case SpecializedMQ:
		return specialized.NewMQSpecializedAgent(s.userId, needResumeTool)
	case SpecializedMySQL:
		return specialized.NewMySQLSpecializedAgent(s.userId, needResumeTool)
	case SpecializedRedis:
		return specialized.NewRedisSpecializedAgent(s.userId, needResumeTool)

	// GroupInterview 群组面试智能体
	case GroupInterview:
		ctx := context.Background()
		return multiagent.NewInterviewHostAgent(ctx, s.userId, needResumeTool)

	default:
		return nil, fmt.Errorf("unknown interview agent type: %s", agentType)
	}
}

// RunInterviewWithCallback 运行面试并通过回调返回结果
// 参数:
//   - ctx: 上下文
//   - agentType: 智能体类型
//   - needResumeTool: 是否需要简历工具
//   - prompt: 提示词/问题
//   - callback: 回调函数，每次接收到消息时调用
//
// 返回:
//   - error: 错误信息
func (s *InterviewAgentService) RunInterviewWithCallback(ctx context.Context, agentType InterviewAgentType, needResumeTool bool, prompt string, callback func(message string) error) error {
	// 获取对应的智能体
	agent, err := s.GetInterviewAgent(agentType, needResumeTool)
	if err != nil {
		log.Printf("[RunInterviewWithCallback] 获取智能体失败: %v", err)
		return err
	}

	// 运行智能体
	_, err = runAgentWithIterator(ctx, agent, prompt, callback)
	if err != nil {
		log.Printf("[RunInterviewWithCallback] 智能体执行出错: %v", err)
		return err
	}

	return nil
}

// runAgentWithIterator 运行智能体的通用方法
// 参数:
//   - ctx: 上下文
//   - agent: 智能体
//   - prompt: 提示词/问题
//   - callback: 可选的回调函数，为 nil 时只收集最后一条消息
//
// 返回:
//   - string: 最后一条消息（仅当 callback 为 nil 时有效）
//   - error: 错误信息
func runAgentWithIterator(ctx context.Context, agent adk.Agent, prompt string, callback func(string) error) (string, error) {
	// 12.1.1 全链路监控：集成 Eino Callbacks，对单次 Agent Run 做 OnStart/OnEnd/OnError 埋点
	monitor := mycallbacks.NewMonitoringHandler()
	runInfo := &callbacks.RunInfo{Name: "InterviewAgent", Component: "Runner"}
	ctx = monitor.OnStart(ctx, runInfo, nil)
	var lastOutput callbacks.CallbackOutput // 用于 OnEnd 时尝试提取 Token（若类型兼容）
	defer func() {
		monitor.OnEnd(ctx, runInfo, lastOutput)
	}()

	// 创建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	// 构建消息
	messages := []adk.Message{
		schema.UserMessage(prompt),
	}

	// 运行智能体
	iter := runner.Run(ctx, messages)

	var lastMessage string
	contentFilter := newThinkContentFilter()
	// Eino 在一次 Run 中可能对同一轮助手回复产出第二条 MessageStream（内容重复），会导致 SSE chunk 与 structured_message 整段重复
	streamCompleted := false
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}

		if event.Err != nil {
			monitor.OnError(ctx, runInfo, event.Err)
			return "", event.Err
		}

		// 保留最后一次 Output，供 OnEnd 中尝试提取 Token（若 Eino 产出类型与 CallbackOutput 兼容）
		if event.Output != nil {
			lastOutput = event.Output
		}

		// 处理消息事件
		if event.Output != nil && event.Output.MessageOutput != nil {
			mo := event.Output.MessageOutput
			if mo.MessageStream != nil {
				stream := mo.MessageStream
				if stream == nil {
					continue
				}
				// 已消费过一条完整流式回复，后续 MessageStream 仅排空，不再推给前端
				if streamCompleted {
					for {
						msg, err := stream.Recv()
						if err != nil {
							if err == io.EOF {
								break
							}
							monitor.OnError(ctx, runInfo, err)
							return "", err
						}
						_ = msg
					}
					log.Printf("[runAgentWithIterator] skipped duplicate MessageStream (same run)")
					continue
				}

				streamHadAssistant := false
				for {
					msg, err := stream.Recv()
					if err != nil {
						if err == io.EOF {
							if tail := contentFilter.Flush(); tail != "" {
								streamHadAssistant = true
								lastMessage += tail
								if callback != nil {
									if err := callback(tail); err != nil {
										monitor.OnError(ctx, runInfo, err)
										return "", err
									}
								}
							}
							break
						}
						monitor.OnError(ctx, runInfo, err)
						return "", err
					}

					// 修复：仅将 Assistant 角色的消息输出给前端，过滤掉 Tool 或 System 消息
					if msg.Role != schema.Assistant {
						continue
					}

					if msg.Content != "" {
						cleaned := contentFilter.Push(msg.Content)
						if cleaned == "" {
							continue
						}
						streamHadAssistant = true
						lastMessage += cleaned
						if callback != nil {
							if err := callback(cleaned); err != nil {
								monitor.OnError(ctx, runInfo, err)
								return "", err
							}
						}
					}
				}
				// 仅当本条流里确实出现过 Assistant 正文时，才认为「本轮流式回复已结束」，避免首条流只有 Tool 时误跳过后续真实流
				if streamHadAssistant {
					streamCompleted = true
				}
			} else if mo.Message != nil && mo.Message.Content != "" {
				// 已在流式中收到过助手正文后，再收到非流式整包常为重复；无正文时仍要走非流式
				if streamCompleted && lastMessage != "" {
					log.Printf("[runAgentWithIterator] skipped duplicate non-stream Message after stream completed")
					continue
				}
				// 处理非流式消息
				msg := mo.Message
				// 修复：仅处理 Assistant 角色
				if msg.Role == schema.Assistant {
					cleaned := contentFilter.Push(msg.Content)
					cleaned += contentFilter.Flush()
					if cleaned == "" {
						continue
					}
					lastMessage = cleaned
					if callback != nil {
						if err := callback(cleaned); err != nil {
							monitor.OnError(ctx, runInfo, err)
							return "", err
						}
					}
				}
			}
		}
	}

	return lastMessage, nil
}

type thinkContentFilter struct {
	buffer  string
	inThink bool
}

var (
	thinkOpenTags      = []string{"<thinking>", "<think>"}
	thinkCloseTags     = []string{"</thinking>", "</think>"}
	maxOpenPrefixKeep  = maxTagLen(thinkOpenTags) - 1
	maxClosePrefixKeep = maxTagLen(thinkCloseTags) - 1
)

func newThinkContentFilter() *thinkContentFilter {
	return &thinkContentFilter{}
}

func (f *thinkContentFilter) Push(chunk string) string {
	if chunk == "" {
		return ""
	}

	f.buffer += chunk
	var out strings.Builder

	for {
		if !f.inThink {
			idx, tagLen := findEarliestTagIndex(strings.ToLower(f.buffer), thinkOpenTags)
			if idx == -1 {
				emitUntil := len(f.buffer) - maxOpenPrefixKeep
				if emitUntil <= 0 {
					return out.String()
				}
				out.WriteString(f.buffer[:emitUntil])
				f.buffer = f.buffer[emitUntil:]
				return out.String()
			}

			out.WriteString(f.buffer[:idx])
			f.buffer = f.buffer[idx+tagLen:]
			f.inThink = true
			continue
		}

		idx, tagLen := findEarliestTagIndex(strings.ToLower(f.buffer), thinkCloseTags)
		if idx == -1 {
			if len(f.buffer) > maxClosePrefixKeep {
				f.buffer = f.buffer[len(f.buffer)-maxClosePrefixKeep:]
			}
			return out.String()
		}

		f.buffer = f.buffer[idx+tagLen:]
		f.inThink = false
	}
}

func (f *thinkContentFilter) Flush() string {
	if f.inThink {
		f.buffer = ""
		return ""
	}
	out := f.buffer
	f.buffer = ""
	return out
}

func findEarliestTagIndex(contentLower string, tags []string) (int, int) {
	index := -1
	length := 0
	for _, tag := range tags {
		i := strings.Index(contentLower, tag)
		if i == -1 {
			continue
		}
		if index == -1 || i < index {
			index = i
			length = len(tag)
		}
	}
	return index, length
}

func maxTagLen(tags []string) int {
	maxLen := 0
	for _, tag := range tags {
		if len(tag) > maxLen {
			maxLen = len(tag)
		}
	}
	return maxLen
}
