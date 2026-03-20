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
	t.Setenv("OPENAI_ASR_MODIFY_LLM_MODEL", "Qwen/Qwen3.5-4B")

	cfg := LoadConfigFromEnv()
	capability := cfg.Capability()
	if capability.Enabled {
		t.Fatal("expected ASR to be disabled when dedicated model name is missing")
	}
}

func TestLoadConfigFromEnvModifierIsOptional(t *testing.T) {
	t.Setenv("OPENAI_ASR_BASE_URL", "https://api.siliconflow.cn/v1")
	t.Setenv("OPENAI_ASR_API_KEY", "asr-key")
	t.Setenv("OPENAI_ASR_MODEL_NAME", "FunAudioLLM/SenseVoiceSmall")
	t.Setenv("OPENAI_ASR_MODIFY_LLM_MODEL", "")

	cfg := LoadConfigFromEnv()
	if !cfg.Capability().Enabled {
		t.Fatal("expected ASR to remain enabled when modifier model is empty")
	}
	if cfg.ModifierEnabled() {
		t.Fatal("expected transcript modifier to stay disabled when modifier model is empty")
	}
}

func TestLoadConfigFromEnvReadsModifierModel(t *testing.T) {
	t.Setenv("OPENAI_ASR_BASE_URL", "https://api.siliconflow.cn/v1")
	t.Setenv("OPENAI_ASR_API_KEY", "asr-key")
	t.Setenv("OPENAI_ASR_MODEL_NAME", "FunAudioLLM/SenseVoiceSmall")
	t.Setenv("OPENAI_ASR_MODIFY_LLM_MODEL", "Qwen/Qwen3.5-4B")

	cfg := LoadConfigFromEnv()
	if cfg.ModifyLLMModel != "Qwen/Qwen3.5-4B" {
		t.Fatalf("expected modifier model to be loaded, got %q", cfg.ModifyLLMModel)
	}
	if !cfg.ModifierEnabled() {
		t.Fatal("expected transcript modifier to be enabled")
	}
}
