package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"interview-agents/internal/config"
	"log"
	"net/http"
	"time"
)

// FeishuMessage 飞书消息结构
type FeishuMessage struct {
	MsgType string      `json:"msg_type"`
	Content interface{} `json:"content"`
}

// FeishuTextContent 飞书文本消息内容
type FeishuTextContent struct {
	Text string `json:"text"`
}

// FeishuCardMessage 飞书卡片消息结构
type FeishuCardMessage struct {
	MsgType string      `json:"msg_type"`
	Card    interface{} `json:"card"`
}

// FeishuCardContent 飞书卡片内容
type FeishuCardContent struct {
	Header   FeishuCardHeader `json:"header"`
	Elements []interface{}    `json:"elements"`
}

// FeishuCardHeader 飞书卡片标题
type FeishuCardHeader struct {
	Template string          `json:"template"` // blue, red, etc.
	Title    FeishuCardTitle `json:"title"`
}

// FeishuCardTitle 飞书卡片标题内容
type FeishuCardTitle struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

// FeishuCardElement 飞书卡片元素接口
type FeishuCardElement interface{}

// FeishuDivElement 飞书卡片文本块
type FeishuDivElement struct {
	Tag  string           `json:"tag"`
	Text FeishuTextObject `json:"text"`
}

// FeishuTextObject 飞书文本对象
type FeishuTextObject struct {
	Content string `json:"content"`
	Tag     string `json:"tag"` // plain_text, lark_md
}

// SendFeishuAlert 发送飞书告警
func SendFeishuAlert(title, content string) error {
	if !config.Global.Feishu.Enabled {
		log.Printf("[FeishuAlert] 飞书告警未启用，跳过发送")
		return nil
	}

	webhookURL := config.Global.Feishu.WebhookURL
	if webhookURL == "" {
		log.Printf("[FeishuAlert] 飞书 Webhook URL 未配置")
		return fmt.Errorf("飞书 Webhook URL 未配置")
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	text := fmt.Sprintf("【%s】\n时间: %s\n%s", title, timestamp, content)

	message := FeishuMessage{
		MsgType: "text",
		Content: FeishuTextContent{
			Text: text,
		},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Printf("[FeishuAlert] 序列化消息失败: %v", err)
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[FeishuAlert] 发送请求失败: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[FeishuAlert] 飞书返回非200状态码: %d", resp.StatusCode)
		return fmt.Errorf("飞书返回状态码: %d", resp.StatusCode)
	}

	log.Printf("[FeishuAlert] 告警发送成功: %s", title)
	return nil
}

// SendFeishuCard 发送飞书卡片消息
func SendFeishuCard(title string, contentMarkdown string, template string) error {
	if !config.Global.Feishu.Enabled {
		return nil
	}

	webhookURL := config.Global.Feishu.WebhookURL
	if webhookURL == "" {
		return fmt.Errorf("飞书 Webhook URL 未配置")
	}

	card := FeishuCardContent{
		Header: FeishuCardHeader{
			Template: template, // e.g., "red", "blue"
			Title: FeishuCardTitle{
				Content: title,
				Tag:     "plain_text",
			},
		},
		Elements: []interface{}{
			FeishuDivElement{
				Tag: "div",
				Text: FeishuTextObject{
					Tag:     "lark_md",
					Content: contentMarkdown,
				},
			},
		},
	}

	message := FeishuCardMessage{
		MsgType: "interactive",
		Card:    card,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Printf("[FeishuAlert] 序列化卡片失败: %v", err)
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[FeishuAlert] 发送卡片请求失败: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[FeishuAlert] 飞书返回非200状态码: %d", resp.StatusCode)
		return fmt.Errorf("飞书返回状态码: %d", resp.StatusCode)
	}

	return nil
}

// SendDatabaseErrorAlert 发送数据库错误告警
func SendDatabaseErrorAlert(operation string, err error, retryCount int) {
	title := "数据库操作失败告警"
	content := fmt.Sprintf("操作: %s\n错误: %v\n已尝试次数: %d", operation, err, retryCount)

	if alertErr := SendFeishuAlert(title, content); alertErr != nil {
		log.Printf("[FeishuAlert] 发送告警失败: %v", alertErr)
	}
}

// Template 飞书卡片颜色模板
const (
	TemplateRed    = "red"    // 严重/系统异常
	TemplateOrange = "orange" // 业务风控/警告
	TemplateBlue   = "blue"   // 一般信息
)

// SendSystemExceptionAlert 发送系统异常告警（12.2.2 系统异常）
// 用于 Eino 组件错误、超时、Panic 等
func SendSystemExceptionAlert(traceID, component, errMsg string) error {
	title := "🔴 系统异常告警"
	content := fmt.Sprintf("**TraceID**: %s\n**组件**: %s\n**错误**: %s",
		traceID, component, errMsg)
	return SendFeishuCard(title, content, TemplateRed)
}

// SendBusinessRiskAlert 发送业务风控告警（12.2.2 业务风控）
// 用于敏感内容、滥用、配额超限等业务风险
func SendBusinessRiskAlert(reason, detail string) error {
	title := "⚠️ 业务风控告警"
	content := fmt.Sprintf("**原因**: %s\n**详情**: %s", reason, detail)
	return SendFeishuCard(title, content, TemplateOrange)
}
