package asr

import "testing"

func TestLoadConfigFromEnvRequiresDedicatedASRConfig(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "https://api.siliconflow.cn/v1")
	t.Setenv("OPENAI_API_KEY", "general-key")
	t.Setenv("OPENAI_MODEL_NAME", "general-model")
	t.Setenv("OPENAI_ASR_BASE_URL", "")
	t.Setenv("OPENAI_ASR_API_KEY", "")
	t.Setenv("OPENAI_ASR_MODEL_NAME", "")

	cfg := LoadConfigFromEnv()
	capability := cfg.Capability()
	if capability.Enabled {
		t.Fatal("expected ASR to be disabled when dedicated config is missing")
	}
	if capability.Reason != CapabilityReasonNotConfigured {
		t.Fatalf("expected reason %q, got %q", CapabilityReasonNotConfigured, capability.Reason)
	}
}

func TestLoadConfigFromEnvRequiresAllDedicatedFields(t *testing.T) {
	t.Setenv("OPENAI_ASR_BASE_URL", "https://api.siliconflow.cn/v1")
	t.Setenv("OPENAI_ASR_API_KEY", "asr-key")
	t.Setenv("OPENAI_ASR_MODEL_NAME", "")

	cfg := LoadConfigFromEnv()
	capability := cfg.Capability()
	if capability.Enabled {
		t.Fatal("expected ASR to be disabled when dedicated model name is missing")
	}
}
