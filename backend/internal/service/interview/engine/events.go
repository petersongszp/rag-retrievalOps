package core

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// SendSSEEvent 发送 SSE 事件
func SendSSEEvent(writer io.Writer, event map[string]interface{}) error {
	eventJSON, _ := json.Marshal(event)

	// 获取事件类型
	eventType := "message"
	if t, ok := event["type"]; ok {
		eventType = fmt.Sprintf("%v", t)
	}

	// 标准 SSE 格式：event: type\ndata: {...}\n\n
	message := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(eventJSON))
	n, err := fmt.Fprint(writer, message)
	if err != nil {
		log.Printf("[SSE] Failed to write event: %v (wrote %d bytes)", err, n)
		return err
	}

	// 立即 flush，确保数据发送到客户端
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

// SendErrorEvent 发送错误事件
func SendErrorEvent(writer io.Writer, message string) {
	err := SendSSEEvent(writer, map[string]interface{}{"type": "error", "message": message})
	if err != nil {
		return
	}
}

// SendCompleteEvent 发送完成事件
func SendCompleteEvent(writer io.Writer) {
	err := SendSSEEvent(writer, map[string]interface{}{"type": "complete", "message": "面试已结束"})
	if err != nil {
		return
	}
}

// SendReadyEventWithSession 发送就绪事件
func SendReadyEventWithSession(writer io.Writer, questionIndex int, sessionID string) {
	err := SendSSEEvent(writer, map[string]interface{}{
		"type":           "ready_for_answer",
		"message":        "请回答上述问题",
		"question_index": questionIndex,
		"session_id":     sessionID,
	})
	if err != nil {
		return
	}
}

// SendHeartbeatEvent 发送心跳事件保活连接
func SendHeartbeatEvent(writer io.Writer) error {
	return SendSSEEvent(writer, map[string]interface{}{
		"type":    "heartbeat",
		"message": "连接保活",
	})
}

// SendStructuredMessage 发送结构化消息（支持 MessageSchema）
// 这是第8章要求的核心实现：8.4.1 SSE 流式多路复用
func SendStructuredMessage(writer io.Writer, schema *MessageSchema) error {
	// 将 MessageSchema 转换为 map
	eventData := map[string]interface{}{
		"type":        "structured_message",
		"message_id":  schema.MessageID,
		"timestamp":   schema.Timestamp.Unix(),
		"role":        schema.Role,
		"role_name":   schema.RoleName,
		"role_avatar": schema.RoleAvatar,
		"content":     schema.Content,
		"action_type": schema.ActionType,
		"status":      schema.Status,
	}

	// 添加元数据（如果存在）
	if len(schema.Metadata) > 0 {
		eventData["metadata"] = schema.Metadata
	}

	return SendSSEEvent(writer, eventData)
}

// SendChunkMessage 发送分块消息（用于打字机效果）
func SendChunkMessage(writer io.Writer, role RoleType, chunk string, index int) error {
	config := GetRoleConfig(role)

	eventData := map[string]interface{}{
		"type":        "chunk",
		"role":        role,
		"role_name":   config.RoleName,
		"role_avatar": config.AvatarSeed,
		"content":     chunk,
		"index":       index,
	}

	return SendSSEEvent(writer, eventData)
}

// SendThinkingStatus 发送思考状态
func SendThinkingStatus(writer io.Writer, role RoleType) error {
	config := GetRoleConfig(role)

	eventData := map[string]interface{}{
		"type":        "thinking",
		"role":        role,
		"role_name":   config.RoleName,
		"role_avatar": config.AvatarSeed,
		"action_type": ActionThinking,
	}

	return SendSSEEvent(writer, eventData)
}
