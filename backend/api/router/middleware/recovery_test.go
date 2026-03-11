package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"interview-agents/internal/alert"
	"interview-agents/internal/config"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

// Mock Feishu config to prevent actual network calls during test
func setupTestConfig() {
	config.Global.Feishu.Enabled = false
	config.Global.Feishu.WebhookURL = "https://mock.url"
}

func TestRecoveryMiddleware(t *testing.T) {
	setupTestConfig()

	// Capture if SendFeishuCard was attempted (we can't easily mock the function in Go without an interface,
	// but we can verify the middleware doesn't crash and returns 500)

	h := server.Default()
	h.Use(Recovery())
	h.GET("/panic", func(ctx context.Context, c *app.RequestContext) {
		panic("test panic")
	})

	w := ut.PerformRequest(h.Engine, "GET", "/panic", nil)
	resp := w.Result()

	assert.DeepEqual(t, http.StatusInternalServerError, resp.StatusCode())

	// Read response body - note: hertz response body is a function in some versions or just []byte accessor
	// Based on error: resp.Body is func() []byte
	bodyBytes := resp.Body()
	var body map[string]interface{}
	json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&body)
	// defer resp.Body.Close() // Not needed for hertz response usually if it's not a stream

	// Verify error response structure
	// Response struct: {Code, Message, Data}
	assert.NotNil(t, body["message"])
	assert.DeepEqual(t, "Internal server error", body["message"])

	// Check code if present
	if code, ok := body["code"]; ok {
		// JSON numbers are float64
		assert.DeepEqual(t, float64(500), code.(float64))
	}
}

func TestFeishuCardSerialization(t *testing.T) {
	// Verify manual JSON marshaling of the card structure
	card := alert.FeishuCardContent{
		Header: alert.FeishuCardHeader{
			Template: "red",
			Title: alert.FeishuCardTitle{
				Content: "Test Title",
				Tag:     "plain_text",
			},
		},
		Elements: []interface{}{
			alert.FeishuDivElement{
				Tag: "div",
				Text: alert.FeishuTextObject{
					Tag:     "lark_md",
					Content: "Test Content",
				},
			},
		},
	}

	msg := alert.FeishuCardMessage{
		MsgType: "interactive",
		Card:    card,
	}

	bytes, err := json.Marshal(msg)
	assert.Nil(t, err)
	assert.True(t, len(bytes) > 0)

	// Check key fields exist in JSON
	jsonStr := string(bytes)
	assert.True(t, contains(jsonStr, "interactive"))
	assert.True(t, contains(jsonStr, "red"))
	assert.True(t, contains(jsonStr, "Test Title"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && search(s, substr)
}

func search(s, substr string) bool {
	// simplified contains
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
