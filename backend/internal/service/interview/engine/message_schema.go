package core

import "time"

// MessageSchema 结构化的多角色对话消息体
// 符合第8章要求：8.2.3 通信协议：定义结构化的多角色对话消息体 (Message Schema)
type MessageSchema struct {
	// 基础字段
	MessageID string    `json:"message_id"` // 消息唯一ID
	Timestamp time.Time `json:"timestamp"`  // 时间戳

	// 角色信息
	Role       RoleType `json:"role"`        // 角色类型
	RoleName   string   `json:"role_name"`   // 角色显示名称（如"主面试官"）
	RoleAvatar string   `json:"role_avatar"` // 角色头像标识

	// 内容信息
	Content    string     `json:"content"`     // 消息内容
	ActionType ActionType `json:"action_type"` // 动作类型（思考/说话）

	// 状态信息
	Status MessageStatus `json:"status"` // 消息状态

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"` // 扩展元数据
}

// RoleType 角色类型
type RoleType string

const (
	RoleMainInterviewer    RoleType = "main_interviewer"    // 主面试官
	RoleTechInterviewer    RoleType = "tech_interviewer"    // 技术面试官
	RoleProjectInterviewer RoleType = "project_interviewer" // 项目面试官
	RoleCandidate          RoleType = "candidate"           // 候选人
	RoleSystem             RoleType = "system"              // 系统消息
)

// ActionType 动作类型
type ActionType string

const (
	ActionThinking ActionType = "thinking" // 思考中
	ActionSpeaking ActionType = "speaking" // 说话中
	ActionWaiting  ActionType = "waiting"  // 等待中
)

// MessageStatus 消息状态
type MessageStatus string

const (
	StatusPending   MessageStatus = "pending"   // 待处理
	StatusStreaming MessageStatus = "streaming" // 流式传输中
	StatusComplete  MessageStatus = "complete"  // 完成
	StatusError     MessageStatus = "error"     // 错误
)

// RoleConfig 角色配置
type RoleConfig struct {
	RoleType    RoleType
	RoleName    string
	AvatarSeed  string
	ColorClass  string
	BorderClass string
}

// GetRoleConfig 根据角色类型获取配置
func GetRoleConfig(roleType RoleType) RoleConfig {
	configs := map[RoleType]RoleConfig{
		RoleMainInterviewer: {
			RoleType:    RoleMainInterviewer,
			RoleName:    "主面试官",
			AvatarSeed:  "interviewer-main-v2",
			ColorClass:  "text-orange-500",
			BorderClass: "border-orange-200",
		},
		RoleTechInterviewer: {
			RoleType:    RoleTechInterviewer,
			RoleName:    "技术面试官",
			AvatarSeed:  "interviewer-tech-v2",
			ColorClass:  "text-blue-500",
			BorderClass: "border-blue-200",
		},
		RoleProjectInterviewer: {
			RoleType:    RoleProjectInterviewer,
			RoleName:    "项目面试官",
			AvatarSeed:  "interviewer-project-v2",
			ColorClass:  "text-emerald-500",
			BorderClass: "border-emerald-200",
		},
		RoleCandidate: {
			RoleType:    RoleCandidate,
			RoleName:    "候选人",
			AvatarSeed:  "user-candidate-v2",
			ColorClass:  "text-blue-600",
			BorderClass: "border-blue-100",
		},
		RoleSystem: {
			RoleType:    RoleSystem,
			RoleName:    "系统",
			AvatarSeed:  "system-v2",
			ColorClass:  "text-slate-400",
			BorderClass: "border-slate-200",
		},
	}

	config, ok := configs[roleType]
	if !ok {
		return configs[RoleSystem]
	}
	return config
}

// DetectRoleFromContent 从消息内容中检测角色
// 这是一个临时方法，用于兼容现有的基于前缀的角色识别
func DetectRoleFromContent(content string) RoleType {
	// 优先检查副面试官/技术面试官关键词
	if containsAny(content, []string{"副面试官", "技术副面", "技术面试官", "我是技术面试官：", "I am the technical interviewer:"}) {
		return RoleTechInterviewer
	}

	// 检查项目面试官
	if containsAny(content, []string{"项目面试官", "项目负责人", "项目专家", "我是项目面试官：", "I am the project interviewer:"}) {
		return RoleProjectInterviewer
	}

	// 检查主面试官
	if containsAny(content, []string{"主面试官", "我是主面试官：", "I am the main interviewer:"}) {
		return RoleMainInterviewer
	}

	// 默认为主面试官
	return RoleMainInterviewer
}

// containsAny 检查字符串是否包含任意一个子串
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// NewMessageSchema 创建新的消息体
func NewMessageSchema(role RoleType, content string, actionType ActionType) *MessageSchema {
	config := GetRoleConfig(role)

	return &MessageSchema{
		MessageID:  generateMessageID(),
		Timestamp:  time.Now(),
		Role:       role,
		RoleName:   config.RoleName,
		RoleAvatar: config.AvatarSeed,
		Content:    content,
		ActionType: actionType,
		Status:     StatusStreaming,
		Metadata:   make(map[string]interface{}),
	}
}

// generateMessageID 生成消息ID
func generateMessageID() string {
	return "msg_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}
