package core

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"
)

// WaitForAnswerWithHeartbeat 等待用户答案，并定期发送心跳保活
func WaitForAnswerWithHeartbeat(ctx context.Context, sm *SessionManager, sessionID string, timeout time.Duration, heartbeatInterval time.Duration, writer io.Writer) (string, bool) {
	log.Printf("[Wait Answer] Starting, sessionID: %s, timeout: %v, heartbeatInterval: %v", sessionID, timeout, heartbeatInterval)

	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	// 获取会话的答案通道
	session := sm.GetSession(sessionID)
	if session == nil {
		log.Printf("[Wait Answer] Session not found, sessionID: %s", sessionID)
		return "", false
	}

	heartbeatCount := 0
	for {
		select {
		// 监听 context 取消信号
		case <-ctx.Done():
			log.Printf("[Wait Answer] Context cancelled, sessionID: %s", sessionID)
			return "", false

		// 定期发送心跳保活
		case <-heartbeatTicker.C:
			heartbeatCount++
			log.Printf("[Wait Answer] Sending heartbeat #%d, sessionID: %s", heartbeatCount, sessionID)
			err := SendHeartbeatEvent(writer)
			if err != nil {
				return "", false
			}

		// 接收用户答案
		case answer := <-session.AnswerChan:
			log.Printf("[Wait Answer] Received answer, sessionID: %s, answer: %s", sessionID, answer)
			return answer, true

		// 等待超时
		case <-timeoutTimer.C:
			log.Printf("[Wait Answer] Timeout after %v, sent %d heartbeats, sessionID: %s", timeout, heartbeatCount, sessionID)
			return "", false
		}
	}
}

// SetupSSEResponse 设置 SSE 响应头
func SetupSSEResponse(c interface {
	SetStatusCode(int)
	Header(string, string)
}) {
	c.SetStatusCode(http.StatusOK)
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Cache-Control")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")
	c.Header("X-Content-Type-Options", "nosniff")
}
