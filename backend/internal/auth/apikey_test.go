package auth

import (
	"testing"
	"time"
)

func TestGenerateAPIKey(t *testing.T) {
	key, keyHash, keyPrefix, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	// 检查格式
	if !ValidateAPIKeyFormat(key) {
		t.Errorf("key format invalid: %s", key)
	}

	// 检查 hash
	if keyHash == "" {
		t.Error("keyHash is empty")
	}

	// 检查前缀
	if keyPrefix == "" {
		t.Error("keyPrefix is empty")
	}

	// 检查唯一性
	key2, _, _, _ := GenerateAPIKey()
	if key == key2 {
		t.Error("two generated keys should be different")
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "rag_test123456789abcdef"
	hash1 := HashAPIKey(key)
	hash2 := HashAPIKey(key)

	// 相同输入应产生相同 hash
	if hash1 != hash2 {
		t.Errorf("same key should produce same hash: %s != %s", hash1, hash2)
	}

	// 不同输入应产生不同 hash
	hash3 := HashAPIKey("rag_different_key")
	if hash1 == hash3 {
		t.Error("different keys should produce different hashes")
	}
}

func TestValidateAPIKeyFormat(t *testing.T) {
	tests := []struct {
		key   string
		valid bool
	}{
		{"rag_abcdefghijklmnopqrstuvwxyz123456", true},
		{"rag_short", false},    // 太短
		{"invalid_key", false},  // 无前缀
		{"", false},             // 空
		{"rag_", false},         // 只有前缀
	}

	for _, tt := range tests {
		result := ValidateAPIKeyFormat(tt.key)
		if result != tt.valid {
			t.Errorf("ValidateAPIKeyFormat(%q) = %v, want %v", tt.key, result, tt.valid)
		}
	}
}

func TestFormatParsePermissions(t *testing.T) {
	tests := []struct {
		permissions []string
	}{
		{[]string{"retrieve"}},
		{[]string{"retrieve", "kb_ids"}},
		{[]string{}},
		{nil},
	}

	for _, tt := range tests {
		formatted := FormatPermissions(tt.permissions)
		parsed := ParsePermissions(formatted)

		if tt.permissions == nil || len(tt.permissions) == 0 {
			if len(parsed) != 0 {
				t.Errorf("expected empty permissions, got %v", parsed)
			}
		} else {
			if len(parsed) != len(tt.permissions) {
				t.Errorf("permissions count mismatch: %d != %d", len(parsed), len(tt.permissions))
			}
		}
	}
}

func TestIsAPIKeyExpired(t *testing.T) {
	// nil 表示永不过期
	if IsAPIKeyExpired(nil) {
		t.Error("nil expires_at should not be expired")
	}

	// 未来时间
	future := time.Now().Add(time.Hour)
	if IsAPIKeyExpired(&future) {
		t.Error("future time should not be expired")
	}

	// 过去时间
	past := time.Now().Add(-time.Hour)
	if !IsAPIKeyExpired(&past) {
		t.Error("past time should be expired")
	}
}
