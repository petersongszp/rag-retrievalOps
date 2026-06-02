package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	APIKeyPrefix = "rag_"
	APIKeyLength = 32 // 随机部分长度
)

// GenerateAPIKey 生成 API Key
// 返回: 明文 key, key_hash, key_prefix
func GenerateAPIKey() (string, string, string, error) {
	// 生成随机字节
	randomBytes := make([]byte, APIKeyLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", "", fmt.Errorf("generate random: %w", err)
	}

	// 构造 key
	key := APIKeyPrefix + hex.EncodeToString(randomBytes)

	// 计算 hash
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	// 取前缀
	keyPrefix := key[:12] + "..."

	return key, keyHash, keyPrefix, nil
}

// HashAPIKey 计算 API Key 的 hash
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// ValidateAPIKeyFormat 校验 API Key 格式
func ValidateAPIKeyFormat(key string) bool {
	return strings.HasPrefix(key, APIKeyPrefix) && len(key) >= len(APIKeyPrefix)+10
}

// FormatPermissions 格式化权限为 JSON 字符串
func FormatPermissions(permissions []string) string {
	if len(permissions) == 0 {
		return "[]"
	}
	bytes, err := json.Marshal(permissions)
	if err != nil {
		return "[]"
	}
	return string(bytes)
}

// ParsePermissions 解析权限 JSON 字符串
func ParsePermissions(permissionsStr string) []string {
	if permissionsStr == "" {
		return []string{}
	}
	var permissions []string
	if err := json.Unmarshal([]byte(permissionsStr), &permissions); err != nil {
		return []string{}
	}
	return permissions
}

// IsAPIKeyExpired 检查 API Key 是否过期
func IsAPIKeyExpired(expiresAt *time.Time) bool {
	if expiresAt == nil {
		return false // 永不过期
	}
	return time.Now().After(*expiresAt)
}
