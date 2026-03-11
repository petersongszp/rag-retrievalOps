package alert_test

import (
	"interview-agents/internal/alert"
	"interview-agents/internal/config"
	"errors"
	"fmt"
	"testing"
)

// go test -v ./internal/alert -run TestSendDatabaseErrorAlert
// TestSendDatabaseErrorAlert 测试数据库错误告警
func TestSendDatabaseErrorAlert(t *testing.T) {

	// 真实的 Webhook URL
	webhookURL := "https://open.feishu.cn/open-apis/bot/v2/hook/4c63a7fb-bb8b-44b5-ab00-b8d08ccc4dfa"

	if webhookURL == "" {
		t.Skip("未配置真实的 Webhook URL，跳过集成测试")
	}

	// 配置测试环境
	config.Global.Feishu.Enabled = true
	config.Global.Feishu.WebhookURL = webhookURL

	// 执行测试
	testErr := errors.New("connection timeout")
	alert.SendDatabaseErrorAlert("SaveInterviewDialogues", testErr, 3)

}

// go test -v ./internal/alert -run TestSendFeishuAlert_Integration
// TestSendFeishuAlert_Integration 集成测试（需要真实的飞书 Webhook URL）
// 使用方法：设置环境变量 FEISHU_WEBHOOK_URL_TEST 为真实的 Webhook URL，然后运行测试
func TestSendFeishuAlert_Integration(t *testing.T) {
	// 跳过集成测试（除非明确指定）
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 从环境变量读取真实的 Webhook URL
	webhookURL := "https://open.feishu.cn/open-apis/bot/v2/hook/4c63a7fb-bb8b-44b5-ab00-b8d08ccc4dfa" // 在这里填写你的测试 Webhook URL，或从环境变量读取

	if webhookURL == "" {
		t.Skip("未配置真实的 Webhook URL，跳过集成测试")
	}

	// 配置测试环境
	config.Global.Feishu.Enabled = true
	config.Global.Feishu.WebhookURL = webhookURL

	// 执行测试
	err := alert.SendFeishuAlert("单元测试告警", fmt.Sprintf("这是一条来自单元测试的告警消息\n测试时间: %s", "2025-11-25"))
	if err != nil {
		t.Fatalf("集成测试失败: %v", err)
	}

	t.Log("集成测试成功，请检查飞书群是否收到消息")
}
