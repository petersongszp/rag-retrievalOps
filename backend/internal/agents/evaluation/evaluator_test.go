package evaluation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"interview-agents/internal/config"
	"interview-agents/internal/repository"
)

// findConfigForTest 在测试中查找 config.yaml，若未找到返回空字符串（12.3.1 集成测试可选）
func findConfigForTest() string {
	wd, _ := os.Getwd()
	for _, p := range []string{
		filepath.Join(wd, "config.yaml"),
		filepath.Join(wd, "..", "config.yaml"),
		filepath.Join(wd, "..", "..", "config.yaml"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func TestEvaluator_EvaluateWithHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需配置与 LLM 的集成测试（-short）")
	}
	_ = godotenv.Load()
	configPath := findConfigForTest()
	if configPath == "" {
		t.Skip("未找到 config.yaml，跳过集成测试")
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	cfg.ExpandEnv()
	if err := repository.InitDatabase(cfg.Database); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	ctx := context.Background()
	var userId uint = 1
	ev, err := NewEvaluator(ctx, userId, nil)
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
	evaluate, err := ev.Evaluate(ctx, &EvaluationRequest{
		Domain:       "go",
		CurrentTopic: "",
		Question:     "请谈谈 Go 中 goroutine 与 channel 的关系。",
		Answer:       "goroutine 是轻量级线程，channel 用于通信。",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	t.Logf("ev result: %+v", evaluate)
}
