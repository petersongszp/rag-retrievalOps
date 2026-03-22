package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const transcriptModifierTimeout = 2 * time.Second

const transcriptModifierSystemPrompt = `你是一个面试语音识别文本修正助手。

你的任务是基于“面试官当前问题”和“ASR 识别结果”，只做最小必要的文本修正，让候选人的原意更准确、更可读。

请严格遵守：
1. 不改变用户原意，不补充用户没说过的信息。
2. 不做总结、翻译、润色、扩写，不改成书面范文。
3. 可以结合问题语境修正明显识别错误的专业术语、技术名词、缩写、数字和英文单词。
4. 对中英混说场景，可以删除无意义的停顿词或语气词，例如“嗯”“额”“uh”“em”，但不要删除有实际语义的内容。
5. 保留代码、命令、专有名词、否定表达、时间顺序和不确定语气。
6. 只输出修正后的最终文本，不要输出解释、标签、引号、Markdown 或额外说明。`

type openAITranscriptModifier struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

func NewOpenAITranscriptModifier(cfg ASRConfig) TranscriptModifier {
	return &openAITranscriptModifier{
		endpoint: buildChatCompletionsEndpoint(cfg.BaseURL),
		apiKey:   cfg.APIKey,
		model:    cfg.ModifyLLMModel,
		client: &http.Client{
			Timeout:   0,
			Transport: getASRTransport(),
		},
	}
}

func (m *openAITranscriptModifier) Modify(ctx context.Context, req TranscriptModifyRequest) (string, error) {
	body, err := json.Marshal(map[string]interface{}{
		"model": m.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": transcriptModifierSystemPrompt,
			},
			{
				"role":    "user",
				"content": buildTranscriptModifierUserPrompt(req),
			},
		},
		"temperature": 0,
		"stream":      false,
	})
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("transcript modifier request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content interface{} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return "", err
	}

	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("transcript modifier response has no choices")
	}

	return strings.TrimSpace(extractChatCompletionContent(payload.Choices[0].Message.Content)), nil
}

func buildTranscriptModifierUserPrompt(req TranscriptModifyRequest) string {
	return fmt.Sprintf("面试官问题：\n%s\n\nASR识别结果：\n%s\n\n请输出修正后的最终文本。", req.QuestionText, req.Transcript)
}

func buildChatCompletionsEndpoint(baseURL string) string {
	url := strings.TrimSpace(baseURL)
	url = strings.TrimSuffix(url, "/chat/completions")
	url = strings.TrimSuffix(url, "/")
	return url + "/chat/completions"
}

func extractChatCompletionContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var builder strings.Builder
		for _, item := range v {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			text, _ := part["text"].(string)
			builder.WriteString(text)
		}
		return builder.String()
	default:
		return ""
	}
}
